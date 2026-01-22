# finfocus-plugin-azure-public Project Context

This document defines the technical guardrails and architectural scope of the `finfocus-plugin-azure-public` project.

## Core Architectural Identity
**Live/Runtime gRPC Plugin for Azure Retail Pricing**

`finfocus-plugin-azure-public` is a specialized provider for the FinFocus ecosystem. Unlike its air-gapped AWS counterpart, this plugin operates in a **Live/Runtime** mode, fetching pricing data on-demand from the public Azure Retail Prices API.

## Technical Boundaries ("Hard No's")

1.  **No Authenticated Azure APIs**: This plugin MUST NOT require an Azure Subscription, Tenant ID, Client Secret, or `az login`. It strictly consumes the *unauthenticated* `https://prices.azure.com/api/retail/prices` endpoint.
2.  **No Persistent Storage**: The plugin is stateless. While it employs an in-memory TTL cache for performance, it MUST NOT write to a local database (SQLite, BoltDB) or filesystem for long-term storage.
3.  **No Infrastructure Mutation**: The plugin is read-only. It calculates costs based on `ResourceDescriptor` inputs; it never validates if resources actually exist in Azure.
4.  **No Bulk Data Embedding**: We do NOT embed the Azure pricing catalog (which is massive). We fetch data dynamically based on the requested resources.

## Data Source of Truth
*   **Financial Data**: [Azure Retail Prices API](https://learn.microsoft.com/en-us/rest/api/cost-management/retail-prices/azure-retail-prices).
*   **Filtering**: We primarily filter for `type eq 'Consumption'` to target pay-as-you-go rates, unless specific reservation logic is requested later.

## Interaction Model
*   **Protocol**: gRPC (implementing `finfocus.v1.CostSourceService`).
*   **Lifecycle**: Orchestrated as a subprocess by FinFocus Core.
*   **Discovery**: Announces its listening port via `stdout` (format: `PORT=XXXXX`).
*   **Telemetry**: Structured JSON logging (via `zerolog`) sent exclusively to `stderr` to avoid corrupting the port discovery channel.
*   **Network Strategy**: "Fetch & Cache".
    *   **Fetch**: HTTP GET with OData filters.
    *   **Cache**: In-memory map with TTL (e.g., 24 hours) to deduplicate lookups for identical SKUs during a single run.
    *   **Resiliency**: Robust retry logic (exponential backoff) for HTTP 429/503 responses.

## Key Technologies
*   **Language**: Go 1.25+
*   **Transport**: `github.com/hashicorp/go-retryablehttp`
*   **Protocol**: `google.golang.org/grpc`
*   **Spec**: `github.com/rshade/finfocus-spec` (v0.5.4+)
