# Tasks

## 1. Amend the boundaries

- [x] 1.1 Update `CONTEXT.md` (endpoint rule, credentials rule, interaction
  model) and `CLAUDE.md` (architecture, commands) to name the clef backend.
  Verify: `npx markdownlint-cli2 "**/*.md"` passes and no "TypeSafe only"
  wording remains.

## 2. Extract the shared client plumbing

- [x] 2.1 Move base URL validation, the no-redirect client, the redacting
  logger and the response cache into `internal/clientkit`, parameterized by
  environment variable names; make `jevclient` use it. Verify: `go test
  ./jevclient/...` passes with no test edited, and `go test
  ./internal/cli -run Deps` passes.

## 3. Build `clefclient`

- [x] 3.1 Write failing `httptest` tests for credential and account-ID
  validation, endpoint and base URL rules, redirect refusal and `model` being
  sent. Then implement `NewClient`. Verify: the new tests pass with
  `go test ./clefclient/...`.
- [x] 3.2 Write failing tests for the envelope unwrap (`success: false`,
  missing answer, out-of-range probability) and the 429-only retry policy,
  then implement the `RoundTripper`. Verify: those tests pass and a timeout
  test shows exactly one request.
- [x] 3.3 Provoke one live bad-token call and record the real status and
  Cloudflare code, then write `Classify`, the `clef.*` codes and
  `docs/clef-errors.md` with a table row and JSON example per code. Verify:
  a docs test fails when a code is missing, and `go test ./clefclient/...`
  passes with token redaction covered.
- [x] 3.4 Add `clefclient/live_test.go` that skips without the Cloudflare
  variables. Verify: `go test ./clefclient` skips cleanly with them unset and
  passes with them set.
- [x] 3.5 Add `clefclient` to the dependency rule. Verify:
  `internal/cli/deps_test.go` lists it and passes.
- [x] 3.6 Export `NewTransport` and `BaseURL`, record a real response as
  `clefclient/testdata/response.json`, and document reuse from another client.
  Verify: `go test ./clefclient` covers a plain `net/http` client, refused
  paths and the recorded response, and a throwaway module outside the repo
  imports the package with `replace` and builds.

## 4. Select the backend in the CLI

- [x] 4.0 Add `decision.WithClassifier` (default `jevclient.Classify`),
  used for failed calls and invalid answers in `Choose` and `Rate`. Verify:
  decision tests show a custom classifier sees both failure kinds, and the
  existing decision tests pass unedited.
- [x] 4.1 Add `--backend` to `ask`, `score` and `eval`, `Env.NewClefClient`,
  the per-backend threshold table and classification dispatch; validate the
  name before any request and on `--dry-run`. Verify: CLI tests with fake
  clients cover default, clef, unknown value, missing clef credentials with
  Jev credentials set (no fallback), and threshold override.
- [x] 4.2 Add `backend` to every output, bump `SchemaVersion` to 4 and add
  the `*.v4.json` golden files with `go test ./internal/cli -run Golden
  -update`, leaving the v3 files untouched. Verify: `git diff --stat` shows
  v3 goldens unchanged and the golden tests pass.
- [x] 4.3 Update `docs/jev-decide-cli.md` (flag, exit codes, schema 4) and
  `README.md` (backends, environment variables). Verify: markdownlint passes
  and the docs test for exit codes passes.

## 5. Evidence and CI

- [x] 5.1 Keep `clef_probe_test.go` as the recorded probe and rerun
  `go test -tags probe -run TestClefDecisions -v -count=1`; update the numbers
  in `docs/clef-2026-10-06.md` if they moved. Verify: the table matches the
  run.
- [x] 5.2 Add `.github/workflows/live.yml` with `workflow_dispatch` only,
  least-privilege permissions and the three secrets. Verify: `actionlint` or
  a dry parse passes, the workflow has no `pull_request` trigger, and the
  secret names are confirmed in repository settings.

## 6. Final verification

- [x] 6.1 Run `go build ./...`, `go test ./...`, `golangci-lint run ./...`,
  `npx markdownlint-cli2 "**/*.md"` and `mise exec -- openspec validate
  --all --strict`. Verify: all pass, and `ROADMAP.md` and `TASKS.md` mention
  the change.
