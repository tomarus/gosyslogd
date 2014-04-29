package main

import (
	"./config"
	"./parser"
	"./syslogd"
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/garyburd/redigo/redis"
	_ "github.com/lib/pq"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var parse *parser.Parser
var rdb redis.Conn
var sys *syslogd.Server
var cyc *cycbuf

// Cycbuff storage. A cycbuf contains a map of cycfiles.
// Each cycfile stores a fixed amount of log lines.

// Store up to cycbuflen messages in a cycfile.
const cycbuflen = 1024

type cycfile struct {
	msgs [cycbuflen]*syslogd.Message
	ptr  int
	loop bool
}

type cycbuf struct {
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

func newCycbuf() *cycbuf {
	c := new(cycbuf)
	c.files = make(map[string]*cycfile)
	c.sums = make(map[string]string)
	return c
}

func (cb *cycbuf) AddString(str string, m *syslogd.Message) {
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

func (cb *cycbuf) Add(sum string, m *syslogd.Message) {
	cf, x := cb.files[sum]
	if !x {
		cb.files[sum] = new(cycfile)
		cf = cb.files[sum]
	}
	cf.AddMsg(m)
}

// Call on program exit or signal.
func (cb *cycbuf) Dump() {
}

// Call on program load.
func (cb *cycbuf) Restore() {
}

func (cb *cycbuf) HttpLog(w http.ResponseWriter, r *http.Request) {
	sum := r.FormValue("md5")
	cf, x := cb.files[sum]
	if !x {
		http.NotFound(w, r)
		return
	}
	lines := cf.Range()
	b, err := json.Marshal(&lines)
	if err != nil {
		panic(err)
	}
	w.Write(b)
}

// PostgreSQL helper class

type psqldb struct {
	db    *sql.DB
	table string
}

var psql psqldb

func (p *psqldb) Dial(connstr string) (err error) {
	p.db, err = sql.Open("postgres", connstr)
	if err != nil {
		return
	}

	p.tables()
	return nil
}

// XXX add a timer at first day of month 00:00 to change table name.
func (p *psqldb) tables() {
	tn := "log_" + time.Now().Format("200601")

	err := p.db.QueryRow("SELECT relname FROM pg_class WHERE relname= $1", tn).Scan(&p.table)
	if err != nil {
		panic(err)
	}

	if p.table != tn {
		p.db.Exec(fmt.Sprintf("CREATE TABLE %s (epoch int, match char(32), msg varchar)", tn))
		p.db.Exec(fmt.Sprintf("CREATE INDEX epochidx_%s ON %s(epoch)", tn, tn))
		p.db.Exec(fmt.Sprintf("CREATE INDEX matchidx_%s ON %s(match)", tn, tn))
	}
}

func (p *psqldb) AddUnhandled(md5, content string) (err error) {
	_, err = p.db.Exec(fmt.Sprintf("INSERT INTO %s (epoch, match, msg) VALUES($1,$2,$3)", p.table), time.Now().Unix(), md5, content)
	return err
}

//

func main() {
	var err error

	// Load config.
	err = config.NewConfig()
	if err != nil {
		panic(err)
	}

	// Load logsurfer filtering rules.
	parse, err = parser.New(config.C.RulesDir)
	if err != nil {
		panic(err)
	}

	// Connect to Redis.
	rdb, err = redis.Dial("tcp", config.C.Redis)
	if err != nil {
		panic(err)
	}

	// Connect to PostgreSQL.
	err = psql.Dial(config.C.Postgres)
	if err != nil {
		panic(err)
	}

	// Initialize in memory cyclic buffer cache.
	cyc = newCycbuf()

	// Start HTTP server.
	go Stats.HTTP()

	// Add last-x log lines output
	http.HandleFunc("/log", cyc.HttpLog)

	// Start syslog server.
	sys = syslogd.NewServer()
	go sysloop()

	// Wait for ctrl-c
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	<-sig

	sys.Close()
}

func sysloop() {
	for {
		m := sys.Next()
		if m == nil {
			fmt.Printf("No more messages, exiting.\n")
			sys.Close()
			os.Exit(-1)
		}

		Stats.Tag(m.Tag)
		Stats.Host(m.Hostname)
		Stats.Priority(m.PriorityString())

		cyc.AddString(m.Tag, m)
		cyc.AddString(m.Hostname, m)
		cyc.AddString(m.PriorityString(), m)

		// Parser & matching stuff

		if !parse.HasTag(m.Tag) {
			continue
		}

		if logent, x := parse.Check(m.Tag, m.Raw); x {
			// Mached a regex entry.
			if logent.Important > 1 {
				psql.AddUnhandled(logent.Md5, m.Raw)
				rdb.Do("PUBLISH", "critical", m.Raw)
			}
			cyc.Add(logent.Md5, m)
		} else {
			// No match found.
			psql.AddUnhandled("00000000000000000000000000000000", m.Raw)
			rdb.Do("PUBLISH", "logging", m.Raw)
		}
	}
}
