# gojev

Research workspace for a Go client for TypeSafe AI's Jev (System One) API. No
Go code yet; the module path `github.com/rshade/gojev` is unpublished.

## Layout

- `docs/spec/score-recommendations.md` and `docs/spec/recommendation_scoring.proto`:
  draft `finfocus-spec` proposal for a `RecommendationScorerService`, backed by
  `docs/probe-round2-2026-09-28.md`.
- `testdata/`: synthetic recommendations, pairs and two blind label files used
  by `probe_*_test.go`. `.probe-cache/` holds cached API responses (gitignored).
- `docs/jev-clients.md`: comparison of community Go clients and the checklist
  to run once an API token is available.
- `docs/reference/typesafe-openapi.json`: official OpenAPI spec snapshot
  (2026-09-28). Re-fetch from `https://api.typesafe.ai/openapi.json`.
- `docs/snapshots/*.tsv`: dated client metadata snapshots.
- `scripts/refresh-jev-clients.sh`: refreshes stars, tags and proxy versions
  for repos in `scripts/jev-clients.txt` (`--save` writes a snapshot).

## Project knowledge

- Jev is TypeSafe AI's System One model, not a file format or protocol.
  Endpoint `POST https://api.typesafe.ai/v1/systemone`, bearer auth, key in
  `TYPESAFE_API_KEY`. Keys come from `console.typesafe.ai/keys`.
- No official Go SDK exists. `gojev` is already the name of
  `taigrr/gojev` and `wawan93/gojev`.
- Avoid `openjevai/jev`: it can route data through an unverified third-party
  gateway.
- Keep the API token in `.env` only. Never commit it.
- Clone review targets into the session scratchpad, not this directory.
- FinFocus (`rshade/finfocus`) is the candidate consumer for recommendation
  triage. Its core generates no recommendations (plugins do, over gRPC) and
  forbids external API clients in core, so a Jev integration belongs in a
  plugin or behind a new `finfocus-spec` RPC. Core also drops priority,
  confidence and utilization from plugin recommendations and its
  recommendation cache key ignores resource IDs.
- `jev_recommendations_test.go` is the live probe (`go test -run
  TestJevRecommendations -v -count=1`), built on `kataras/jev` and skipped
  without `TYPESAFE_API_KEY` (env or `.env`). Results and lessons are in
  `docs/probe-2026-09-28.md`: use Jev for gating and labelling, not ranking.
  Its noul and score outputs vary slightly between identical calls.
- Round 2 findings: Jev ranks risk and false positive well against blind
  labels (AUC about 0.91) but is not calibrated (Brier 0.236) and must not
  auto-approve. Core's reduced recommendation loses 0.1 to 0.26 AUC; tags and
  metadata carry the false-positive signal; batching up to about 80 records
  works; pseudonymized identifiers keep duplicate detection. Total Jev spend
  for both rounds was about $0.12.
- Implementation is tracked in GitHub: `rshade/finfocus-spec#556` (scoring
  service, worktree `finfocus-spec-556`), `rshade/finfocus#1569` (retain full
  recommendations, cache-key fix, scoring step; worktree
  `finfocus/.worktrees/issue-1569`) and `rshade/finfocus#1570` (Jev scorer
  plugin at `plugins/jev/`, worktree `finfocus/.worktrees/issue-1570`). All
  changes there are uncommitted by design; the plugin builds against the spec
  worktree through a temporary `replace` that must be removed before merge.
- CLI/library name: `jev-decide`. GitHub repo
  `git@github.com:rshade/jev-decide.git` (has only the initial LICENSE, README
  and `.gitignore`); this directory is a
  git checkout of it on `main` with nothing committed or pushed, and `go.mod`
  already uses `github.com/rshade/jev-decide`. The local directory is still
  named `gojev`; do not rename it while implementation agents read it by
  path. Scope: thin typed Jev client, `ask`, `score` and `eval` commands on
  `ax-go`, and an optional fast-path pre-screen for the `decide` skill. The
  debate protocol itself stays in the skill.
- `jev-decide` must be typed so callers can rely on it: generic Go API over
  typed option sets (`Choose[T ~string]`), a validated probability type
  instead of bare floats, a sealed result type (decided, uncertain, escalate)
  so callers must handle low confidence, typed errors and no silent zero
  values, versioned JSON output schemas exposed through `ax-go` `__schema` and
  pinned by golden tests, and decision-spec input validated before any API
  call is spent.
- `finfocus-spec` PR #559 (branch `556-recommendation-scoring`, five commits)
  is open. Its CI runs commitlint v21; a local v20 accepted a body line that
  began with `word:` which v21 parses as a footer. Validate messages with the
  CI version (`npm i @commitlint/cli@21.2.3` in a scratch dir), and never
  start a commit-body line with `word:`.
- `mise.toml` pins OpenSpec (`npm:@fission-ai/openspec`, the tool ID is not
  `openspec`). Run it as `mise exec -- openspec ...`. `openspec init` needs
  `--tools claude` when non-interactive. It generated `openspec/` and 12
  skills plus 12 commands in `.claude/` (`/opsx:propose`, `/opsx:apply`, ...).
  markdownlint passes with them present, so no ignore entries yet.
- `CONTEXT.md` (boundaries) and `ROADMAP.md` are tracked against GitHub
  issues #1 to #11 in `rshade/jev-decide`. Spikes use `spike` and
  `timebox/*` labels. #10 was closed as `kill`: recommendation scoring
  belongs in the finfocus repos, not here.
