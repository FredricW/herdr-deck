BIN := bin/herdr-deck
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build test lint run clean

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

clean:
	rm -rf bin
