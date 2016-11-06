all:
	go build ./cmd/gosyslogd
	go build ./cmd/goweblogd

test:
	go test -cover ./...

clean:
	@rm gosyslogd goweblogd
