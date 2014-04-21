package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
)

// Path to JSON config files, first one found is used.
var configFiles = [...]string{
	"/etc/gosyslogd.conf",
	"/etc/gosyslogd/gosyslogd.conf",
	"/usr/local/etc/gosyslogd.conf",
	"/usr/local/etc/gosyslogd/gosyslogd.conf",
	"./gosyslogd.conf",
	"./etc/gosyslogd.conf"}

type config struct {
	File      string
	UnixPath  string `json:"unixpath"`
	SockAddr  string `json:"sockaddr"`
	RulesDir  string `json:"rules"`
	LogDir    string `json:"logdir"`
	Redis     string `json:"redis"`
	Postgres  string `json:"postgres"`
	MongoHost string `json:"mongohost"`
	MongoColl string `json:"mongocoll"`
	HTTP      string `json:"http"`
}

var C config

func openConfig() (f *os.File, err error) {
	for _, cf := range configFiles {
		f, err = os.Open(cf)
		if err == nil {
			return f, nil
		}
	}
	return
}

func NewConfig() error {
	f, err := openConfig()
	if err != nil {
		return errors.New("No configuration file was found.")
	}
	defer f.Close()

	C.File = f.Name()

	b := new(bytes.Buffer)
	_, err = b.ReadFrom(f)
	if err != nil {
		return err
	}

	err = json.Unmarshal(b.Bytes(), &C)
	if err != nil {
		return err
	}

	return nil
}
