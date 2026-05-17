.PHONY: help tidy build test lint lint-auth cover generate docker run-init

GO ?= go
BIN ?= ./bin/dbil
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

help:
	@echo "Targets:"
	@echo "  tidy      - go mod tidy"
	@echo "  build     - build $(BIN)"
	@echo "  test      - run all tests with race detector + coverage"
	@echo "  lint      - golangci-lint run"
	@echo "  cover     - generate HTML coverage report"
	@echo "  generate  - run sqlc"
	@echo "  docker    - docker build -t dbil:dev ."
	@echo "  run-init  - run \$$(BIN) init in ./dbil-data"

tidy:
	$(GO) mod tidy

build:
	mkdir -p $(dir $(BIN))
	$(GO) build -trimpath -ldflags='$(LDFLAGS)' -o $(BIN) ./cmd/dbil

test:
	$(GO) test -race -count=1 -coverprofile=coverage.out ./...

lint:
	golangci-lint run

lint-auth:
	$(GO) run ./scripts/lint-auth

cover: test
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

generate:
	sqlc generate

docker:
	docker build -t dbil:dev .

run-init: build
	mkdir -p ./dbil-data
	DBIL_DATA_DIR=./dbil-data $(BIN) init
