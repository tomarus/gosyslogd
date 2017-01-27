package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

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
