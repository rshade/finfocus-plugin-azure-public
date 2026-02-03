# Quickstart: CI Pipeline

**Feature**: 005-ci-pipeline
**Date**: 2026-02-03

## Overview

This feature adds GitHub Actions CI to automatically validate code quality on pull
requests and main branch commits.

## What Gets Created

1. **Workflow File**: `.github/workflows/test.yml`
   - Triggers on PR to main and push to main
   - Runs tests, lint, and build
   - Caches Go modules for faster execution

2. **README Badge**: CI status badge at top of README.md

## How It Works

```text
PR Created/Updated → CI Triggers → Test → Lint → Build → Status Reported
     or
Push to main → CI Triggers → Test → Lint → Build → Status Reported
```

## Workflow Steps

| Step           | Command                       | Purpose                        |
|----------------|-------------------------------|--------------------------------|
| 1. Checkout    | actions/checkout@v6           | Clone repository               |
| 2. Setup Go    | actions/setup-go@v6           | Install Go 1.25 with caching   |
| 3. Test        | `make test`                   | Run tests with race detector   |
| 4. Lint        | golangci-lint-action@v6       | Run code quality checks        |
| 5. Build       | `make build`                  | Verify binary compiles         |

## Expected Timing

- **First run (cold cache)**: ~5-8 minutes
- **Subsequent runs (cached)**: ~3-5 minutes
- **Maximum allowed**: 10 minutes

## Verification

After implementation:

1. **PR Validation**: Open a PR targeting main, verify CI runs
2. **Pass Scenario**: All checks should show green
3. **Fail Scenario**: Introduce a test failure, verify CI fails with clear error
4. **Badge**: Verify README shows CI status badge

## Troubleshooting

| Issue               | Cause                       | Solution                        |
|---------------------|-----------------------------|---------------------------------|
| Lint timeout        | Large codebase or cold cache| Makefile has 10m timeout        |
| Module fetch fails  | Network issue or private deps| Check go.sum, verify access    |
| Tests fail          | Code issue                  | Check test output in logs       |
| Workflow syntax err | Invalid YAML                | Use GitHub Actions validator    |
