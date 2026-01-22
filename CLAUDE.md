# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Identity

`finfocus-plugin-azure-public` is a **Live/Runtime gRPC plugin** that provides real-time Azure cost estimates for the FinFocus ecosystem. Unlike air-gapped counterparts, this plugin operates by fetching pricing data on-demand from the public Azure Retail Prices API.

## Essential Commands

```bash
make build   # Build the plugin binary (outputs: finfocus-plugin-azure-public)
make test    # Run all tests
make lint    # Run golangci-lint (can take >5 minutes)
make clean   # Remove build artifacts
```

**Timeout Note**: `make lint` can run longer than 5 minutes; use extended timeouts when running locally.

## Core Architectural Constraints ("Hard No's")

1. **No Authenticated Azure APIs**: MUST NOT require Azure credentials (Subscription, Tenant ID, Client Secret, or `az login`). Only uses unauthenticated `https://prices.azure.com/api/retail/prices` endpoint.

2. **No Persistent Storage**: Stateless operation only. In-memory TTL cache is allowed, but MUST NOT use SQLite, BoltDB, or filesystem for long-term storage.

3. **No Infrastructure Mutation**: Read-only cost calculations based on `ResourceDescriptor` inputs. Never validates if resources actually exist in Azure.

4. **No Bulk Data Embedding**: Never embed the Azure pricing catalog. All data fetched dynamically based on requested resources.

## Plugin Architecture

### Protocol & Lifecycle
- **Protocol**: gRPC implementing `finfocus.v1.CostSourceService`
- **Lifecycle**: Launched as subprocess by FinFocus Core
- **Discovery**: Announces listening port via stdout (format: `PORT=XXXXX`)
- **Telemetry**: Structured JSON logging via `zerolog` sent exclusively to stderr to avoid corrupting port discovery

### Network Strategy: "Fetch & Cache"
- **Fetch**: HTTP GET with OData filters against Azure Retail Prices API
- **Cache**: In-memory map with TTL (24 hours default) to deduplicate identical SKU lookups
- **Resiliency**: Exponential backoff retry logic for HTTP 429/503 responses

### Logging Constraints
- **stdout**: Reserved for `PORT=XXXXX` announcement ONLY
- **stderr**: All other structured logs must go here

## Key Technologies

- **Language**: Go 1.25+
- **HTTP Transport**: `github.com/hashicorp/go-retryablehttp`
- **gRPC**: `google.golang.org/grpc`
- **Plugin SDK**: `github.com/rshade/finfocus-spec` (v0.5.4+)
- **Logging**: `github.com/rs/zerolog`

## Data Source

**Financial Data**: [Azure Retail Prices API](https://learn.microsoft.com/en-us/rest/api/cost-management/retail-prices/azure-retail-prices)

Primary filter: `type eq 'Consumption'` (pay-as-go rates)

## Development Approach

This project follows **Spec-Driven Development (Speckit)** workflow. See ROADMAP.md for phased implementation plan:
- Phase 1: Scaffold & Transport (v0.1.0)
- Phase 2: Azure Client (v0.2.0)
- Phase 3: Caching Layer (v0.3.0)
- Phase 4: Field Mapping & Estimation (v0.4.0)

## Testing Guidelines

**Unit Tests**: Mock the HTTP client to test parsing logic without hitting live API

**Integration Tests**: Minimal live tests against public API (be mindful of rate limits)

## Version Management

Development versions auto-calculated from latest git tag:
- Format: `MAJOR.MINOR.NEXT_PATCH-dev`
- Injected via LDFLAGS: `-X main.version=$(DEV_VERSION)`
