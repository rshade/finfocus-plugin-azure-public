# Spec Kit history (frozen)

The directories here, `001-go-module-init` to `023-focus-13-record`, are the
Spec Kit features that built this plugin up to v0.1.0. They are read-only.
Do not edit them and do not add new numbered directories.

New work uses OpenSpec:

- Current behavior lives in [`openspec/specs/`](../openspec/specs/), one
  directory per capability. Each requirement names the Go tests that prove it,
  and `scripts/check-spec-tests.sh` checks that those tests pass.
- A behavior change is an OpenSpec change under `openspec/changes/`. It is
  proposed, applied, verified, and archived. Archiving folds it into
  `openspec/specs/`.
- `/pick-issue` decides whether an issue needs a change or a direct fix.

These directories describe the design at the time each feature was built.
Later features and fixes changed some of that behavior, so the code, the tests,
and `openspec/specs/` win over anything written here.

The project rules are still in
[`.specify/memory/constitution.md`](../.specify/memory/constitution.md).
