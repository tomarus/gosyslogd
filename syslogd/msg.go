package syslogd

import (
	"bytes"
	"fmt"
	"github.com/moovweb/rubex"
	"log/syslog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Message defines all parsable fields of a syslog message.
// Not all fields might be available in each message.
type Message struct {
	Received time.Time
	Priority syslog.Priority
	Hostname string
	Tag      string
	Pid      int
	Raw      string
}

var sysfmt *rubex.Regexp
var mu sync.Mutex
var hostname string

func init() {
	sysfmt = rubex.MustCompile("<([0-9]+)>(.{15}|.{25}) (.*?): (.*)")
	h, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	hostname = h
}

func NewMessage(pkt []byte, size int) *Message {
	mu.Lock()
	res := sysfmt.FindSubmatch(pkt)
	mu.Unlock()
	if len(res) != 5 {
		fmt.Printf("Cant parse: %d %s\n", len(res), string(pkt))
		return nil
	}

	msg := new(Message)
	msg.Received = time.Now()

	p, _ := strconv.ParseInt(string(res[1]), 10, 64)
	msg.Priority = syslog.Priority(p)
	// received timestamp = res[2]

	tagpid := ""
	misc := res[3]
	// Check for either "hostname tagpid" or "tagpid"
	a := bytes.SplitN(misc, []byte(" "), 2)
	if len(a) == 2 {
		msg.Hostname = string(a[0])
		tagpid = string(a[1])
	} else {
		msg.Hostname = hostname
		tagpid = string(a[0])
	}

	// tagpid is either "tag[pid]" or just "tag".
	if n := strings.Index(tagpid, "["); n > 0 {
		p, _ = strconv.ParseInt(tagpid[n+1:(len(tagpid)-1)], 10, 64)
		msg.Pid = int(p)
		msg.Tag = tagpid[:n]
	} else {
		msg.Tag = tagpid
	}

	// Raw string excluding priority including timestamp.
	n := bytes.IndexByte(pkt, '>')
	if n > 0 {
		if size > 0 {
			msg.Raw = strings.TrimSpace(string(pkt[n+1 : size]))
		} else {
			msg.Raw = strings.TrimSpace(string(pkt[n+1:]))
		}
	} else {
		msg.Raw = strings.TrimSpace(string(pkt))
	}

	return msg
}

var severeties = [...]string{
	"emerg", "alert", "crit", "err", "warning", "notice", "info", "debug",
}

var facilities = [...]string{
	"kern", "user", "mail", "daemon", "auth", "syslog", "lpr", "news",
	"uucp", "cron", "authpriv", "ftp", "unknown", "unknown", "unknown", "unknown",
	"local0", "local1", "local2", "local3", "local4", "local5", "local6", "local7",
}

func (m *Message) Severity() string {
	return severeties[m.Priority&7]
}

func (m *Message) Facility() string {
	return facilities[m.Priority>>3]
}

func (m *Message) PriorityString() string {
	return m.Facility() + "." + m.Severity()
}
