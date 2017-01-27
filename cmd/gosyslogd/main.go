package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/garyburd/redigo/redis"
	_ "github.com/lib/pq"
	"github.com/tomarus/gosyslogd/cycbuf"
	"github.com/tomarus/gosyslogd/parser"
	"github.com/tomarus/gosyslogd/syslogd"
	"golang.org/x/net/websocket"
)

var parse *parser.Parser
var rdb redis.Conn
var sys *syslogd.Server
var cyc *cycbuf.Cycbuf
var stats *sysstats

func main() {
	var err error

	// Load config.
	err = loadConfig()
	if err != nil {
		panic(err)
	}

	// Load logsurfer filtering rules.
	parse, err = parser.New(cfg.RulesDir)
	if err != nil {
		panic(err)
	}

	// Connect to Redis.
	rdb, err = redis.Dial("tcp", cfg.Redis)
	if err != nil {
		panic(err)
	}

	// Connect to PostgreSQL.
	if cfg.Postgres != "" {
		err = psql.Dial(cfg.Postgres)
		if err != nil {
			panic(err)
		}
	}

	// Initialize in memory cyclic buffer cache.
	cyc = cycbuf.New()

	stats = newStats()

	// Start HTTP server.
	go func() {
		err := http.ListenAndServe(cfg.HTTP, nil)
		if err != nil {
			panic(err)
		}
	}()

	// Add last-x log lines output
	http.HandleFunc("/log", cyc.HttpLog)
	http.Handle("/stream", websocket.Handler(cyc.HttpStream))

	// Start syslog server.
	sys = syslogd.NewServer(syslogd.Options{SockAddr: cfg.SockAddr, UnixPath: cfg.UnixPath, Archive: cfg.Archive})
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

		stats.Tag(m.Tag)
		stats.Host(m.Hostname)
		stats.Priority(m.PriorityString())

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
				if cfg.Postgres != "" {
					psql.AddUnhandled(logent.Md5, string(m.Raw))
				}
				rdb.Do("PUBLISH", "critical", m.Raw)
			}
			cyc.Add(logent.Md5, m)
		} else {
			// No match found.
			if cfg.Postgres != "" {
				psql.AddUnhandled("00000000000000000000000000000000", string(m.Raw))
			}
			cyc.Add("00000000000000000000000000000000", m)
			rdb.Do("PUBLISH", "logging", m.Raw)
		}
	}
}
