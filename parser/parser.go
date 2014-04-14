package parser

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"github.com/moovweb/rubex"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"time"
)

type Parser struct {
	tagmap map[string]*tagParser
}

type tagParser struct {
	entrymap  map[string]*Logent
	entryarr  logEntries
	totchecks int64
	Tag       string
}

type logEntries []*Logent

type Logent struct {
	rex   *rubex.Regexp
	raw   string
	count int64

	Md5       string
	Important int
}

// New reads the directory structure pointed to by path for match lists.
// The name of the file must be the same as the matched syslog tag, eg "cron", "sshd", etc.
// Each list contains a list of regexes.
func New(path string) (*Parser, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	p := new(Parser)
	p.tagmap = make(map[string]*tagParser)

	for _, file := range files {
		tp, err := p.newTagParser(path + "/" + file.Name())
		if err != nil {
			return nil, err
		}
		tp.Tag = file.Name()
		p.tagmap[tp.Tag] = tp
		fmt.Printf("Watching tag \"%s\" using %d entries.\n", file.Name(), len(tp.entryarr))
	}

	t0 = time.Now()
	return p, nil
}

func (p *Parser) newTagParser(fn string) (*tagParser, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}

	tp := new(tagParser)
	tp.entrymap = make(map[string]*Logent)

	s := bufio.NewScanner(f)
	i := 0
	for s.Scan() {
		i++
		line := s.Text()
		if line == "" || line[0] == '#' {
			continue
		}

		e := new(Logent)

		if strings.HasPrefix(line, "!!") {
			e.raw = strings.TrimPrefix(line, "!!")
			e.Important = 2
		} else if strings.HasPrefix(line, "!") {
			e.raw = strings.TrimPrefix(line, "!")
			e.Important = 1
		} else {
			e.raw = line
		}

		e.rex, err = rubex.Compile(line)
		if err != nil {
			return nil, fmt.Errorf("Error on line %d of %s: %v", i, fn, err)
		}

		h := md5.New()
		io.WriteString(h, line)
		e.Md5 = fmt.Sprintf("%x", h.Sum(nil))

		tp.entrymap[e.Md5] = e
	}

	tp.entryarr = make(logEntries, 0, len(tp.entrymap))
	for _, x := range tp.entrymap {
		tp.entryarr = append(tp.entryarr, x)
	}
	return tp, nil
}

var t0 time.Time

// HasTag checks if a regex match list is available for a given tag.
func (p *Parser) HasTag(tag string) bool {
	_, x := p.tagmap[tag]
	return x
}

// Check checks for a regex match for tag "tag".
// Returns Logent for the matched regex and true or false if matched.
func (p *Parser) Check(tag, msg string) (*Logent, bool) {
	tp, x := p.tagmap[tag]
	if !x {
		panic("Should be available.")
		return nil, false
	}

	tp.totchecks++
	if tp.totchecks%50000 == 0 {
		tp.optimize()
		//d := time.Now().Sub(t0)
		//fmt.Printf("50klines %v %.2f/s\n", d, float64(50000.0/d.Seconds()))
		//t0 = time.Now()
	}

	for i := 0; i < len(tp.entryarr); i++ {
		e := tp.entryarr[i]
		if e.rex.MatchString(msg) {
			e.count++
			return e, true
		}
	}
	return nil, false
}

// optimize sorts the logentries map into a sorted slice
// ordered by most used first.
func (tp *tagParser) optimize() {
	sort.Sort(tp.entryarr)
}

// Implement sort interface for log entries list.
func (e logEntries) Len() int {
	return len(e)
}

func (e logEntries) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func (e logEntries) Less(i, j int) bool {
	return e[j].count < e[i].count
}
