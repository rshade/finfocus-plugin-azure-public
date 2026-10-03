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
Record the starting commit for review before Spec Kit changes the branch name.

| Issue scope | Route |
| --- | --- |
| `spec-first`, or a feature requiring design and acceptance criteria | Spec Kit, then implementation |
| Focused `bug`, `enhancement` to an existing estimator, `component/testing`, `component/build`, or documentation work | Direct implementation with tests written first |
| Umbrella issue or unresolved `finfocus-spec` prerequisite | Report the blocker; offer a bounded child issue |

Read the body rather than routing solely by label. If a purported small fix
requires a new resource type, RPC behavior, or cache key dimension, reassess
its scope and whether an existing specification covers it.

### Spec Kit route

Look for an existing feature under `specs/` and resume its branch and artifacts
where appropriate. For a new feature, run specification creation **inside the
worktree**. The command creates a numbered feature branch; use the branch and
paths it returns for all later work, including the PR.

1. [speckit.specify](speckit.specify.md): create `spec.md`.
2. [speckit.clarify](speckit.clarify.md): resolve material ambiguity if needed.
3. [speckit.plan](speckit.plan.md): create `plan.md` and supporting design.
4. [speckit.tasks](speckit.tasks.md): create `tasks.md`.
5. [speckit.analyze](speckit.analyze.md): reconcile requirements, design, and
   tasks; address valid findings before implementation and rerun after changes.
6. [speckit.implement](speckit.implement.md): implement and update task status.

Let `.specify/scripts/bash/create-new-feature.sh` discover the next number;
do not pass `--number`. Check active feature branches and claims for numbering
collisions. Coordinate concurrent specification creation instead of committing
a specification on `main` to reserve a number.

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
```

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
commit with a Conventional Commit message. Include `Closes #N` and the spec
directory when relevant; use a breaking-change marker when warranted. Validate
the message with commitlint before committing:

```bash
printf '%s\n' "$COMMIT_MSG" | npx commitlint
git add <named files>
git commit -F "$COMMIT_MSG_FILE"
```

If a pre-commit hook or commitlint fails, fix the cause and create a new
commit; do not bypass hooks.

The PR body should describe the problem, resulting behavior, linked issue,
spec if any, and validation results (including whether the integration suite
ran). Store the body in a temporary file outside the worktree and pass it with
`--body-file`. The PR title must also pass commitlint.

```bash
BRANCH="$(git branch --show-current)"
gh pr list --repo "$REPO" --head "$BRANCH" --state open
git push -u origin "$BRANCH"
gh pr create --repo "$REPO" --base main --head "$BRANCH" \
  --title "<conventional commit subject>" --body-file "$PR_BODY_FILE"
```

Set `PR_BODY_FILE` to the prepared file and replace the title before execution.
Create the PR only if the check above found none. Leave the issue open for the
closing reference to resolve on merge. Stop at the PR; merging requires the
user's instruction.

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

Report the issue and selection reason, route and spec directory if any,
validation and review results, branch and worktree, PR URL, and whether the
claim is released or retained. Mention selection outside `roadmap/current`
explicitly. Do not pick another issue.
