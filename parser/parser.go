package parser

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Parser handles parsing of regexps to raw syslog messages.
type Parser struct {
	tagmap map[string]*tagParser
}

type tagParser struct {
	entrymap  map[string]*Logent
	entryarr  logEntries
	totchecks int64
	Tag       string
	filename  string
	lastmod   time.Time
}

type logEntries []*Logent

// Logent defines a single unique log entry. A unique Logent is defined as
// a tag, a hostname, a priority name or a regexp.
type Logent struct {
	rex   *regexp.Regexp
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

	go p.reloader()
	return p, nil
}

func (p *Parser) reloader() {
	for {
		time.Sleep(time.Second * 10)

		for _, tp := range p.tagmap {
			fi, err := os.Stat(tp.filename)
			if err != nil {
				panic(err)
			}

			if fi.ModTime().After(tp.lastmod) {
				newtp, err := p.newTagParser(tp.filename)
				if err != nil {
					panic(err)
				}
				newtp.Tag = tp.Tag
				p.tagmap[tp.Tag] = newtp
				fmt.Printf("Reloaded watched tag \"%s\" using %d entries.\n", tp.Tag, len(newtp.entryarr))
			}
		}
	}
}

func (p *Parser) newTagParser(fn string) (*tagParser, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tp := new(tagParser)
	tp.entrymap = make(map[string]*Logent)
	tp.filename = fn

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	tp.lastmod = fi.ModTime()

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

		e.rex, err = regexp.Compile(line)
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
		// return nil, false
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
