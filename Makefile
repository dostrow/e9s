BINARY := e9s
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build build-gui install clean test

build:
	go build $(LDFLAGS) -o $(BINARY) .

build-gui:
	go build -tags gui $(LDFLAGS) -o $(BINARY)-gui ./cmd/e9s-gui

install:
	go install $(LDFLAGS) .

test:
	go test ./...

clean:
	rm -f $(BINARY) $(BINARY)-gui

linux-amd64:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-linux-amd64 .

linux-arm64:
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-linux-arm64 .

darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-darwin-arm64 .

windows-amd64:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-windows-amd64.exe .
