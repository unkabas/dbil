.PHONY: help tidy build test lint lint-auth cover generate docker image-size run-init web-deps web-build

GO   ?= go
NPM  ?= npm
BIN  ?= ./bin/dbil
WEB  ?= ./web
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

help:
	@echo "Targets:"
	@echo "  tidy      - go mod tidy"
	@echo "  web-deps  - install frontend dependencies (npm install in $(WEB))"
	@echo "  web-build - build the frontend bundle into $(WEB)/dist"
	@echo "  build     - build $(BIN) (depends on web-build so the binary"
	@echo "              ships with the SPA embedded)"
	@echo "  test      - run all tests with race detector + coverage"
	@echo "  lint      - golangci-lint run"
	@echo "  lint-auth - static check that every API handler is gated"
	@echo "  cover     - generate HTML coverage report"
	@echo "  generate  - run sqlc"
	@echo "  docker    - docker build -t dbil:dev ."
	@echo "  run-init  - explicitly bootstrap ./dbil-data with \$$(BIN) init"

tidy:
	$(GO) mod tidy

web-deps:
	cd $(WEB) && $(NPM) install --no-audit --no-fund

web-build: web-deps
	cd $(WEB) && $(NPM) run build

build: web-build
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

# Fails when the local image exceeds 30 MB (uncompressed manifest size).
# Spec target is ≤25 MB; 30 MB is the hard ceiling enforced in CI as well.
image-size:
	@SIZE=$$(docker image inspect dbil:dev --format '{{.Size}}'); \
	echo "dbil:dev = $$SIZE bytes ($$((SIZE/1024/1024)) MB)"; \
	if [ $$SIZE -gt $$((30*1024*1024)) ]; then \
		echo "FAIL: image > 30 MB"; exit 1; \
	fi

run-init: build
	mkdir -p ./dbil-data
	DBIL_DATA_DIR=./dbil-data $(BIN) init
