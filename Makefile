BINARY := bin/plasma-plugin-mcp

.PHONY: build test fmt vet check clean

build:
	go build -o $(BINARY) ./cmd/plasma-plugin-mcp

test:
	go test ./...

fmt:
	gofmt -l -w cmd internal

vet:
	go vet ./...

check: fmt vet test build

clean:
	rm -f $(BINARY)
