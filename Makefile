SHELL := /bin/sh

.PHONY: help build test test-unit test-integration test-race lint fmt tidy clean trafficgen trafficgen-docker

GO ?= go

help:
	@echo "Available targets:"
	@echo "  make build             - Build all packages"
	@echo "  make test              - Run full test suite"
	@echo "  make test-unit         - Run unit-focused tests"
	@echo "  make test-integration  - Run integration-focused tests"
	@echo "  make test-race         - Run tests with race detector"
	@echo "  make lint              - Run go vet"
	@echo "  make fmt               - Format all Go files"
	@echo "  make tidy              - Tidy Go modules"
	@echo "  make clean             - Clean test cache"
	@echo "  make trafficgen        - Run synthetic traffic generator"
	@echo "  make trafficgen-docker - Run traffic generator in Docker profile"

build:
	$(GO) build ./...

test:
	$(GO) test ./... -count=1

# Unit-focused set: excludes API router integration test package selection.
test-unit:
	$(GO) test ./internal/domain ./internal/config ./internal/service/transaction ./internal/api/errors ./internal/api/handlers ./internal/api/middleware -count=1

# Integration-focused set: API integration-style router tests.
test-integration:
	$(GO) test ./internal/api -run Router_ -count=1 -v

test-race:
	$(GO) test ./... -race -count=1

lint:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

clean:
	$(GO) clean -testcache

trafficgen:
	$(GO) run ./cmd/trafficgen $(ARGS)

trafficgen-docker:
	docker compose --profile trafficgen up --build trafficgen
