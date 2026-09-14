GO ?= go
VERSION ?= development
COMMIT ?= unknown
BUILD_DATE ?= unknown
BUILD_DIR ?= bin

VERSION_PACKAGE := github.com/regutierrez/traicr/internal/version
LDFLAGS := -X $(VERSION_PACKAGE).buildVersion=$(VERSION) \
	-X $(VERSION_PACKAGE).buildCommit=$(COMMIT) \
	-X $(VERSION_PACKAGE).buildDate=$(BUILD_DATE)

.PHONY: all build build-collector build-server fmt lint test test-race clean

all: lint test build

build: build-collector build-server

build-collector:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/traicr ./cmd/traicr

build-server:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/traicr-server ./cmd/traicr-server

fmt:
	$(GO) fmt ./...

lint:
	test -z "$$(gofmt -l .)"
	$(GO) vet ./...

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

clean:
	rm -rf $(BUILD_DIR) artifacts
