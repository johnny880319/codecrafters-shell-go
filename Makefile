APP_NAME := my_shell
BINARY := bin/$(APP_NAME)
GO ?= go
PKG := ./...
GOLANGCI_LINT ?= golangci-lint
CONTAINER_RUNTIME ?= docker

CACHE_DIR := $(CURDIR)/.cache
GO_BUILD_CACHE := $(CACHE_DIR)/go-build
GO_MOD_CACHE := $(CACHE_DIR)/gomod
GOLANGCI_CACHE := $(CACHE_DIR)/golangci-lint
GO_ENV := GOCACHE="$(GO_BUILD_CACHE)" GOMODCACHE="$(GO_MOD_CACHE)"

.PHONY: help prepare-cache fmt fmt-check vet lint test test-race build run clean ci pre-commit docker-build docker-run

prepare-cache:
	@mkdir -p $(GO_BUILD_CACHE) $(GO_MOD_CACHE) $(GOLANGCI_CACHE)

help:
	@echo "Available targets:"
	@echo "  make fmt         - Run gofmt for all Go packages"
	@echo "  make fmt-check   - Fail if gofmt changes are needed"
	@echo "  make vet         - Run go vet"
	@echo "  make lint        - Run golangci-lint with local cache"
	@echo "  make test        - Run unit tests"
	@echo "  make test-race   - Run tests with the race detector"
	@echo "  make build       - Build ./cmd/$(APP_NAME) into $(BINARY)"
	@echo "  make ci          - Run the same checks as CI"
	@echo "  make docker-build - Build container image"
	@echo "  make docker-run  - Run container image interactively"

fmt:
	@$(MAKE) prepare-cache
	@$(GO_ENV) $(GO) fmt $(PKG)

fmt-check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "Run 'make fmt' to format files:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

vet:
	@$(MAKE) prepare-cache
	@$(GO_ENV) $(GO) vet $(PKG)

lint:
	@$(MAKE) prepare-cache
	@$(GO_ENV) GOLANGCI_LINT_CACHE="$(GOLANGCI_CACHE)" $(GOLANGCI_LINT) run --timeout=5m

test:
	@$(MAKE) prepare-cache
	@$(GO_ENV) $(GO) test $(PKG)

test-race:
	@$(MAKE) prepare-cache
	@$(GO_ENV) $(GO) test -race -covermode=atomic $(PKG)

build:
	@$(MAKE) prepare-cache
	@mkdir -p bin
	@$(GO_ENV) $(GO) build -trimpath -o $(BINARY) ./cmd/$(APP_NAME)

run:
	@$(MAKE) prepare-cache
	@$(GO_ENV) $(GO) run ./cmd/$(APP_NAME)

ci: fmt-check vet lint test-race build

pre-commit:
	@pre-commit run --all-files

clean:
	@rm -rf bin $(CACHE_DIR)
	@rm -f /tmp/codecrafters-build-shell-go

docker-build:
	@$(CONTAINER_RUNTIME) build -t $(APP_NAME):dev .

docker-run:
	@$(CONTAINER_RUNTIME) run --rm -it $(APP_NAME):dev
