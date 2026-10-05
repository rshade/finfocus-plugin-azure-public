#!/usr/bin/env bash
# Every baseline requirement in openspec/specs/ names the Go tests that prove
# it on a `Tests:` line of backticked `TestName` references. This fails when a
# requirement has no such line, or when a named test does not run and pass in
# the default (non integration) build: a missing, skipped, or integration-only
# test is a failure, and so is a package that does not build. A spec whose
# Purpose is empty or still the archive placeholder (TBD) fails too.
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

names_file="$(mktemp)"
results_file="$(mktemp)"
trap 'rm -f "$names_file" "$results_file"' EXIT

python3 - "$names_file" <<'PY'
import pathlib
import re
import sys

failures = []
names = set()
for spec in sorted(pathlib.Path("openspec/specs").glob("*/spec.md")):
    text = spec.read_text()
    # openspec archive writes a placeholder Purpose for a new capability.
    purpose = re.search(r"^## Purpose\s*\n(.*?)(?=^## |\Z)", text, flags=re.M | re.S)
    if not purpose or not purpose.group(1).strip() or re.search(r"\bTBD\b", purpose.group(1)):
        failures.append(f"{spec}: Purpose is missing or still TBD")
    blocks = re.split(r"^### Requirement: ", text, flags=re.M)[1:]
    for block in blocks:
        title = block.splitlines()[0].strip()
        # Only the Tests: paragraph counts; a test named in prose proves nothing.
        line = re.search(r"^Tests:(.*(?:\n(?!\s*\n|#).*)*)", block, flags=re.M)
        tests = set(re.findall(r"`(Test[A-Za-z0-9_]+)`", line.group(1))) if line else set()
        if not tests:
            failures.append(f"{spec}: requirement '{title}' has no Tests: line naming a test")
        names |= tests

if failures:
    print("\n".join(failures), file=sys.stderr)
    sys.exit(1)
if not names:
    print("no requirements found under openspec/specs", file=sys.stderr)
    sys.exit(1)
pathlib.Path(sys.argv[1]).write_text("\n".join(sorted(names)) + "\n")
PY

pattern="^($(paste -sd'|' "$names_file"))\$"
go test -count=1 -json -run "$pattern" ./... >"$results_file" || true

python3 - "$names_file" "$results_file" <<'PY'
import json
import sys

wanted = set(open(sys.argv[1]).read().split())
outcome = {}
output = {}
build_output = []
failed_packages = []
for line in open(sys.argv[2]):
    try:
        event = json.loads(line)
    except json.JSONDecodeError:
        continue
    test = event.get("Test", "")
    action = event.get("Action")
    # go test -json reports compile errors as build-output events, and a build
    # failure, vet failure, panic, or timeout as a package-level fail.
    if action == "build-output":
        build_output.append(event.get("Output", ""))
        continue
    if action == "fail" and not test:
        failed_packages.append(event.get("Package", "?"))
        continue
    top = test.split("/", 1)[0]
    if top in wanted and action == "output":
        output.setdefault(top, []).append(event.get("Output", ""))
    if top not in wanted or action not in ("pass", "fail", "skip"):
        continue
    if test != top:
        # A parent passes even when every subtest skips, so a skipped
        # subtest means the requirement is not proven.
        if action != "pass" and outcome.get(top) != "fail":
            outcome[top] = f"{action} ({test})"
    elif outcome.get(top, "pass") == "pass" or action == "fail":
        outcome[top] = action

bad = sorted(
    f"{name}: {outcome.get(name, 'did not run (missing, integration-only, or its package failed)')}"
    for name in wanted
    if outcome.get(name) != "pass"
)
if build_output:
    print("build output:\n" + "".join(build_output), file=sys.stderr)
if failed_packages:
    print("failed packages: " + ", ".join(sorted(set(failed_packages))), file=sys.stderr)
for name in sorted(wanted):
    if outcome.get(name, "pass") != "pass" and output.get(name):
        # The tail of the test's own output says why; -json hides it otherwise.
        print(f"--- output of {name} ---\n" + "".join(output[name][-30:]), file=sys.stderr)
if bad or failed_packages:
    print("spec tests that do not pass:\n  " + "\n  ".join(bad), file=sys.stderr)
    sys.exit(1)
print(f"{len(wanted)} spec tests pass")
PY
