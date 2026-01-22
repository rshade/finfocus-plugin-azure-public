# Gemini Context for finfocus-plugin-azure-public

This file provides context for Gemini when working with this repository.

## Project Overview

**finfocus-plugin-azure-public** is a Live/Runtime gRPC plugin for FinFocus that estimates Azure infrastructure costs by querying the Azure Retail Prices API.

- **Purpose:** Provide accurate, on-demand pricing without Azure credentials.
- **Architecture:**
    - **Protocol:** gRPC (`finfocus.v1.CostSourceService`).
    - **Data Source:** Azure Retail Prices API (Unauthenticated).
    - **Strategy:** Fetch & Cache (In-memory TTL cache).
    - **Resiliency:** Retries with exponential backoff.

## Active Technologies
- Go 1.25.5
- `github.com/hashicorp/go-retryablehttp` (HTTP Transport)
- `github.com/rshade/finfocus-spec` (Plugin SDK)
- `github.com/rs/zerolog` (Logging)
- N/A (Stateless) (001-go-module-init)

## Development Guidelines

### "Hard No's"
- **No Auth**: Do not implement Azure AD authentication.
- **No DB**: Do not add persistent storage (SQLite, etc.).
- **No Terraform**: Do not use Terraform providers directly.

### Testing
- **Unit Tests**: Mock the HTTP client to test parsing logic without hitting the live API.
- **Integration Tests**: Minimal live tests against the public API (rate limited).

### Logging
- **Stdout**: Reserved for `PORT=XXXX` only.
- **Stderr**: All other logs.

## Recent Changes
- 001-go-module-init: Added Go 1.25.5
