# Development version
LATEST_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
LATEST_VERSION := $(shell echo $(LATEST_TAG) | sed 's/^v//')
MAJOR := $(shell echo $(LATEST_VERSION) | cut -d. -f1)
MINOR := $(shell echo $(LATEST_VERSION) | cut -d. -f2)
PATCH := $(shell echo $(LATEST_VERSION) | cut -d. -f3 | grep -E '^[0-9]+$$' || echo "0")
NEXT_PATCH := $(shell echo $$(($(PATCH) + 1)))
DEV_VERSION := $(MAJOR).$(MINOR).$(NEXT_PATCH)-dev
LDFLAGS := -X main.version=$(DEV_VERSION)

.PHONY: all
all: build

.PHONY: help
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*775649' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the plugin binary
	@echo "Building finfocus-plugin-azure-public..."
	@go build -ldflags "$(LDFLAGS)" -o finfocus-plugin-azure-public ./cmd/finfocus-plugin-azure-public

.PHONY: test
test: ## Run all tests
	@echo "Running tests..."
	@go test -v ./...

.PHONY: lint
lint: ## Run golangci-lint
	@echo "Running linter..."
	@golangci-lint run ./...

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -f finfocus-plugin-azure-public
