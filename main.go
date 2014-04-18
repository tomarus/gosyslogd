package main

import (
	"./parser"
	"./syslogd"
	"database/sql"
	"fmt"
	"github.com/garyburd/redigo/redis"
	_ "github.com/lib/pq"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var parse *parser.Parser
var rdb redis.Conn
var sys *syslogd.Server

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

// MongoDB helper class

type moo struct {
	mongo *mgo.Session
	db    *mgo.Database
	cols  map[string]*mgo.Collection
}

var mongo moo

func (m *moo) collection(md5 string) *mgo.Collection {
	if coll, x := m.cols[md5]; x {
		return coll
	}

	newcol := m.db.C(md5)
	err := newcol.Create(&mgo.CollectionInfo{Capped: true, MaxBytes: 262144})
	if err != nil {
		panic(err)
	}

	m.cols[md5] = newcol
	return newcol
}

func (m *moo) Add(md5, rawmsg string) {
	col := m.collection(md5)
	err := col.Insert(bson.M{"epoch": time.Now().Unix(), "msg": rawmsg})
	if err != nil {
		panic(err)
	}
}

func (m *moo) Dial(addr string) (err error) {
	m.mongo, err = mgo.Dial(addr)
	if err != nil {
		return err
	}
	m.mongo.SetMode(mgo.Strong, true)
	m.db = m.mongo.DB("logsurfer")
	return nil
}

//

func main() {
	var err error

	// Load logsurfer filtering rules.
	parse, err = parser.New("/etc/gosyslogd")
	if err != nil {
		panic(err)
	}

	// Connect to Redis.
	rdb, err = redis.Dial("tcp", "localhost:6379")
	if err != nil {
		panic(err)
	}

	// Connect to PostgreSQL.
	err = psql.Dial("user=news dbname=greenlog sslmode=disable")
	if err != nil {
		panic(err)
	}

	// Connect to MongoDB.
	err = mongo.Dial("localhost")
	if err != nil {
		panic(err)
	}

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

		if !parse.HasTag(m.Tag) {
			continue
		}

		if logent, x := parse.Check(m.Tag, m.Raw); x {
			// Mached a regex entry.
			if logent.Important > 1 {
				psql.AddUnhandled(logent.Md5, m.Raw)
				rdb.Do("PUBLISH", "critical", m.Raw)
				mongo.Add(logent.Md5, m.Raw)
			}
		} else {
			// No match found.
			psql.AddUnhandled("00000000000000000000000000000000", m.Raw)
			rdb.Do("PUBLISH", "logging", m.Raw)
		}
	}
}
