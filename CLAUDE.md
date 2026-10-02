# CLAUDE.md

<!-- markdownlint-disable-next-line MD013 -->
This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`jev-decide`: a typed Go library and CLI for TypeSafe AI's Jev (System One)
model, for fast-path decision support. The module path is
`github.com/rshade/jev-decide`; the local directory is still named `gojev`, so
do not rename it. `gojev` is taken upstream (`taigrr/gojev`, `wawan93/gojev`).

Read `CONTEXT.md` before designing anything. Its "Hard No's" and verification
list are the review bar: no auto-approval, no silent zero values, no bare-float
probabilities, no API spend before validation, no schema change without a
version bump, and no debate logic (that stays in the `decide` skill).
`ROADMAP.md` maps work to GitHub issues in `rshade/jev-decide`.

## Commands

Go 1.27.1 is required (`ax-go` v0.7.0 needs it). There is no Makefile.

```sh
go build ./...
# safe, no spend:
go test ./decision/... ./jevclient/... ./eval/... ./internal/... ./cmd/...
go test ./decision -run TestChooseTurnsConfidence -v   # single test
go test ./internal/cli -run Golden -update   # rewrite golden files
golangci-lint run ./...
npx markdownlint-cli2 "**/*.md"   # .markdownlint.yaml disables MD013 for tables
mise exec -- openspec ...         # OpenSpec is pinned in mise.toml
```

**Never run `go test ./...` or `go test` in the repository root casually.**
The root `*_test.go` files are live probes and spikes that read `./.env`
themselves, so unsetting `TYPESAFE_API_KEY` does not stop them, and they spend
real money. Run one deliberately, e.g.
`go test -run TestJevRecommendations -v -count=1`. The `live_test.go` files in
`decision/` and `jevclient/` skip unless `TYPESAFE_API_KEY` is set in the
environment (they do not read `.env`).

## Architecture

Three layers plus a metrics package, with a dependency rule enforced by a
test:

- `jevclient/` wraps the pinned `kataras/jev` client. `NewClient` is the only
  supported constructor: it reads the key from the environment, enforces an
  https-or-loopback `TYPESAFE_BASE_URL`, never follows redirects, retries only
  429/529, and redacts the key in logs. `WithResponseCache(dir)` is an opt-in
  on-disk cache (a `RoundTripper` keyed on method, host, path and body; only
  200s that answer every question asked; never invalidated). `Classify` maps
  failures to `ax-go` `contract.Error` codes, which must all be listed in
  `docs/jev-errors.md` (`docs_test.go` fails otherwise).
- `decision/` is the typed domain. `Choose[T ~string]` (over `Options[T]`) and
  `Rate` (over ordered `Levels`, 2 to 10) return sealed results:
  `Result[T]` (`Decided`, `Uncertain`, `Escalate`, consumed with `Match`) and
  `ScoreResult` (consumed with `MatchScore`). Only `Decided` has `Choice()` and
  only `ScoreDecided` has `Level()`; the others expose `Leading()`. The case is
  picked by confidence against `Thresholds` (`DefaultThresholds()`: confident
  0.9, floor 0.5, both placeholders until issue #6). Failures are errors,
  never result cases. `Question.Validate` runs before any request and returns
  `*FieldError` (empty or typed-nil `State`, size estimate over 32k tokens at
  bytes/4, more than 255 options). `Spec` is the JSON decision-spec document:
  `ParseSpec`, `MergeFlags`, then `Question` or `RateQuestion`.
- `eval/` scores labelled choice results (`NewResult`, `Compute`): accuracy,
  contested AUC, Brier, per-threshold precision/recall. Undefined metrics are
  an absent `Metric`, never 0. `OutcomeOf` mirrors `decision`'s unexported
  `classify` rule.
- `internal/cli/` and `cmd/jev-decide/` implement `ask`, `score` and `eval` on
  `ax.Execute`. `cli.Run` takes an injectable `Env` (stdio, `Getenv`,
  `NewClient(...jevclient.Option)`) so tests never touch the process. Output
  is a `contract` envelope with `data.schema_version`. Exit codes: 0 decided,
  10 uncertain, 11 escalate, `ax` codes 1 to 4 for failures (`eval` exits 0
  with a report); each must appear in `docs/jev-decide-cli.md`. `--dry-run`
  validates and stops without a key or a request.

`decision` and `jevclient` may import only `ax-go`'s `contract` package, and
`eval` only those plus `decision`; `internal/cli/deps_test.go` fails if any of
them pull in `ax`'s OpenTelemetry/gRPC dependencies.

### Output schema versioning

Golden files in `internal/cli/testdata/golden/` pin every outcome and the
`__schema` output per version (`*.v<N>.json`; the current version is 2). Any
change to output shape, including a new command in `__schema`, needs a
`SchemaVersion` bump plus a new set of golden files, not an edit to the
existing ones. Each golden case has a `since` version; a new command starts at
the current one.

## Tests

Unit tests use `httptest` through `jevclient` and never read `.env`. Keep it
that way: new tests that need Jev must fake it with `httptest`.

The root probe and spike tests (`probe_*_test.go`, `jev_*_test.go`) and
`testdata/` are throwaway research evidence. Do not copy their code into
production packages; rewrite instead. `.probe-cache/` holds cached API
responses and is gitignored.

## Research docs

- `docs/reference/typesafe-openapi.json`: the API contract (re-fetch from
  `https://api.typesafe.ai/openapi.json`). Only `POST /v1/systemone` and
  `GET /v1/models` are used.
- `docs/probe-*.md`, `docs/spike-decisions-2026-09-29.md`: empirical findings.
  Jev ranks well (AUC about 0.91) but is not calibrated (Brier 0.236), so use
  it for gating and labelling, never auto-approval. Answers vary slightly
  between identical calls.
- `docs/jev-clients.md`: why `kataras/jev` was adopted (and its risks), plus
  `scripts/refresh-jev-clients.sh` to refresh client metadata snapshots.
- `docs/spec/`: a draft `finfocus-spec` scoring proposal. Recommendation
  scoring for FinFocus belongs in the finfocus repos, not here (issue #10).

## Conventions

- Key in `TYPESAFE_API_KEY` (from `console.typesafe.ai/keys`), kept in the
  environment or the ignored `.env`. Never depend on `openjevai/jev` or any
  non-TypeSafe gateway.
- Issues close through `Closes #N` in `PR_MESSAGE.md` (gitignored), never via
  `gh issue close`.
- Commitlint in related repos is v21, which parses a body line starting with
  `word:` as a footer; never start a commit-body line that way.
- Clone repos under review into the session scratchpad, not this directory.
- `/pick-issue` (`.claude/commands/pick-issue.md`, with a Codex wrapper in
  `.agents/skills/pick-issue/`) claims one `roadmap/current` issue through a
  `processing:roadmap` label and routes it through OpenSpec or straight to
  code. Its Phases 3 and 6 assume commits go to `main`; update them at v0.1.0.
