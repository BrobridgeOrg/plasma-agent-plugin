BINARY := bin/plasma-plugin-mcp
VERSION := $(shell cat VERSION)
MODULE := github.com/BrobridgeOrg/plasma-plugin
LDFLAGS := -X $(MODULE)/internal/plasmamcp.Version=$(VERSION) -X $(MODULE)/internal/ophionproxy.Version=$(VERSION)

.PHONY: build test fmt vet check clean release test-launcher test-config

build:
	go build -ldflags '$(LDFLAGS)' -o $(BINARY) ./cmd/plasma-plugin-mcp

release:
	bash scripts/build-release.sh

test-launcher: build
	python3 scripts/test-launcher.py

test-config:
	python3 scripts/test-config.py

test:
	go test ./...

fmt:
	gofmt -l -w cmd internal

vet:
	go vet ./...

check: fmt vet test test-launcher test-config

clean:
	rm -f $(BINARY)
