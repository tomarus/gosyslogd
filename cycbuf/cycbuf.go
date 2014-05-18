// Package cycbuf implements cyclic storage.
// Each cycbuf contains a map of cycfiles.
// Each cycfile stores a fixed amount of log lines.
// A cycbuf file is allocated for each md5 sum, i.e.
// Tags, Hostnames, Priority & Regex match.

package cycbuf

import (
	"../syslogd"
	"code.google.com/p/go.net/websocket"
	"crypto/md5"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq"
	"io"
	"net/http"
	"strconv"
)

// Store up to cycbuflen messages in a cycfile.
const cycbuflen = 1024

type cycfile struct {
	msgs [cycbuflen]*syslogd.Message
	ptr  int
	loop bool
	subs map[*websocket.Conn]chan *syslogd.Message
}

type Cycbuf struct {
	files map[string]*cycfile
	sums  map[string]string
}

func (cf *cycfile) Init() {
	cf.subs = make(map[*websocket.Conn]chan *syslogd.Message)
	cf.loop = false
	cf.ptr = 0
}

func (cf *cycfile) AddMsg(m *syslogd.Message) {
	cf.msgs[cf.ptr] = m
	cf.ptr++
	if cf.ptr > cycbuflen-1 {
		cf.ptr = 0
		cf.loop = true
	}
	cf.publish(m)
}

// Range returns a full range slice of X syslogd messages.
func (cf *cycfile) Range() []*syslogd.Message {
	s1 := cf.msgs[0:cf.ptr]
	if cf.loop {
		s2 := cf.msgs[cf.ptr+1 : len(cf.msgs)]
		return append(s2, s1...)
	}

	return s1
}

func (cf *cycfile) Last(max int) []*syslogd.Message {
	if max == 0 {
		max = cycbuflen
	}

	r := cf.Range()
	if len(r) < max {
		max = len(r)
	}

	newrange := make([]*syslogd.Message, max)
	for i,j := len(r)-1, 0; j<max; i, j = i-1, j+1 {
		newrange[j] = r[i]
	}
	return newrange
}

func (cf *cycfile) publish(m *syslogd.Message) {
	for ws, ch := range cf.subs {
		select {
		case ch <- m:
		default:
			cf.Unsubscribe(ws)
			close(ch)
		}
	}
}

func (cf *cycfile) Subscribe(ws *websocket.Conn, mchan chan *syslogd.Message) error {
	cf.subs[ws] = mchan
	return nil
}

func (cf *cycfile) Unsubscribe(ws *websocket.Conn) error {
	delete(cf.subs, ws)
	return nil
}

func New() *Cycbuf {
	c := new(Cycbuf)
	c.files = make(map[string]*cycfile)
	c.sums = make(map[string]string)
	return c
}

func (cb *Cycbuf) AddString(str string, m *syslogd.Message) {
	sum, x := cb.sums[str]
	if !x {
		h := md5.New()
		io.WriteString(h, str)
		cb.sums[str] = fmt.Sprintf("%x", h.Sum(nil))
		sum = cb.sums[str]
	}

	cf, x := cb.files[sum]
	if !x {
		cb.files[sum] = new(cycfile)
		cb.files[sum].Init()
		cf = cb.files[sum]
	}
	cf.AddMsg(m)
}

func (cb *Cycbuf) Add(sum string, m *syslogd.Message) {
	cf, x := cb.files[sum]
	if !x {
		cb.files[sum] = new(cycfile)
		cb.files[sum].Init()
		cf = cb.files[sum]
	}
	cf.AddMsg(m)
}

// Call on program exit or signal.
func (cb *Cycbuf) Dump() {
}

// Call on program load.
func (cb *Cycbuf) Restore() {
}

// HttpLog returns the last "max" messages of "md5" in json format.
func (cb *Cycbuf) HttpLog(w http.ResponseWriter, r *http.Request) {
	sum := r.FormValue("md5")
	max := r.FormValue("max")

	cf, x := cb.files[sum]
	if !x {
		http.NotFound(w, r)
		return
	}

	var imax int64 = 0
	if max != "" {
		imax, _ = strconv.ParseInt(max, 10, 64)
	}

	lines := cf.Last(int(imax))

	b, err := json.Marshal(&lines)
	if err != nil {
		panic(err)
	}
	w.Write(b)
}

func (cb *Cycbuf) HttpStream(ws *websocket.Conn) {
	c := ws.Config()
	v := c.Location.Query()
	md5 := v.Get("md5")

	cf, x := cb.files[md5]
	if !x {
		fmt.Fprintf(ws, "No such sum %s", md5)
		return
	}

	ch := make(chan *syslogd.Message, 1)
	cf.Subscribe(ws, ch)

	for {
		select {
		case m := <-ch:
			b, err := json.Marshal(&m)
			if err != nil {
				panic(err)
			}
			_, err = ws.Write(b)
			if err != nil {
				cf.Unsubscribe(ws)
				close(ch)
				return
			}
		}
	}
}
