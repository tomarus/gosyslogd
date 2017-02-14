// Package syslogd provides a library to write syslog servers.
//
// Example
//
//	sys := syslogd.NewServer(syslog.Options{SockAddr: config.C.SockAddr, UnixPath: config.C.UnixPath})
//	go func() {
//		for {
//			msg := sys.Next()
//			if msg == nil {
//				// no more messages, server exiting
//			}
//	}()
package syslogd

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

const defaultSockAddr = ":514"
const defaultUnixPath = "/dev/log"
const defaultBufferSize = 128 * 1024

// Options contain all configuration required for operating a syslog server.
// If one or more options are blank, unix syslogd defaults are used.
type Options struct {
	// SockAddr contains the listening port for UDP and TCP listeners, default is ":514"
	SockAddr string

	// UnixPath contains the unix socket path for unix syslogging. If you are running
	// this a the primary server on a unix system set this to "/dev/log" (which is default)
	// otherwise you're free to choose a path depending on what your application uses.
	UnixPath string

	// BufferSize contains the maximum number of messages queued before we block.
	// Defaults to 128k
	BufferSize int

	// LogDir defines the path where to store logfiles. Leave empty to not write logfiles.
	LogDir string
}

// Server contains internal data for syslog server processes.
type Server struct {
	tcp  *net.TCPConn
	udp  *net.UDPConn
	unix *net.UnixConn
	bus  chan *Message
	stop chan bool
	opts Options
	arch *archive
}

// Keep track of 'active' sending nodes.
var activeNodes map[string]time.Time
var nodeLock sync.RWMutex

func init() {
	activeNodes = make(map[string]time.Time)
}

// NumActiveNodes returns the number of nodes which have sent messages during the last X secs.
func NumActiveNodes(secs int) int {
	nodeLock.RLock()
	defer nodeLock.RUnlock()
	num := 0
	for _, n := range activeNodes {
		if time.Now().Sub(n) < time.Duration(secs)*time.Second {
			num++
		}
	}
	return num
}

func (s *Server) listenUDP() error {
	a, err := net.ResolveUDPAddr("udp", s.opts.SockAddr)
	if err != nil {
		return err
	}
	s.udp, err = net.ListenUDP("udp", a)
	if err != nil {
		return err
	}
	go s.receivePacket(s.udp)
	return nil
}

func (s *Server) listenUnix() error {
	os.Remove(s.opts.UnixPath)
	a, err := net.ResolveUnixAddr("unixgram", s.opts.UnixPath)
	if err != nil {
		return err
	}
	s.unix, err = net.ListenUnixgram("unixgram", a)
	if err != nil {
		return err
	}
	err = os.Chmod(s.opts.UnixPath, 0666)
	if err != nil {
		return err
	}
	go s.receivePacket(s.unix)
	return nil
}

func (s *Server) listenTCP() error {
	sock, err := net.Listen("tcp", s.opts.SockAddr)
	if err != nil {
		return err
	}

	go func() {
		accpt := make(chan net.Conn)
		for {
			go func() {
				client, err := sock.Accept()
				if err != nil {
					sock.Close()
					s.Close()
					return
				}
				accpt <- client
			}()

			select {
			case client := <-accpt:
				// Accepted new client connection.
				go s.receiveTCP(client)
			}
		}
	}()
	return nil
}

func (s *Server) receivePacket(con net.PacketConn) {
	buf := make([]byte, 4096)
	for {
		n, addr, err := con.ReadFrom(buf)
		if err != nil {
			return
		}
		nodeLock.Lock()
		activeNodes[addr.String()] = time.Now()
		nodeLock.Unlock()
		s.processBuf(buf, n)
	}
}

func (s *Server) receiveTCP(con net.Conn) {
	buf := bufio.NewReader(con)
	for {
		buf, err := buf.ReadBytes('\n')
		if err != nil {
			return
		}
		nodeLock.Lock()
		activeNodes[con.RemoteAddr().String()] = time.Now()
		nodeLock.Unlock()
		s.processBuf(buf, 0)
	}
}

func (s *Server) processBuf(b []byte, n int) {
	msg, err := NewMessage(b, n)
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}
	if msg != nil {
		s.bus <- msg
		s.arch.write(msg)
	}
}

// Next retrieves the next message from the syslog queue.
func (s *Server) Next() *Message {
	select {
	case m := <-s.bus:
		return m
	case <-s.stop:
		return nil
	}
}

// Close closes the syslog server.
func (s *Server) Close() {
	s.stop <- true
	s.arch.CloseAll()
}

// NewServer creates and initializes a new syslog server process.
func NewServer(opts Options) *Server {
	s := new(Server)
	s.bus = make(chan *Message, 128*1024)

	if opts.SockAddr == "" {
		opts.SockAddr = defaultSockAddr
	}
	if opts.UnixPath == "" {
		opts.UnixPath = defaultUnixPath
	}
	if opts.BufferSize == 0 {
		opts.BufferSize = defaultBufferSize
	}
	s.opts = opts

	s.bus = make(chan *Message, opts.BufferSize)
	s.stop = make(chan bool)
	s.arch = newArchive(opts.LogDir)

	err := s.listenUnix()
	if err != nil {
		panic(err)
	}
	err = s.listenUDP()
	if err != nil {
		panic(err)
	}
	err = s.listenTCP()
	if err != nil {
		panic(err)
	}
	return s
}
