# finfocus-plugin-azure-public Strategic Roadmap

This roadmap outlines the development of the Azure Retail Pricing plugin for FinFocus. It follows the **Spec-Driven Development (Speckit)** workflow.

## Mission Statement
To provide accurate, real-time Azure cost estimates for FinFocus by querying the Azure Retail Prices API, ensuring resilience and performance through intelligent caching and robust transport logic.

---

## Phase 1: Scaffold & Transport (v0.1.0)
**Goal:** Establish the plugin structure, build system, and basic gRPC connectivity.

- [ ] **Repo Setup**
    - [ ] Initialize Go module `github.com/rshade/finfocus-plugin-azure-public`.
    - [ ] Setup `Makefile` with `build`, `test`, `lint` targets.
    - [ ] Configure CI (GitHub Actions) for build and lint.
- [ ] **gRPC Server**
    - [ ] Implement `main.go` entrypoint with `finfocus-spec` SDK.
    - [ ] Implement `CostSourceService` stub.
    - [ ] Implement `zerolog` structured logging (stdout for PORT, stderr for logs).
    - [ ] Verify port announcement (`PORT=12345`).

## Phase 2: The Azure Client (v0.2.0)
**Goal:** Implement the HTTP client capable of querying the Azure Retail API reliably.

- [ ] **HTTP Transport**
    - [ ] Integrate `hashicorp/go-retryablehttp` with backoff configuration.
    - [ ] Implement `GetPrices(filter string)` function.
    - [ ] Handle pagination (`NextPageLink`).
    - [ ] Handle API errors (429, 503) gracefully.
- [ ] **Data Model**
    - [ ] Define Go structs for `PriceItem` and API responses.
    - [ ] Implement basic OData query builder (e.g., `armRegionName eq 'eastus'`).

## Phase 3: The Caching Layer (v0.3.0)
**Goal:** Prevent API throttling and improve performance for repetitive lookups.

- [ ] **In-Memory Cache**
    - [ ] Implement a thread-safe map for caching API responses.
    - [ ] Implement TTL (Time-To-Live) logic (default: 24h).
    - [ ] Add cache hit/miss logging.
- [ ] **Concurrency**
    - [ ] Ensure multiple concurrent gRPC requests can share cache entries.

## Phase 4: Field Mapping & Estimation (v0.4.0)
**Goal:** Connect the FinFocus generic `ResourceDescriptor` to Azure-specific queries.

- [ ] **Resource Mapping**
    - [ ] Implement mapping logic for Virtual Machines (SkuName, Region).
    - [ ] Implement mapping logic for Managed Disks (Size, Tier).
    - [ ] Implement mapping for "Consumption" type filtering.
- [ ] **Cost Calculation**
    - [ ] Calculate hourly/monthly costs from `unitPrice` and `retailPrice`.
    - [ ] Return `CostResult` protobuf messages.

---

## Future Vision
- [ ] **Reservation Support**: Add filtering for Reservation terms.
- [ ] **Currency Conversion**: Support non-USD currencies if needed.
- [ ] **Hybrid/Spot Pricing**: logic for spot instances if exposed by API.
