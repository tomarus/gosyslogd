package syslogd

import (
	"bufio"
	"net"
	"os"
)

const unix_path string = "/dev/log"
const sock_addr string = ":514"

type Server struct {
	tcp  *net.TCPConn
	udp  *net.UDPConn
	unix *net.UnixConn
	bus  chan *Message
	stop chan bool
	arch *archive
}

func (s *Server) listenUDP() error {
	a, err := net.ResolveUDPAddr("udp", sock_addr)
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
	os.Remove(unix_path)
	a, err := net.ResolveUnixAddr("unixgram", unix_path)
	if err != nil {
		return err
	}
	s.unix, err = net.ListenUnixgram("unixgram", a)
	if err != nil {
		return err
	}
	err = os.Chmod(unix_path, 0666)
	if err != nil {
		return err
	}
	go s.receivePacket(s.unix)
	return nil
}

func (s *Server) listenTCP() error {
	sock, err := net.Listen("tcp", sock_addr)
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
		n, _, err := con.ReadFrom(buf)
		if err != nil {
			return
		}
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
		s.processBuf(buf, 0)
	}
}

func (s *Server) processBuf(b []byte, n int) {
	msg := NewMessage(b, n)
	if msg != nil {
		s.arch.write(msg)
		s.bus <- msg
	}
}

func (s *Server) Next() *Message {
	select {
	case m := <-s.bus:
		return m
	case <-s.stop:
		return nil
	}
}

func (s *Server) Close() {
	s.stop <- true
	s.arch.CloseAll()
}

func NewServer() *Server {
	s := new(Server)
	s.bus = make(chan *Message, 128*1024)
	s.stop = make(chan bool)
	s.arch = NewArchive()

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
