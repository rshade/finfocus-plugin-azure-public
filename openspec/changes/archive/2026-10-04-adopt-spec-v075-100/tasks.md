# Tasks

## 1. Tests first

- [x] 1.1 Add `TestSDKConformance_AzureSample_PassesEveryLevel`: run Basic, Standard, and Advanced conformance with the Azure sample resource against `pluginsdk.NewServerWithOptions(calc, nil, nil, PluginInfo())` and a fixture price server. Verify: `go test -count=1 -run '^TestSDKConformance_AzureSample_PassesEveryLevel$' ./internal/pricing/`. Break check: on v0.7.4 the suite has no sample-resource option, and the default AWS sample failed 7 Basic checks.
- [x] 1.2 Add `TestUnservedRPCs_OverGRPC_ReturnUnimplemented` for `GetBudgets` and `DismissRecommendation`. Verify: `go test -count=1 -run '^TestUnservedRPCs_OverGRPC_ReturnUnimplemented$' ./internal/pricing/`. Break check: on v0.7.4 both return `Internal`.

## 2. Adopt v0.7.5

- [x] 2.1 `go get github.com/rshade/finfocus-spec@v0.7.5` and `go mod tidy`. Verify: `go build ./...`. Break check: the two tests above fail or do not build before the bump.
- [x] 2.2 Regenerate the manifests. Verify: `go test -count=1 -run TestExpectedManifest ./internal/pricing/`. Break check: before regenerating, `TestExpectedManifest_CommittedFiles_MatchExpected` fails on `spec_version`.

## 3. Docs and gates

- [x] 3.1 Update the spec version and the conformance and status-code notes in CLAUDE.md, CONTEXT.md, TASKS.md, and ROADMAP.md (README.md needs none). Verify: `git grep -n 'v0\.7\.4' -- CLAUDE.md CONTEXT.md README.md TASKS.md` lists only lines that name the version that added a field.
- [x] 3.2 Run the gates. Verify: `make build`, `make test`, `make lint`, `make spec-check` all pass.
