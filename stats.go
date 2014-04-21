package main

import (
	"./config"
	"expvar"
	"net"
	"net/http"
)

type stats struct {
	listener net.Listener
}

var Stats stats

var tags = expvar.NewMap("tags")
var hosts = expvar.NewMap("hosts")
var priority = expvar.NewMap("pri")

func (s *stats) HTTP() (err error) {
	s.listener, err = net.Listen("tcp", config.C.HTTP)
	if err != nil {
		panic(err)
	}

	err = http.Serve(s.listener, nil)
	if err != nil {
		panic(err)
	}

	return
}

func (s *stats) Tag(tag string) {
	tags.Add(tag, 1)
}

func (s *stats) Host(host string) {
	hosts.Add(host, 1)
}

func (s *stats) Priority(pri string) {
	priority.Add(pri, 1)
}
