package syslogd

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"path"
	"sync"
	"syscall"
	"time"
)

const logpath = "/var/log/syslogd"

type archfile struct {
	buf  *bufio.Writer
	file *os.File
	sync bool
	last time.Time
	mu   sync.Mutex
}

type archive struct {
	files map[string]*archfile
}

func NewArchive() *archive {
	a := new(archive)
	a.files = make(map[string]*archfile)
	go a.syncer()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP)
	go a.reloader(sig)
	return a
}

func (a *archive) reloader(sig chan os.Signal) {
	for {
		<-sig
		fmt.Printf("Received signal, reopening files.\n")
		a.CloseAll()
	}
}

// Sync flushes the file buffer if the last message was received more than 2 seconds ago.
func (af *archfile) Sync() {
	if af.sync && af.last.Add(2*time.Second).Before(time.Now()) {
		af.mu.Lock()
		af.buf.Flush()
		af.mu.Unlock()
	}
}

// CheckClose checks if the last received message was more than 2 minutes ago and closes itself.
// Returns true if the file was closed, false otherwise.
func (af *archfile) CheckClose() bool {
	// Close if latest log is < 2 minute ago.
	if af.last.Add(2 * time.Minute).Before(time.Now()) {
		af.buf.Flush()
		af.file.Close()
		return true
	}
	return false
}

func (a *archive) CloseAll() {
	for fn, f := range a.files {
		f.buf.Flush()
		f.file.Close()
		delete(a.files, fn)
	}
}

func (a *archive) syncer() {
	for {
		<-time.After(5 * time.Second)
		for fn, af := range a.files {
			af.Sync()
			if af.CheckClose() {
				delete(a.files, fn)
			}
		}
	}
}

func (a *archive) write(m *Message) {
	// XXX make configurable.
	//fn := fmt.Sprintf("%s/%s.%s.log", logpath, m.Facility(), m.Severity())
	//fn := fmt.Sprintf("%s/%04d/%02d/%02d/%s/%s.%s.log", logpath, time.Now().Year(), time.Now().Month(), time.Now().Day(), m.Hostname, m.Facility(), m.Severity())
	fn := fmt.Sprintf("%s/%04d/%02d/%02d/%s.log", logpath, time.Now().Year(), time.Now().Month(), time.Now().Day(), m.Hostname)

	if _, x := a.files[fn]; !x {
		os.MkdirAll(path.Dir(fn), 0755)
		f, err := os.OpenFile(fn, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0664)
		if err != nil {
			panic(err)
		}
		a.files[fn] = &archfile{buf: bufio.NewWriter(f), file: f}
	}

	a.files[fn].write(m)
}

func (af *archfile) write(m *Message) {
	af.mu.Lock()
	_, err := af.buf.WriteString(m.Raw)
	if err != nil {
		panic(err)
	}
	_, err = af.buf.WriteString("\n")
	if err != nil {
		panic(err)
	}
	af.mu.Unlock()
	af.last = time.Now()
	af.sync = true
}
