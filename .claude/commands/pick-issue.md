---
title: "Pick a Roadmap Issue"
description: Choose and claim one finfocus-plugin-azure-public roadmap issue, implement and verify it, open a pull request, and release the claim.
layout: "docs"
---

Choose exactly one issue in `rshade/finfocus-plugin-azure-public`, take it
through implementation and verification, and open a pull request. Stop after
that issue. Adapted from FinFocus's `pick-issue` workflow for this plugin's
commands and repository rules.

An invocation to work an issue includes the claim and release comments below.
A request only to recommend an issue, or to edit this workflow, does not.
Respect the user's requested scope and existing authorization throughout.

## Phase 0 — Preflight

Read [CLAUDE.md](../../CLAUDE.md), [CONTEXT.md](../../CONTEXT.md), and the
[constitution](../../.specify/memory/constitution.md). Check for additional
instructions in the directories you will touch.

```bash
ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"
REPO=rshade/finfocus-plugin-azure-public
gh auth status
git remote -v
git status --short
git worktree list
git fetch origin
```

Confirm `origin` points to this plugin before using `origin/main`. Work in an
isolated worktree; preserve existing tracked and untracked changes in the
original checkout. Never stash someone else's work or stage files with
`git add .` or `git add -A`.

## Phase 1 — Choose and claim

### Inspect current work and candidates

```bash
gh label list --repo "$REPO" --limit 200 --json name,description
gh issue list --repo "$REPO" --state open --label roadmap/current --limit 100 \
  --json number,title,body,labels,assignees
gh pr list --repo "$REPO" --state open --limit 100 \
  --json number,title,headRefName,body
```

If a result reaches its limit, retrieve the remaining pages before concluding
that work is unclaimed. Check issues carrying `processing:roadmap` if that
label exists, and inspect their claim comments as described below.

The plugin uses `roadmap/current`, `roadmap/next`, `roadmap/future`,
`priority/critical`, `priority/high`, `priority/medium`, `priority/low`,
`effort/small`, `effort/medium`, `effort/large`, and `spec-first`. Lanes are
`component/transport`, `component/http-client`, `component/models`,
`component/cache`, `component/estimation`, `component/build`, and
`component/testing`. Discover other labels at runtime.

Use component labels as a first overlap signal, then inspect issue bodies and
relevant files. Most pricing work converges on `internal/pricing`
(`calculator.go`, `mapper.go`), `internal/azureclient`, `go.mod`, `go.sum`,
and shared `testdata/` fixtures such as `plan-expected.json`. A different or
absent component label does not prove independence.

Exclude claimed issues, work already covered by an open PR, blocked issues,
and umbrella issues whose children should be implemented separately. Use
[ROADMAP.md](../../ROADMAP.md) for context, but confirm issue state on GitHub.
Protocol and SDK changes belong in `finfocus-spec`; do not define new RPC
methods or message fields here to bypass a pending spec release. Never add Azure
authentication, persistent storage, or an embedded pricing catalog to satisfy
an issue (see [CONTEXT.md](../../CONTEXT.md)).

If the user provided an issue number, inspect that issue directly, including
its state, labels, comments, dependencies, and linked PRs. An explicit number
can select an issue outside `roadmap/current`, but cannot override another
worker's claim. Report any overlap before implementation.

Otherwise present a table of eligible issues: number, type, component, priority,
effort, and title. Order by `priority/*` (critical first), then `spec-first`,
`bug`, and maintenance work. Mark missing metadata as unknown. Ask the user to
choose unless they already asked you to choose autonomously.

If no eligible `roadmap/current` issues remain, report whether the queue is
empty, blocked, or claimed. Ask before widening to `roadmap/next` unless the
user already authorized it. Do not change roadmap labels to promote work.

### Claim and verify ownership

`processing:roadmap` is an advisory coordination label. GitHub label mutations
are not atomic locks. If the label is absent from the repository, create it
when actually claiming an issue:

```bash
gh label create processing:roadmap --repo "$REPO" --color D93F0B \
  --description "Issue claimed by a pick-issue worker"
```

If creation reports that the label already exists, re-read it and proceed;
other failures must be resolved before claiming.

Use a unique token for this invocation and keep it available across shell
calls. Set `N` to the chosen issue number. Before posting, read all issue
comments and labels. An unreleased claim is active regardless of age; a label
without a claim is ambiguous. Report either case rather than stealing it.

```bash
CLAIM="claim: $(hostname)/$$-$(date -u +%s)-$(openssl rand -hex 8)"
gh issue comment "$N" --repo "$REPO" --body "$CLAIM"
gh issue edit "$N" --repo "$REPO" --add-label processing:roadmap
gh api --paginate "repos/$REPO/issues/$N/comments" \
  --jq '.[] | {id, created_at, body}'
```

A claim comment is a body starting with `claim:`. A release comment has the
exact body `release: <full claim body>`. Among claims without matching releases,
the earliest comment wins; use the numeric comment ID to break timestamp ties.
Re-read comments and labels before implementation. Also recheck overlapping
claims on other issues; this protocol does not atomically reserve packages.

