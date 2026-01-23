# Makefile for finfocus-plugin-azure-public

# Binary name
BINARY_NAME := finfocus-plugin-azure-public

# Version calculation
GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
GIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION := $(shell echo $(GIT_TAG) | sed 's/^v//')

# Logic for next version (simple patch increment for dev)
MAJOR := $(shell echo $(VERSION) | cut -d. -f1)
MINOR := $(shell echo $(VERSION) | cut -d. -f2)
PATCH := $(shell echo $(VERSION) | cut -d. -f3 | cut -d- -f1)
NEXT_PATCH := $(shell echo $$(($(PATCH) + 1)))
DEV_VERSION := $(MAJOR).$(MINOR).$(NEXT_PATCH)-dev

# Linker flags to inject version
LDFLAGS := -X main.version=$(DEV_VERSION)

.PHONY: all
all: help

.PHONY: build
build:
	@echo "Building $(BINARY_NAME) version $(DEV_VERSION)..."
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) ./cmd/finfocus-plugin-azure-public

.PHONY: test
test:
	@echo "Running tests..."
	go test -v -race ./...

.PHONY: lint
lint:
	@echo "Running lint..."
	golangci-lint run --timeout=10m ./...

.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)

.PHONY: ensure
ensure:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build   - Compile the binary with version information"
	@echo "  test    - Run unit tests with race detection"
	@echo "  lint    - Run code quality checks (golangci-lint)"
	@echo "  clean   - Remove build artifacts"
	@echo "  ensure  - Install development dependencies"
	@echo "  help    - Show this help message"