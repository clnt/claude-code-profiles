VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build test lint install clean

build:
	go build $(LDFLAGS) -o bin/ccp .

test:
	go test ./... -v -race -count=1

lint:
	golangci-lint run ./...

install: build
	cp bin/ccp /usr/local/bin/ccp

clean:
	rm -rf bin/ dist/
