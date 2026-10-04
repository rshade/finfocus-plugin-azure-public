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

.PHONY: spec-check
spec-check:
	@echo "Checking OpenSpec specs..."
	openspec validate --all --strict --no-interactive
	bash scripts/check-spec-tests.sh

.PHONY: vet
vet:
	@echo "Running go vet..."
	go vet ./...

.PHONY: vulncheck
vulncheck:
	@echo "Running govulncheck..."
	govulncheck ./...

.PHONY: lint
lint: vet
	@echo "Running golangci-lint..."
	golangci-lint run --timeout=10m ./...
	@echo "Running markdownlint..."
	markdownlint '*.md' 2>/dev/null || true
	@echo "Running openspec validate..."
	openspec validate --all --strict --no-interactive
	@echo "Running vale..."
	vale --config=.vale.ini --glob='!{.claude/skills/**,.claude/commands/opsx/**,.gemini/**,.opencode/**}' . 2>/dev/null || true
	@echo "Running actionlint..."
	actionlint .github/workflows/ 2>/dev/null || true

.PHONY: fmt
fmt:
	@echo "Running gofmt..."
	gofmt -s -w .

.PHONY: goreleaser-check
goreleaser-check:
	@echo "Checking goreleaser config..."
	goreleaser check

.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)

.PHONY: ensure
ensure:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.9.0

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build              - Compile the binary with version information"
	@echo "  test               - Run unit tests with race detection"
	@echo "  vet                - Run go vet"
	@echo "  vulncheck          - Run govulncheck for security vulnerabilities"
	@echo "  lint               - Run all linters (vet, golangci-lint, markdownlint, openspec, vale, actionlint)"
	@echo "  spec-check         - Validate OpenSpec specs and run the tests each requirement names"
	@echo "  fmt                - Format code with gofmt"
	@echo "  goreleaser-check   - Verify goreleaser configuration"
	@echo "  clean              - Remove build artifacts"
	@echo "  ensure             - Install development dependencies"
	@echo "  help               - Show this help message"
