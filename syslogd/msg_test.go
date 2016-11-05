package syslogd

import (
	"testing"
)

func TestMessage(t *testing.T) {
	pkt := []byte(`<27>2016-03-12T11:10:49+01:00 host001 processTag[12345]: payload`)
	testmsg, err := NewMessage(pkt, len(pkt))
	if err != nil {
		t.Fatal(err)
	}

	if testmsg.Tag != "processTag" {
		t.Error(`Expected Tag "processTag"`)
	}

	if testmsg.Pid != 12345 {
		t.Errorf("Expected Pid 12345, got %d", testmsg.Pid)
	}

	if testmsg.Hostname != "host001" {
		t.Errorf(`Expected Hostname "%s", got %s`, "host001", testmsg.Hostname)
	}

	// Should expand this test.
	if testmsg.PriorityString() != "daemon.err" {
		t.Errorf(`Expected priority "daemon.err", got %s`, testmsg.PriorityString())
	}
}

func TestTagpid(t *testing.T) {
	pkt := []byte(`<27>2016-03-12T11:10:49+01:00 processTag[12345]: payload`)
	testmsg, err := NewMessage(pkt, len(pkt))
	if err != nil {
		t.Fatal(err)
	}

	if testmsg.Tag != "processTag" {
		t.Error(`Expected Tag "processTag"`)
	}
	if testmsg.Hostname != hostname {
		t.Errorf(`Expected Hostname "%s", got %s`, hostname, testmsg.Hostname)
	}
}

func TestTag(t *testing.T) {
	pkt := []byte(`<27>2016-03-12T11:10:49+01:00 host001 processTag: payload`)
	testmsg, err := NewMessage(pkt, len(pkt))
	if err != nil {
		t.Fatal(err)
	}

	if testmsg.Tag != "processTag" {
		t.Error(`Expected Tag "processTag"`)
	}
	if testmsg.Hostname != "host001" {
		t.Errorf(`Expected Hostname "%s", got %s`, "host001", testmsg.Hostname)
	}
}

func TestUnparsable(t *testing.T) {
	pkt := []byte(`2016-03-12T11:10:49+01:00 host001 processTag[123]: payload`)
	testmsg, _ := NewMessage(pkt, len(pkt))

	if testmsg != nil {
		t.Log("Did not expect a valid message.")
	}
}

func TestMaybeParsable(t *testing.T) {
	t.Skip("RAW is invalid, size klopt niet, FIXME")
	// Something other weird going on during testing, we can't test indexbyte(">")
	// see msg.go:80 dunno why i created that check but somehow it was needed.

	pkt := []byte(`<27>2016-03-12T11:10:49+01:00 host001 processTag[123]: `)
	testmsg, err := NewMessage(pkt, len(pkt)) // Raw overflows
	if err != nil {
		t.Fatal(err)
	}

	if testmsg != nil {
		t.Log("Did not expect a valid message.")
	}
}
