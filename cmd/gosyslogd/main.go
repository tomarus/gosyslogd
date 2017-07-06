package main

import (
	"flag"
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

var verbose = flag.Bool("v", false, "Log all unwanted messages to stdout.")
var tail = flag.Bool("tail", false, "Tail -f the unwanted log connecting to a running gosyslogd.")

const nullmd5 = "00000000000000000000000000000000"

func main() {
	var err error

	flag.Parse()

	// Load config.
	err = loadConfig()
	if err != nil {
		panic(err)
	}

	// Connect to Redis.
	rdb, err = redis.Dial("tcp", cfg.Redis)
	if err != nil {
		panic(err)
	}

	if *tail {
		if err := tailf(rdb); err != nil {
			panic(err)
		}
		return
	}

	// Load logsurfer filtering rules.
	parse, err = parser.New(cfg.RulesDir)
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
	sys = syslogd.NewServer(syslogd.Options{SockAddr: cfg.SockAddr, UnixPath: cfg.UnixPath, LogDir: cfg.LogDir})
	go sysloop()

	// Wait for ctrl-c
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	<-sig

	sys.Close()
}

func tailf(c redis.Conn) error {
	psc := redis.PubSubConn{Conn: c}
	psc.Subscribe("logging")
	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			fmt.Printf("%s\n", v.Data)
		case redis.Subscription:
			// nothing
		case error:
			return v
		}
	}
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
				psql.AddUnhandled(nullmd5, string(m.Raw))
			}
			cyc.Add(nullmd5, m)
			rdb.Do("PUBLISH", "logging", m.Raw)

			if *verbose {
				fmt.Println(string(m.Raw))
			}
		}

	}
}