If another worker wins, post a release for your own token and return to the
chooser. **Never remove the winner's label.** If any claim operation fails,
inspect the resulting state before retrying; do not assume ownership or post
another claim token. Do not expire an old claim automatically. A stale claim
requires an explicit handoff or user resolution.

### Reconcile the issue with the repository

Read the full issue and comments, locate the cited code with `rg`, and check
relevant commits and tests before routing it. Verify referenced dependency
issues and PRs, including `finfocus-spec` releases. Old file paths, line numbers,
meter names, and prices are evidence to investigate, not implementation
instructions. The live Azure Retail Prices API is the source of truth for
meter, product, and unit names; record the query date for any price you cite.

If the work is already complete, report the implementing commits and tests.
Close the issue only when authorized, then release your claim and stop.

## Phase 2 — Create a worktree and choose the route

```bash
WORKTREE="$(dirname "$ROOT")/finfocus-plugin-azure-public-$N"
git worktree add "$WORKTREE" -b "issue-$N" origin/main
cd "$WORKTREE"
```

If the branch or path exists, inspect it and resume only when it belongs to
this issue; otherwise choose a unique path and branch. Never overwrite it.
Record the starting commit for review.

| Issue scope | Route |
| --- | --- |
| Changes plugin behavior: a requirement in `openspec/specs/` is added, changed, or removed (an RPC, a resource type, a meter or unit, the cost arithmetic, an error code, a cache key dimension) | OpenSpec change, then implementation, verify, archive |
| Contained fix that changes no requirement: a `bug` where the code breaks an existing requirement, `component/testing`, `component/build`, or documentation work | Direct implementation with tests written first |
| Umbrella issue or unresolved `finfocus-spec` prerequisite | Report the blocker; offer a bounded child issue |

Read the body rather than routing solely by label. If a purported small fix
requires a new resource type, RPC behavior, or cache key dimension, reassess
its scope and whether an existing requirement in `openspec/specs/` covers it.
A `spec-first` issue waits on finfocus-spec. Once that release ships, route it
by the table: usually OpenSpec, because adopting a new spec field changes a
requirement.
Do not send a one-file fix through OpenSpec: propose, apply, verify, and
archive for a one-line change is ceremony without protection.

### OpenSpec route

The CLI is pinned in `mise.toml`; run it as `mise exec -- openspec ...`
inside the worktree. Check `mise exec -- openspec list --json` for an open
change that already covers the issue and resume it rather than starting a
second one. Use the skills `openspec init` generated under `.claude/skills/`:

1. `openspec-propose` (`/opsx:propose`): write `proposal.md`, `design.md`
   when the change needs one, the delta specs under
   `specs/<capability>/spec.md`, and `tasks.md`. Name the change with a
   kebab-case slug that includes the issue number, such as
   `add-redis-pricing-42`.
2. `openspec-apply-change` (`/opsx:apply`): work the tasks in order. Tick a
   task in `tasks.md` when its `Verify:` command passes, one task at a time,
   never in bulk before archive.
3. `openspec-verify-change` (`/opsx:verify`): compare the code with the
   artifacts and fix every real finding. This step is required, and it does
   not replace the Phase 3 review of the diff.
4. `openspec-archive-change` (`/opsx:archive`): fold the delta specs into
   `openspec/specs/` and move the change to `openspec/changes/archive/`.
   Archive lands in the same commit as the code, after verify passes. An
   archive that creates a capability writes `Purpose` as `TBD - created by
   archiving change ...`; replace it with one or two sentences.
   `make spec-check` fails while a `TBD` Purpose remains.

**Tests-first checkpoint.** `tasks.md` starts with the tests. Before you touch
the implementation or a dependency, run the first task group's `Verify:`
commands and record how each test fails or fails to build; that output is the
break check. A spike that measures the change first (for example, bumping a
dependency to see what breaks) belongs in a scratch worktree. Discard it and
start the change from `origin/main`, so the tests still come first.

Every task in `tasks.md` carries a `Verify:` command that proves it (a
`go test -run` pattern, `make spec-check`, or a live query with its date) and
a break check: what you changed to watch the verify fail before it passed.

Every added or modified requirement keeps the baseline rule from
`openspec/config.yaml`: a `Tests:` line naming the Go tests that prove it.
Write those tests first. Check the change as you go:

```bash
mise exec -- openspec status --change "<slug>" --json
mise exec -- openspec validate "<slug>" --strict --no-interactive
```

Never edit `specs/001-*` to `specs/023-*`; they are frozen Spec Kit history.

### Implementation rules (both routes)

The constitution makes TDD non-negotiable: write the failing test first, then
the implementation. Use table-driven tests named
`Test<Function>_<Scenario>_<ExpectedOutcome>`, mock the HTTP client in unit
tests, and keep live API calls in `examples/` integration tests. Keep logs on
`stderr` via `zerolog` and `stdout` reserved for `PORT=`.

