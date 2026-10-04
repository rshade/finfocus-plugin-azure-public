# Proposal

## Why

finfocus-spec v0.7.5 fixes the two SDK problems this plugin reported (issue #100):

- The conformance suite sent a hard-coded AWS resource, so an Azure plugin
  failed at Basic level whatever it returned (finfocus-spec#625).
- The SDK rewrapped every handler error as `Internal`, so `GetBudgets` and
  `DismissRecommendation`, which the plugin does not serve, reached hosts as a
  server fault (finfocus-spec#626).

Adopting v0.7.5 makes the plugin pass conformance at all three levels and report
unserved RPCs as `Unimplemented`. Both are worth locking in with tests. This
ships as plugin v0.1.1.

## What Changes

- `go.mod` moves to finfocus-spec v0.7.5. `GetPluginInfo` and the generated
  manifests report spec version `v0.7.5`.
- Served through the SDK, `GetBudgets` and `DismissRecommendation` return
  `Unimplemented` instead of `Internal`.
- A unit test runs the SDK conformance suite at Basic, Standard, and Advanced
  with an Azure virtual machine sample resource and a fixture price server, so
  CI fails on any conformance regression.

## Capabilities

### New Capabilities

- `conformance`: the plugin passes the finfocus-spec SDK conformance suite,
  served through the SDK, with an Azure sample resource.

### Modified Capabilities

- `plugin-info`: add a requirement that RPCs the plugin does not serve report
  `Unimplemented` over gRPC.

## Impact

- `go.mod`, `go.sum`, `manifest.json`, `manifest.yaml`.
- New tests in `internal/pricing`; no pricing logic changes.
- Docs: CLAUDE.md, CONTEXT.md, TASKS.md, and ROADMAP.md name v0.7.5 and drop
  the closed upstream follow-ups. README.md names v0.7.4 only as the release
  that added the actual-cost descriptor, so it needs no edit.
- Also in this change: CLAUDE.md now says core v0.4.2 sends the actual-cost
  `resource` (finfocus#1680), and records why Release Please dropped the #98
  fix from the release notes. `.markdownlint-cli2.jsonc` skips OpenSpec delta
  specs, which start with `## ADDED Requirements` by format.
- The plugin's own version (`pluginVersion`) is not bumped by Release Please,
  so v0.1.1 still reports 0.1.0 from `GetPluginInfo`. That predates this
  change and is tracked separately.
