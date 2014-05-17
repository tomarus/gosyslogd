package cycbuf

import (
	"../syslogd"
	"crypto/md5"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq"
	"io"
	"net/http"
	"strconv"
)

// Cycbuff storage. A cycbuf contains a map of cycfiles.
// Each cycfile stores a fixed amount of log lines.

// Store up to cycbuflen messages in a cycfile.
const cycbuflen = 1024

type cycfile struct {
	msgs [cycbuflen]*syslogd.Message
	ptr  int
	loop bool
}

type Cycbuf struct {
	files map[string]*cycfile
	sums  map[string]string
}

func (cf *cycfile) AddMsg(m *syslogd.Message) {
	cf.msgs[cf.ptr] = m
	cf.ptr++
	if cf.ptr > cycbuflen-1 {
		cf.ptr = 0
		cf.loop = true
	}
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
		cf = cb.files[sum]
	}
	cf.AddMsg(m)
}

func (cb *Cycbuf) Add(sum string, m *syslogd.Message) {
	cf, x := cb.files[sum]
	if !x {
		cb.files[sum] = new(cycfile)
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
	lines := cf.Range()
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}

	if max != "" {
		imax, _ := strconv.ParseInt(max, 10, 64)
		lines = lines[0:imax]
	}

	b, err := json.Marshal(&lines)
	if err != nil {
		panic(err)
	}
	w.Write(b)
}
