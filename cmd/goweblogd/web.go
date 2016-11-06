package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"
)

func writeStack() {
	f, err := os.OpenFile("/var/log/dumps/goweblogd.trace", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Can't open file for stacktrace.")
	}
	defer f.Close()

	buf := make([]byte, 1048576)
	runtime.Stack(buf, true)
	fmt.Fprintf(f, string(buf))
}

func hh(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("FATAL: %v", err)
				writeStack()
			}
		}()
		t0 := time.Now()
		fn(w, r)
		t1 := time.Now()
		fmt.Printf("http_handler: GET %s %v.\n", r.URL, t1.Sub(t0))

		if t1.Sub(t0) > time.Second {
			fmt.Printf("Slow HTTP Request: GET %s %v.\n", r.URL, t1.Sub(t0))
		}
	}
}

func static(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "public"+r.URL.Path)
}

// XXX support multiple syslogds
var body []byte

func refreshSyslogd() {
	resp, err := http.Get(config.Syslogd[0] + "/debug/vars")
	if err != nil {
		fmt.Printf("Can't refresh syslog server; %v\n", err)
		return
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Can't refresh syslog server; %v\n", err)
		return
	}
}

func dataStats(w http.ResponseWriter, r *http.Request) {
	// XXX support multiple syslogds
	w.Write(body)
}

func dataLog(w http.ResponseWriter, r *http.Request) {
	md5 := r.FormValue("md5")
	max := r.FormValue("max")
	resp, err := http.Get(config.Syslogd[0] + "/log?md5=" + md5 + "&max=" + max)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	w.Write(body)
}

func httpInit() {
	http.HandleFunc("/data/stats", hh(dataStats))
	http.HandleFunc("/log", hh(dataLog))
	http.HandleFunc("/", hh(static))
	http.HandleFunc("/js", hh(static))

	go func() {
		for {
			refreshSyslogd()
			time.Sleep(3 * time.Second)
		}
	}()
}

func httpServer() (err error) {
	listener, err := net.Listen("tcp", config.HTTP)
	if err != nil {
		return err
	}

	return http.Serve(listener, nil)
}
