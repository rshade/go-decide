# CLAUDE.md

<!-- markdownlint-disable-next-line MD013 -->
This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`go-decide`: a typed Go library and CLI for System One decision models, Jev
by default, for fast-path decision support. The module path is
`github.com/rshade/go-decide`; the local directory is still named `gojev`, so
do not rename it. `gojev` is taken upstream (`taigrr/gojev`, `wawan93/gojev`).

Read `CONTEXT.md` before designing anything. Its "Hard No's" and verification
list are the review bar: no auto-approval, no silent zero values, no bare-float
probabilities, no API spend before validation, no schema change without a
version bump, and no debate logic in Go (the debate stays in the `decide`
skill prompt).
`ROADMAP.md` maps work to GitHub issues in `rshade/go-decide`.

## Commands

Go 1.27.1 is required (`ax-go` v0.9.0 needs it). There is no Makefile.

```sh
go build ./...
# safe, no spend. Probes are behind -tags probe:
go test ./...
go test ./decision -run TestChooseTurnsConfidence -v   # single test
go test ./internal/cli -run Golden -update   # rewrite golden files
golangci-lint run ./...
# .markdownlint.yaml disables MD013 for tables
mise exec -- markdownlint-cli2 "**/*.md"
mise exec -- openspec ...         # OpenSpec is pinned in mise.toml
```

**Probes need `-tags probe` and spend money.** The root `probe_*_test.go` and
`jev_*_test.go` files are built only with that tag. `go test ./...` does not
run them. Run one deliberately, for example
`go test -tags probe -run TestJevRecommendations -count=1`. Those files read
`./.env` themselves. The `live_test.go` files in `decision/` and `jevclient/`
skip unless `TYPESAFE_API_KEY` is set in the environment, and the one in
`clefclient/` unless `CLOUDFLARE_AUTH_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` are
(none of them read `.env`; export it first with `set -a; . ./.env; set +a`).
`clef_probe_test.go` is the clef probe, with Jev's cached answers beside it.

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
- `clefclient/` builds a `*jev.Client` for Cloudflare's clef, with the same
  guarantees (token from `CLOUDFLARE_AUTH_TOKEN`, account from
  `CLOUDFLARE_ACCOUNT_ID`, https-or-loopback `CLOUDFLARE_BASE_URL`, no
  redirects, retries only 429). It cannot marshal `jev.Questions` itself (jev's
  encoder is private), so a `RoundTripper` swaps the System One path for the
  clef run path and unwraps Cloudflare's `result` envelope. `Classify` gives
  `clef.*` codes, listed in `docs/clef-errors.md`. The shared plumbing (base
  URL checks, no-redirect client, redacting logger, response cache) lives in
  `internal/clientkit`.
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
- `internal/cli/` and `cmd/go-decide/` implement `ask`, `score` and `eval` on
  `ax.Execute`. `cli.Run` takes an injectable `Env` (stdio, `Getenv`,
  `NewClient(...jevclient.Option)`) so tests never touch the process. Output
  is a `contract` envelope with `data.schema_version`. Exit codes: 0 decided,
  10 uncertain, 11 escalate, `ax` codes 1 to 4 for failures (`eval` exits 0
  with a report); each must appear in `docs/jev-decide-cli.md`. `--dry-run`
  validates and stops without a key or a request. `mcp-server` (ax-go's `mcp`
  package, imported only in `internal/cli`) serves `ask` and `score` as MCP
  tools, `go-decide-ask` and `go-decide-score`; `eval` is excluded with
  `mcp.Exclude` because one call is one paid request per decision. Outcomes
  over MCP are results, never errors, and `Run` ignores the recorded outcome
  for `mcp-server`, so the server's exit code is never 10 or 11. The server
  also serves the `decide` skill (`internal/cli/skill.go`): each embedded file
  as a static resource at `go-decide://skills/decide/<path>`, a short
  `decide` prompt, and `mcp.WithInstructions` text that points at `SKILL.md`.
  The instructions and those descriptions come from `SKILL.md`'s frontmatter
  `description`, so editing it changes `__schema` and needs a schema bump.

`decision`, `jevclient` and `clefclient` may import only `ax-go`'s `contract`
package, and `eval` only those plus `decision`; `internal/cli/deps_test.go`
fails if any of them pull in `ax`'s OpenTelemetry/gRPC dependencies.

`ask`, `score` and `eval` take `--backend jev|clef` (default `jev`). A run
uses one backend and its credentials only, never falling back to the other.
clef rejects a question with empty instructions (a 400, code 5006) where Jev
accepts it, so `backend.requiresInstructions` makes the CLI fail it as exit 2
before any request, on a dry run too. `decision.WithClassifier` makes `Choose`
and `Rate` classify failures with the backend's `Classify` (default
`jevclient.Classify`); the CLI passes it.
`Env.NewClefClient` is the injectable constructor for clef.

`skills/decide/` is the product `decide` skill (separate from
`.agents/skills/`, which is repo workflow tooling). Its optional
pre-screen calls `ask` or the MCP tool `go-decide-ask` and branches on
`decided`, `uncertain` and `escalate`. A decided result skips the debate
and is not approval. The debate itself stays in that prompt. The `skills`
package embeds those files for the binary (`//go:embed` cannot reach a parent
directory from `internal/cli`), so the repository file is the only copy, and
it may import only the standard library.

### Output schema versioning

Golden files in `internal/cli/testdata/golden/` pin every outcome and the
`__schema` output per version (`*.v<N>.json`; the current version is 6). Any
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

- Keys in `TYPESAFE_API_KEY` (from `console.typesafe.ai/keys`),
  `CLOUDFLARE_AUTH_TOKEN` and `CLOUDFLARE_ACCOUNT_ID`, kept in the
  environment or the ignored `.env`. Never depend on `openjevai/jev` or any
  gateway other than TypeSafe's and Cloudflare's own endpoints.
- Issues close through `Closes #N` in `PR_MESSAGE.md` (gitignored), never via
  `gh issue close`.
- Commitlint in related repos is v21, which parses a body line starting with
  `word:` as a footer; never start a commit-body line that way.
- Clone repos under review into the session scratchpad, not this directory.
- `/pick-issue` (`.claude/commands/pick-issue.md`, with a Codex wrapper in
  `.agents/skills/pick-issue/`) claims one `roadmap/current` issue through a
  `processing:roadmap` label and routes it through OpenSpec or straight to
  code. Since v0.1.0 every change lands as a pull request from a worktree
  (`git worktree add ../gojev-<N> -b issue-<N> origin/main`), never as a
  commit on `main`. CI must pass, and merging is the user's call.
