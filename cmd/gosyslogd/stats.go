package main

import (
	"expvar"
)

type sysstats struct {
	tags, hosts, priority *expvar.Map
}

func newStats() *sysstats {
	s := &sysstats{}
	s.tags = expvar.NewMap("tags")
	s.hosts = expvar.NewMap("hosts")
	s.priority = expvar.NewMap("pri")
	return s
}

func (s *sysstats) Tag(tag string) {
	s.tags.Add(tag, 1)
}

func (s *sysstats) Host(host string) {
	s.hosts.Add(host, 1)
}

func (s *sysstats) Priority(pri string) {
	s.priority.Add(pri, 1)
}
