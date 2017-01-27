package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
)

// Path to JSON config files, first one found is used.
var configFiles = [...]string{
	"/etc/goweblogd.conf",
	"/etc/goweblogd/goweblogd.conf",
	"/usr/local/etc/goweblogd.conf",
	"/usr/local/etc/goweblogd/goweblogd.conf",
	"./goweblogd.conf",
	"./etc/goweblogd.conf"}

type cfg struct {
	File    string
	Redis   string   `json:"redis"`
	HTTP    string   `json:"http"`
	Syslogd []string `json:"syslogd"`
}

var config cfg

func openConfig() (f *os.File, err error) {
	for _, cf := range configFiles {
		f, err = os.Open(cf)
		if err == nil {
			return f, nil
		}
	}
	return
}

func newConfig() error {
	f, err := openConfig()
	if err != nil {
		return errors.New("No configuration file was found.")
	}
	defer f.Close()

	config.File = f.Name()

	b := new(bytes.Buffer)
	_, err = b.ReadFrom(f)
	if err != nil {
		return err
	}

	return json.Unmarshal(b.Bytes(), &config)
}
