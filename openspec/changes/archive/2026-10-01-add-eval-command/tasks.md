# Tasks

## 1. Metrics library (`eval`)

- [x] 1.1 Add the `eval` package with a package doc, a labelled-result
  constructor (class, pick, `decision.Probability` confidence, correct
  option) and a typed error for an invalid class, an invalid confidence, a
  dominant result without a correct option and a contested result with one.
  Verify: table tests for each error case, and a safe/unsafe table for a
  correct pick, a wrong pick and a contested pick at 0.99.
- [x] 1.2 Add `Compute` over a non-empty result set, returning accuracy,
  contested AUC (ties count one half) and Brier score. Undefined metrics are
  absent, never zero, and an empty set is a typed error. Verify: tests for
  perfect separation (AUC 1), reversed separation (AUC 0), all ties
  (AUC 0.5), contested only (accuracy and AUC absent), and a single wrong
  pick at 0.9 (Brier 0.81, accuracy 0).
- [x] 1.3 Add per-threshold rows (the 0.05 to 0.95 grid plus the run's
  thresholds, ascending, without duplicates) and the outcome counts under the
  run's thresholds, including decided-but-unsafe. Verify: tests for a 0.92
  confident threshold appearing between 0.9 and 0.95, a row that selects
  nothing (precision absent), no safe results (recall absent), and a
  contested pick at 0.95 counted as decided and unsafe.
- [x] 1.4 Add `eval` to the list in `internal/cli/deps_test.go` and add an
  `ExampleCompute`. Verify: `go test ./eval/... ./internal/cli -run Deps`
  passes and `go doc ./eval` shows the example.

## 2. Response cache (`jevclient`)

- [x] 2.1 Add `WithResponseCache(dir)`. It installs an `http.Client` whose
  transport serves and fills the cache, keyed on the SHA-256 of method,
  scheme, host, path and body, and whose `CheckRedirect` returns
  `http.ErrUseLastResponse`. Only 200s are stored, with a temporary-name
  write and rename, in a 0700 directory with 0600 files. The request body
  is restored after reading. Verify: `httptest` tests for a hit sending one
  request, a 500 followed by success caching only the success, a truncated
  entry being refetched and replaced, the token absent from every cache
  file, and file and directory modes.
- [x] 2.2 Prove the other client guarantees hold with the cache on. Verify:
  tests that a 302 is still not followed, a 429 is retried and not cached,
  and a client without the option writes no file (checked in a
  `t.TempDir()` working directory).
- [x] 2.3 Document the cache in the `jevclient` package doc: opt-in, what
  the key covers, that it is never invalidated, and that deleting the
  directory resets it. Verify: `go doc ./jevclient` shows the section and
  `golangci-lint run ./jevclient/...` is clean.

## 3. Input files and the `eval` command

- [x] 3.1 Change `Env.NewClient` to take `...jevclient.Option` and update
  `ask`, `score` and the tests. Verify: `go test ./internal/cli/...` passes
  with no golden change.
- [x] 3.2 Add `internal/cli/evalinput.go`. It parses the decision set and
  truth file with unknown entry fields disallowed, checks ids (empty,
  repeated, mismatched), classes, option names and correct options, and
  builds each question through `decision.Spec.Question()`, with the state
  as `title`, `context`, `constraints` and raw `options`. Errors are
  validation errors named `decisions[<id>].<field>` or `truth[<id>].<field>`.
  Verify: table tests for each failure in the
  `eval-command` spec. Also verify that the bundled
  `testdata/decisions.json` and `decisions_truth.json` load into 40
  questions (20 dominant, 20 contested), and that `pros` reaches the state.
- [x] 3.3 Add the `eval` command with `--decisions`, `--truth`,
  `--instructions`, `--floor`, `--confident` and `--cache-dir`. It runs
  sequentially, fails fast with the decision id in the error, and prints
  `EvalOutput`, or `EvalDryRunOutput` under `--dry-run`. Verify: `httptest`
  tests for a full report with exit 0, a missing answer on the second
  decision (no report, failure code, one request sent after the first), a
  rerun with `--cache-dir` sending no request, no file written without
  `--cache-dir`, custom instructions reaching every request, invalid input
  sending zero requests with exit 2, and a dry run with no key sending
  nothing.
- [x] 3.4 Register `eval` under the root command and mention it in the root
  help text. Verify: `go run ./cmd/jev-decide eval --help` lists every flag.

## 4. Output contract and schema version 2

- [x] 4.1 Add a `since` version to each golden case (1 for the existing
  ones, 2 for `eval.report` and `eval.dryrun`) and make the frozen-files
  check start from it. Verify: the golden tests still pass at version 1
  before the bump.
- [x] 4.2 Bump `SchemaVersion` to 2 and write the v2 golden files with
  `go test ./internal/cli -run Golden -update`. Verify: every `.v1.json` is
  byte-for-byte unchanged (`git diff --stat` shows only new files under
  `testdata/golden/`), the only difference between each v1 and v2 `ask` and
  `score` file is `schema_version`, and `schema.v2.json` lists `eval`.
- [x] 4.3 Document `eval` in `docs/jev-decide-cli.md`: flags, input format,
  report fields, exit code 0 for a report, the cache, and that the bundled
  set is a smoke test. Verify: `go test ./internal/cli -run Docs` passes and
  `npx markdownlint-cli2 "docs/*.md"` is clean.

## 5. Docs and integration

- [x] 5.1 Update `README.md` usage, `CONTEXT.md` (`eval` shipped, not
  planned), `CLAUDE.md` (the `eval` package and the cache option) and
  `ROADMAP.md` #5. Verify: `npx markdownlint-cli2 "**/*.md"` is clean.
- [x] 5.2 Run the full safe gate set. Verify: `go build ./...`, `gofmt -l .`
  (empty), `go vet ./...`,
  `go test ./decision/... ./jevclient/... ./eval/... ./internal/... ./cmd/...`,
  `golangci-lint run ./...` and
  `mise exec -- openspec validate add-eval-command --strict` all pass.
- [x] 5.3 With the user's approval only, because it spends money, run `eval`
  once against the live API on the bundled set with a scratch `--cache-dir`,
  then a second time. Verify: the first run's AUC is near the spike's 0.99,
  and the second run sends no request.
