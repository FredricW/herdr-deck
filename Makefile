BIN := bin/herdr-deck

.PHONY: build test lint run clean

build:
	go build -o $(BIN) ./cmd/herdr-deck

test:
	go test ./...

# golangci-lint when installed, else gofmt + go vet.
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found; running gofmt and go vet"; \
		out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi; \
		go vet ./...; \
	fi

run: build
	./$(BIN) $(ARGS)

clean:
	rm -rf bin
