BINARY  := wasphole
CMD     := ./cmd/wasphole
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
TAG     := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "none")
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.tag=$(TAG) -X main.buildDate=$(DATE)"

.PHONY: build run test vet lint clean help

build:
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

run: build
	./$(BINARY) server -config config.yaml

test:
	go test ./...

vet:
	go vet ./...

lint: vet
	go build ./...

clean:
	rm -f $(BINARY)
	rm -rf cache/ sessions/

help:
	@echo "build   — compile binary"
	@echo "run     — build and run with config.yaml"
	@echo "test    — run tests"
	@echo "vet     — go vet"
	@echo "lint    — vet + build check"
	@echo "clean   — remove binary, cache, sessions"
