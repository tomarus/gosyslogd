package syslogd

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"
)

const logpath = "/var/log/syslogd"

type archfile struct {
	buf  *bufio.Writer
	file *os.File
}

type archive struct {
	files map[string]*archfile
	sync  map[string]bool
}

func NewArchive() *archive {
	a := new(archive)
	a.files = make(map[string]*archfile)
	a.sync = make(map[string]bool)
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
		a.flush()
		a.close()
	}
}

func (a *archive) flush() {
	for _, f := range a.files {
		f.buf.Flush()
	}
}

func (a *archive) close() {
	for fn, f := range a.files {
		f.buf.Flush()
		f.file.Close()
		delete(a.files, fn)
	}
}

func (a *archive) syncer() {
	for {
		<-time.After(2 * time.Second)
		for fn, _ := range a.files {
			if a.sync[fn] == true {
				a.files[fn].buf.Flush()
				a.sync[fn] = false
			}
		}
	}
}

func (a *archive) write(m *Message) {
	// XXX make configurable, when using dates, reload each day
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

	a.files[fn].buf.WriteString(m.Raw)
	a.files[fn].buf.WriteString("\n")
	a.sync[fn] = true
}
