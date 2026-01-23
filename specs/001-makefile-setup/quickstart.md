# Quickstart: Using the Makefile

## Prerequisites
- Go 1.25.5+
- Git
- Make

## Commands

### Setup
Install development tools:
```bash
make ensure
```

### Build
Build the plugin binary (outputs to project root):
```bash
make build
```
Verify version:
```bash
./finfocus-plugin-azure-public --version
```

### Test
Run unit tests with race detection:
```bash
make test
```

### Lint
Check code quality:
```bash
make lint
```

### Clean
Remove build artifacts:
```bash
make clean
```

### Help
View all targets:
```bash
make help
```
