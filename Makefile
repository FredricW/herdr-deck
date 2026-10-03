BIN := bin/herdr-deck
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build test lint run demo clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/herdr-deck

test:
	go test ./...

# golangci-lint (with .golangci.yml) when installed, else gofmt + go vet.
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --config .golangci.yml ./...; \
	else \
		echo "golangci-lint not found; running gofmt and go vet"; \
		out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi; \
		go vet ./...; \
	fi

run: build
	./$(BIN) $(ARGS)

# The README's GIFs: a build stamped with a fixed version, then every tape
# in docs/demo through Charm's VHS (needs vhs, ttyd and ffmpeg). The tapes
# run --fake in a scratch home folder, $(DEMO_HOME), never your own.
# TAPES=docs/demo/news.tape renders only the tapes named.
DEMO_DIR := $(CURDIR)/bin/demo
DEMO_HOME := /tmp/herdr-deck-demo
DEMO_VERSION := v1.2.3

demo:
	rm -rf $(DEMO_DIR) && mkdir -p $(DEMO_DIR)
	go build -ldflags "-X main.version=$(DEMO_VERSION)" -o $(DEMO_DIR)/herdr-deck ./cmd/herdr-deck
	@for t in $${TAPES:-docs/demo/*.tape}; do \
		[ "$$t" = docs/demo/setup.tape ] && continue; \
		rm -rf $(DEMO_HOME) && mkdir -p $(DEMO_HOME)/.config/herdr-deck; \
		cp docs/demo/config.toml $(DEMO_HOME)/.config/herdr-deck/config.toml; \
		echo "vhs $$t"; DEMO_DIR=$(DEMO_DIR) DEMO_HOME=$(DEMO_HOME) vhs -q "$$t" || exit 1; \
	done
	rm -rf $(DEMO_HOME)

clean:
	rm -rf bin
