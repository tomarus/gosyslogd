package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/garyburd/redigo/redis"
	_ "github.com/lib/pq"
	"github.com/tomarus/gosyslogd/config"
	"github.com/tomarus/gosyslogd/cycbuf"
	"github.com/tomarus/gosyslogd/parser"
	"github.com/tomarus/gosyslogd/syslogd"
	"golang.org/x/net/websocket"
)

var parse *parser.Parser
var rdb redis.Conn
var sys *syslogd.Server
var cyc *cycbuf.Cycbuf

type psqlmsg struct {
	md5     string
	content string
}

type psqldb struct {
	db     *sql.DB
	table  string
	msgbus chan *psqlmsg
}

var psql psqldb

func (p *psqldb) Dial(connstr string) (err error) {
	p.db, err = sql.Open("postgres", connstr)
	if err != nil {
		return
	}

	p.tables()
	go p.changeTables()

	p.msgbus = make(chan *psqlmsg, 1024*1024)
	go p.msgReader()
	return nil
}

func (p *psqldb) changeTables() {
	ticker := p.updateTicker()
	for {
		<-ticker.C
		p.tables()
		ticker = p.updateTicker()
	}
}

func (p *psqldb) updateTicker() *time.Ticker {
	// Get current first day of month 00:00
	nextTick := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Local)
	// Add 1 month
	nextTick = nextTick.AddDate(0, 1, 0)

	diff := nextTick.Sub(time.Now())
	return time.NewTicker(diff)
}

func (p *psqldb) tables() {
	tn := "log_" + time.Now().Format("200601")

	err := p.db.QueryRow("SELECT relname FROM pg_class WHERE relname= $1", tn).Scan(&p.table)
	if err != nil && err != sql.ErrNoRows {
		panic(err)
	}

	if p.table != tn {
		p.db.Exec(fmt.Sprintf("CREATE TABLE %s (epoch int, match char(32), msg varchar)", tn))
		p.db.Exec(fmt.Sprintf("CREATE INDEX epochidx_%s ON %s(epoch)", tn, tn))
		p.db.Exec(fmt.Sprintf("CREATE INDEX matchidx_%s ON %s(match)", tn, tn))
	}
}

func (p *psqldb) msgReader() {
	for {
		select {
		case m := <-p.msgbus:
			_, err := p.db.Exec(fmt.Sprintf("INSERT INTO %s (epoch, match, msg) VALUES($1,$2,$3)", p.table), time.Now().Unix(), m.md5, m.content)
			if err != nil {
				panic(err)
			}
		}
	}
}

func (p *psqldb) AddUnhandled(md5, content string) (err error) {
	p.msgbus <- &psqlmsg{md5: md5, content: content}
	return nil
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
	cyc = cycbuf.New()

	// Start HTTP server.
	go Stats.HTTP()

	// Add last-x log lines output
	http.HandleFunc("/log", cyc.HttpLog)
	http.Handle("/stream", websocket.Handler(cyc.HttpStream))

	// Start syslog server.
	sys = syslogd.NewServer(syslogd.Options{SockAddr: config.C.SockAddr, UnixPath: config.C.UnixPath})
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

		if logent, x := parse.Check(m.Tag, string(m.Raw)); x {
			// Mached a regex entry.
			if logent.Important > 1 {
				psql.AddUnhandled(logent.Md5, string(m.Raw))
				rdb.Do("PUBLISH", "critical", m.Raw)
			}
			cyc.Add(logent.Md5, m)
		} else {
			// No match found.
			psql.AddUnhandled("00000000000000000000000000000000", string(m.Raw))
			cyc.Add("00000000000000000000000000000000", m)
			rdb.Do("PUBLISH", "logging", m.Raw)
		}
	}
}
