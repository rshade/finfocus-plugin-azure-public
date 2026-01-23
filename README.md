# finfocus-plugin-azure-public

A Live/Runtime gRPC plugin for FinFocus that estimates Azure infrastructure costs by querying the Azure Retail Prices API.

## Purpose
This plugin enables FinFocus to provide accurate, on-demand pricing for Azure resources without requiring Azure credentials. It operates by fetching public pricing data from the Azure Retail Prices API and caching it for performance.

## Getting Started

### Prerequisites
- Go 1.25.5 or higher
- Internet connection (to fetch pricing data)

### Installation
1. Clone the repository:
   ```bash
   git clone https://github.com/rshade/finfocus-plugin-azure-public.git
   cd finfocus-plugin-azure-public
   ```
2. Install tools and build the plugin:
   ```bash
   make ensure
   make build
   ```

### Usage
Run the binary directly. It will start a gRPC server on a random port and print the port number to stdout.
```bash
./finfocus-plugin-azure-public
# Output: PORT=12345
```

## Available Commands

| Command | Description |
|---------|-------------|
| `make build` | Compile binary with version info |
| `make test` | Run unit tests with race detection |
| `make lint` | Run code quality checks |
| `make clean` | Remove build artifacts |
| `make ensure` | Install development dependencies |
| `make help` | Show available targets |

## Development
See [CLAUDE.md](CLAUDE.md) for development commands and guidelines.