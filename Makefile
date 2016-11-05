all:
	go build ./cmd/gosyslogd

test:
	go test -cover ./...

clean:
	@rm gosyslogd