Update [CLAUDE.md](../../CLAUDE.md) and `docs/` when behavior, resource types,
or attribute aliases change. Do not edit `CHANGELOG.md`; Release Please owns
it. Do not edit `.release-please-manifest.json` or tag a release.

## Phase 3 — Verify and review

Run the repository's gates from the implementation worktree:

```bash
make build
make test
make lint
make spec-check
```

`make spec-check` runs `openspec validate --all --strict` and
`scripts/check-spec-tests.sh`, which fails when a requirement names no test or
a named test is missing, fails, or skips. Run it on both routes: a direct fix
can still break a test a requirement names.

`make test` already runs `go test -race ./...`. When the change touches
`internal/azureclient`, a resource quote, or a `testdata/` fixture, also run
the live integration suite:

```bash
go test -v -tags=integration -timeout=5m ./examples/...
```

Report a skipped integration run (`SKIP_INTEGRATION=true` or no network)
instead of claiming it passed. Run `make vulncheck` when `go.mod` changes.

`make lint` swallows markdownlint, vale, and actionlint failures with
`|| true`, so a green `make lint` says nothing about them. Run them directly
on what you changed and fix what they report:

```bash
markdownlint-cli2 <changed .md files>
vale <changed .md files>
actionlint .github/workflows/   # only if workflows changed
```

Review the implementation against the recorded base commit and issue
acceptance criteria. Use the available `code-review` skill and fix
substantiated findings. If `scout` is available, report its improvement
opportunities without expanding this issue's scope. Spec analysis does not
replace review of the implemented diff.

Report actual failures and unavailable tools.

## Phase 4 — Commit and open the pull request

Invoking this workflow is the explicit instruction to commit, push, and open
the PR for the chosen issue. Stage only the files you changed, by name, and
commit with a Conventional Commit message. Include `Closes #N` and the
archived OpenSpec change when there is one; use a breaking-change marker when
warranted. Validate the message with commitlint before committing:

```bash
printf '%s\n' "$COMMIT_MSG" | npx commitlint
git add <named files>
git commit -F "$COMMIT_MSG_FILE"
```

If a pre-commit hook or commitlint fails, fix the cause and create a new
commit; do not bypass hooks.

Write the PR message with `/pr-message`, giving it issue `N` instead of
letting it search by branch name. Without that command, write `PR_MESSAGE.md`
in the repository root in its format: a commitlint subject line, `## Summary`,
`## Test plan` (say whether the integration suite ran), `## Changes`, and a
last line `Closes #N`. Git ignores the file. Validate it:

```bash
npx --yes markdownlint-cli --disable MD041 -- PR_MESSAGE.md
cat PR_MESSAGE.md | npx commitlint
```

The PR description must contain `Closes #N`. A squash merge makes the
description the commit, so a closing keyword that is only in the branch
commits is dropped and the issue stays open; #100, #102, #103, and #104
stayed open that way. Release Please parses that description too, so keep
shell blocks out of it (see the Release Please section of CLAUDE.md).

```bash
BRANCH="$(git branch --show-current)"
gh pr list --repo "$REPO" --head "$BRANCH" --state open
git push -u origin "$BRANCH"
tail -n +3 PR_MESSAGE.md | gh pr create --repo "$REPO" --base main \
  --head "$BRANCH" --title "$(head -n 1 PR_MESSAGE.md)" --body-file -
gh pr view "$BRANCH" --repo "$REPO" \
  --json closingIssuesReferences --jq '[.closingIssuesReferences[].number]'
```

Create the PR only if `gh pr list` found none. The last command must list
`N`. If it does not, fix the description with `gh pr edit --body-file` and
check again before reporting the PR. Leave the issue open for the closing
reference to resolve on merge. Stop at the PR; merging requires the user's
instruction.

## Phase 5 — Release or retain the claim

After opening the PR, or explicitly abandoning the attempt, record the PR or
remaining work in an issue comment. Re-read ownership before cleanup. Remove
the label only if you still own the winning claim and no other unreleased
claim needs it, then post the exact release marker:

```bash
gh issue edit "$N" --repo "$REPO" --remove-label processing:roadmap
gh issue comment "$N" --repo "$REPO" --body "release: $CLAIM"
```

If another unreleased claim exists, leave the label and release only your own
token. A losing worker must never remove the label. Verify the final comments
and label state; report cleanup failures instead of claiming release succeeded.

While waiting for a user decision, retain the claim and say why. Keep any
worktree with uncommitted or unpushed work. Remove a completed worktree only
after confirming its work is preserved remotely and it is clean; run removal
from the original checkout and never force it.

## Phase 6 — Report and stop

Report the issue and selection reason, route and OpenSpec change if any,
validation and review results, branch and worktree, PR URL, the issues the PR
closes on merge, and whether the claim is released or retained. Mention
selection outside `roadmap/current` explicitly. Do not pick another issue.
