package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/garyburd/redigo/redis"
	_ "github.com/lib/pq"
)

var rdb redis.Conn

func main() {
	// Load config.
	err := newConfig()
	if err != nil {
		panic(err)
	}

	// Connect to Redis.
	rdb, err = redis.Dial("tcp", config.Redis)
	if err != nil {
		panic(err)
	}

	// Start HTTP server.
	httpInit()
	go httpServer()

	// Wait for ctrl-c
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	<-sig
}
