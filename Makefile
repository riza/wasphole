BINARY  := wasphole
CMD     := ./cmd/wasphole
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build run test vet lint clean help

build:
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

run: build
	./$(BINARY) -config config.yaml

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
