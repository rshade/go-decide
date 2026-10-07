# go-decide v0.1.0 - Task Breakdown

**Goal:** Release v0.1.0 as a typed, safe-by-construction Go library and CLI
for System One decision models (Jev today), with `ask`, `score` and `eval`
working, the API key provably redacted, `go test ./...` safe to run anywhere,
a release pipeline that has been dry-run, and the module renamed to
`github.com/rshade/go-decide` before anyone imports it.

**Roadmap:** Commit straight to `main` until the v0.1.0 tag exists (no CI
gate yet). Then switch to branch-and-PR (JD-4.6).

Each task carries a **Status** line: `TODO`, or `DONE` with the command that
proves it and a break check (break the code on purpose, watch the proof fail,
restore it). A task is not done without a break check.

---

## Release and tag rules (decided before the first tag)

The release files are copied from `rshade/finfocus-plugin-azure-public`, a
setup that has proven solid, and adapted (JD-4.3, JD-4.4). There are still no
tags, milestones or `CHANGELOG.md`. The rules, including the lessons the
finfocus plugin family learned on its first releases:

1. `release-please-config.json` uses `"release-type": "go"`,
   `"include-component-in-tag": false` (otherwise tags gain a component
   prefix and GoReleaser cannot parse them) and `"initial-version": "0.1.0"`
   (otherwise the first release PR can propose 1.0.0).
2. `.release-please-manifest.json` keeps `"."` at `0.0.0` until the first
   release PR merges.
3. `bump-minor-pre-major` is true and `bump-patch-for-minor-pre-major` is
   true (decided 2026-10-06): before 1.0 a `feat:` bumps the patch, so
   releases stay on 0.1.x. The operator bumps minors by hand, with a
   `Release-As: 0.2.0` footer on a commit or an edit to the release PR.
   This differs from `rshade/ax-go` and azure-public, which use false.
4. Never hand-edit `CHANGELOG.md`. It is generated, and it fails
   markdownlint, so it goes in the markdownlint ignore list.
5. Wrap commit bodies at 72 columns. Never start a body line with `word:`,
   because commitlint v21 reads it as a footer.
6. GoReleaser builds the `go-decide` binary (after JD-3.3) for
   linux, darwin and windows on amd64 and arm64, plus archives and
   `checksums.txt`. No Docker image, Homebrew tap or deb/rpm for v0.1.0.
7. `release.yml` triggers on `release: created` and `workflow_dispatch`
   with a `tag` input, never `push: tags` (release-please creates tags
   through the API, so a tag trigger never fires). Use
   `googleapis/release-please-action@v5.0.0` and
   `goreleaser/goreleaser-action@v7`.
8. Release-please needs a token other than `GITHUB_TOKEN`
   (`RELEASE_PLEASE_TOKEN`), or its release PR will not trigger CI.
9. Prove the config before the first tag:
   `GORELEASER_CURRENT_TAG=v0.1.0 goreleaser release --snapshot --clean --skip=publish`
   and `goreleaser check` must both succeed and write archives.

---

## Scope (decided 2026-10-06)

**v0.1.0 is the current feature set, made safe to publish under its final
name.** No new decision features. Threshold tuning and the `decide`
pre-screen need real past decisions, which only the operator has, so they
are Tier B.

| Tier | Issues | Tasks |
| --- | --- | --- |
| Done | #1, #2, #3, #4, #5, #10, #11 | JD-0.x |
| A, v0.1.0 | #12, #13, #16, #17, #18, #19, #20 | JD-1.x to JD-4.x |
| B, after v0.1.0 | #6, #7, #8, #9, #21, #22, plus Clef provider support if the spike says go | JD-5.x |

### What the research found that this plan did not know (2026-10-06)

- **Cloudflare released Clef and Clef-flash on 2026-10-01** as open-weight
  (Apache 2.0) System One models on Workers AI, with weights on Hugging
  Face. Cloudflare says they follow the System One API (same `state`,
  `questions`, `noul`/`choice`/`score`, `criteria`, `answers`), take 64k of
  context against Jev's 32k, accept images, and were trained with a Brier
  loss for calibration. Its posts also give median latency of 209 ms
  (Clef), 39 ms (Clef-flash) and 524 ms (Jev). These are vendor claims,
  five days old.
- **Clef is not a base-URL swap.** The endpoint is
  `api.cloudflare.com/client/v4/accounts/<account>/ai/run/@cf/cloudflare/clef`,
  not `/v1/systemone`, with a Cloudflare token and an account id. Workers
  AI usually wraps responses in `{"result": ..., "success": ...}`. Whether
  it does here is unverified, and it decides whether `kataras/jev` can be
  reused through a transport.
- **Supporting Clef crosses two documented boundaries:** CONTEXT.md's "talk
  to `api.typesafe.ai`" Hard No and the `jev-client` spec requirement
  "Endpoint is restricted to TypeSafe or an explicit safe base URL". The
  32k limit in `Question.Validate` would become per-provider.
- **`go test ./...` is not offline.** The root probes skip without a key,
  except `jev_recommendations_test.go:198`, which calls the real API with a
  fake key. Any CI running `go test ./...` makes a live call.
- **Renaming the binary changes `__schema`** (its `tool` field), and the
  golden rule says any output change needs a `SchemaVersion` bump, not an
  edit to the frozen v2 files.
- **Name candidates:** bare `decide` collides with the `decide` skill, which
  CONTEXT.md defines as the debate this tool escalates to, and 27 public
  repos already use it. `go-decide` has 5 small repos. Go module paths are
  owner-scoped, so neither breaks anything technically.
- **Live baseline (2026-10-01, bundled 40 decisions, Jev):** accuracy 1.0,
  contested AUC 0.985, Brier 0.188, and 2 contested decisions decided above
  0.9 (dec-009 at 0.98, dec-011 at 0.91).

---

## Current State

| Item | Current | Note |
| --- | --- | --- |
| Module path | `github.com/rshade/jev-decide` | Local directory stays `gojev`; see JD-3.3 |
| GitHub repo | `rshade/go-decide` (renamed, found 2026-10-06) | `origin` still reads `rshade/jev-decide` and works through the redirect |
| Binary | `jev-decide` (`cmd/jev-decide`) | `ask`, `score`, `eval`, `__schema` |
| Output schema | `SchemaVersion` 2 | v1 and v2 goldens frozen; cases carry `since` |
| Go | 1.27.1 (`ax-go` v0.7.0 needs it) | No Makefile |
| Tooling pins | `mise.toml`: OpenSpec 1.13.2 only | Go, golangci-lint unpinned |
| CI, tags, releases | none | Repo is public |
| OpenSpec | 11 main specs, no active changes | Changes archive with their code commit |
| Open issues | #6, #7, #8, #9, #12, #13 | #12, #13 carry no `roadmap/*` label |
| Workflow | `/pick-issue` claims via `processing:roadmap` | Commits to `main` until v0.1.0 |

Offline gates, green at `64ac643`:

```bash
go build ./... && go vet ./... && test -z "$(gofmt -l .)"
go test -race ./decision/... ./jevclient/... ./eval/... ./internal/... ./cmd/...
golangci-lint run ./...
mise exec -- openspec validate --all --strict
```

---

## Phase 0: Shipped (JD-0.x)

### JD-0.1: ask and score commands with versioned JSON output

**Status:** DONE, `418f392`; golden files pin every outcome at v1; break
check is the golden test itself (a field added without a bump fails).

**Related Issues:** [#4](https://github.com/rshade/go-decide/issues/4)
(closed), on top of #1, #2 and #3 (closed 2026-09-30).

### JD-0.2: eval command, metrics package, response cache

**Status:** DONE, `64ac643`; offline gates green plus one approved live run
(see research); break checks: the wrong-question cache test failed before
the `answersEveryQuestion` fix, and the lazy cache-directory test failed on
the eager `MkdirAll`.

**Related Issues:** [#5](https://github.com/rshade/go-decide/issues/5)
(closed 2026-10-02).

---

## Phase 1: Safe to run, honest to read (JD-1.x)

### JD-1.1: Put the root probes behind a build tag

**Status:** TODO

**ID:** JD-1.1
**Description:** Add `//go:build probe` to every root `*_test.go`
(`probe_*_test.go`, `jev_*_test.go`), so `go test ./...` is offline and
safe for CI and contributors. Running a probe becomes
`go test -tags probe -run <Name> -count=1`. Keep the code as throwaway
evidence; do not move it into packages.

**Files Modified:** root `*_test.go`, `CLAUDE.md` (Commands), `README.md`.

**Acceptance Criteria:**

- `go test ./...` with no `.env` and no network makes zero HTTP requests,
  proved by running it with `HTTPS_PROXY=http://127.0.0.1:9`
- `go vet -tags probe ./...` still compiles the probes
- CLAUDE.md's "never run `go test ./...`" warning becomes "probes need
  `-tags probe` and spend money"
- break check: remove the tag from `jev_recommendations_test.go` and the
  dead-proxy run fails on the invalid-key test

**Related Issues:** [#16](https://github.com/rshade/go-decide/issues/16).

### JD-1.2: Clean markdownlint across the repo

**Status:** TODO

**ID:** JD-1.2
**Description:** `npx markdownlint-cli2 "**/*.md"` fails on files nobody
authors: vendored `.claude/skills/openspec-*/**` and
`.claude/commands/opsx/**`, an old archived `tasks.md`, the ignored
`PR_MESSAGE.md` (MD041), and later `CHANGELOG.md`. Add a
`.markdownlint-cli2.yaml` with `ignores` for those paths, fix the archived
line, and update `/pick-issue` Phase 4 if its command changes.

**Acceptance Criteria:** `npx markdownlint-cli2 "**/*.md"` exits 0 with no
CLI exclusions; break check: add a 90-column line to `README.md` and it
fails.

**Related Issues:** [#17](https://github.com/rshade/go-decide/issues/17).

### JD-1.3: Sync the roadmap with what shipped

**Status:** DONE 2026-10-06, `/roadmap sync`; `roadmap-check.sh` went from 15
errors to exit 0 with no findings. #12 and #13 moved to Immediate Focus with
`roadmap/current`, #4 and #5 moved to Completed (2026-Q4), and the closed
issues lost their phase labels.

**ID:** JD-1.3
**Description:** ROADMAP.md lists #5 as done under Near-Term (v0.2.0) while
Immediate Focus (v0.1.0) holds only #4. #12 and #13 carry no `roadmap/*`
label. Run `/roadmap` (the operator, not an agent) to promote the Tier A
issues, label #12 and #13, and move #4 and #5 to Completed Milestones.

**Acceptance Criteria:** every open issue carries exactly one `roadmap/*`
label matching its ROADMAP.md section.

**Related Issues:** #4, #5, #12, #13.

### JD-1.4: Correct #9's deliverable path

**Status:** TODO

**ID:** JD-1.4
**Description:** #9 asks for the verdict in `.specify/assessments/`, a Spec
Kit path. The repo uses OpenSpec, and earlier spikes landed in `docs/`
(`docs/spike-decisions-2026-09-29.md`). Propose the edit to the issue body;
editing an issue needs the operator's yes.

**Related Issues:** #9.

---

## Phase 2: Client hardening (JD-2.x)

### JD-2.1: Cover key redaction on retry and failure log lines

**Status:** TODO

**ID:** JD-2.1
**Description:** As specified in #12: in `jevclient/logging_test.go`, make
the server answer 429 (and separately a 5xx) with a body echoing the fake
key, at `TYPESAFE_LOG_LEVEL=info`, and assert the `typesafe retry` line is
present, the key absent and `[REDACTED]` present. Also cover the cache
path: with `WithResponseCache`, a logged failure still redacts.

**Acceptance Criteria:** break check by mutation: remove the `ReplaceAttr`
redaction in `newLogger` and the test fails. Tests stay `httptest` only.

**Related Issues:** [#12](https://github.com/rshade/go-decide/issues/12).

### JD-2.2: Document the logging policy and the cache in the package doc

**Status:** TODO

**ID:** JD-2.2
**Description:** As specified in #13: add the logging bullet to the
`jevclient` package doc (`TYPESAFE_LOG_LEVEL` values, stderr, key
replaced in every attribute, invalid level returns `jev.ErrConfig`). The
response cache section added for #5 is already there; check the two read
consistently with the README table.

**Acceptance Criteria:** `go doc ./jevclient` shows the bullet;
`golangci-lint run ./...` clean.

**Related Issues:** [#13](https://github.com/rshade/go-decide/issues/13).

---

## Phase 3: Provider and name (JD-3.x)

*The name is decided: `go-decide`. v0.1.0 waits for JD-3.1 (decided
2026-10-06), whose verdict now decides only whether Clef support (JD-5.5)
follows the release. The rename is free only before the first tag.*

### JD-3.1: Spike: is Clef a drop-in System One provider? (timebox 1d)

**Status:** TODO

**ID:** JD-3.1
**Description:** File as a `spike`, `timebox/1d` issue. In a root probe
(tagged `probe`, JD-1.1), call `@cf/cloudflare/clef` and
`@cf/cloudflare/clef-flash` through the Workers AI REST endpoint with one
`noul`, one `choice` and one `score` question. Record the exact request and
response bodies (envelope or not), error shapes, and whether `kataras/jev`
can reach it through a custom `http.RoundTripper` that rewrites the path
and unwraps the envelope. Then run the CLI's `eval` on the bundled 40
decisions for Jev, Clef and Clef-flash, with `--cache-dir` in a scratch
directory, and compare accuracy, contested AUC, Brier and decided-unsafe
at 0.9.

**Deliverable:** `docs/spike-clef-<date>.md` with a verdict (go /
needs-clarification / kill / parked) on two questions: is multi-provider
support a transport, or a second client; and does Clef's calibration claim
hold on this data (Brier against Jev's 0.188).

**Acceptance Criteria:** spend is approved first and stays under $1;
nothing from the probe is copied into packages; no Cloudflare token in the
repo.

**Related Issues:** [#18](https://github.com/rshade/go-decide/issues/18);
gates the release (decided) and decides JD-5.5.

### JD-3.2: Decide the name

**Status:** DECIDED 2026-10-06 by the operator: `go-decide`, module
`github.com/rshade/go-decide`, binary `go-decide`. Recorded in CONTEXT.md
as part of JD-3.3.

**ID:** JD-3.2
**Description:** The name drops `jev` so it stays true if Clef lands, and
avoids bare `decide`, which is the debate skill this tool escalates to.
`gh search repos` found 5 small `go-decide` repos (0 to 1 stars); module
paths are owner-scoped, so nothing conflicts. The binary keeps the `go-`
prefix so it never reads as the `decide` skill.

**Related Issues:** [#19](https://github.com/rshade/go-decide/issues/19).

### JD-3.3: Rename to go-decide

**Status:** TODO

**ID:** JD-3.3
**Description:** Change the module path to `github.com/rshade/go-decide` in
`go.mod` and every import, move `cmd/jev-decide/` to `cmd/go-decide/` (and
`main` in `.goreleaser.yaml` with it),
change the cobra root `Use`, and update docs, README, CLAUDE.md, CONTEXT.md
("No name collision" now names `go-decide`), ROADMAP.md, `/pick-issue`
and its Codex wrapper. The GitHub repo is already `rshade/go-decide`; point
`origin` at it with
`git remote set-url origin git@github.com:rshade/go-decide.git`.
Keep the product words honest: the CLI asks a System One model, Jev by
default. The
local directory may stay `gojev`. The binary name appears in `__schema`
(`tool`) and in help text, so bump `SchemaVersion` to 3 and add v3 goldens;
never edit the v1 or v2 files.

**Acceptance Criteria:** offline gates green; `go list -m` shows the new
path; `__schema` v3 shows the new tool name; break check: an old import
path left in one file fails `go build`.

**Related Issues:** [#19](https://github.com/rshade/go-decide/issues/19).

---

## Phase 4: CI and release (JD-4.x)

*All of Phase 4 is [#20](https://github.com/rshade/go-decide/issues/20). It
depends on #16, #17, #19 and the #18 verdict.*

### JD-4.1: Pin the toolchain in mise

**Status:** PARTIAL, Go 1.27.1 and GoReleaser 2.18.2 are pinned for the
release workflow; golangci-lint is still unpinned.

**ID:** JD-4.1
**Description:** Pin Go 1.27.1 and golangci-lint in `mise.toml` beside
OpenSpec, so CI and local runs agree. `go.mod`'s `go` line must match.

### JD-4.2: CI workflow

**Status:** TODO

**ID:** JD-4.2
**Description:** `.github/workflows/ci.yml` on push and pull request:
build, `gofmt -l`, `go vet ./...`, `go test -race ./...` (offline after
JD-1.1), `golangci-lint run ./...`, `npx markdownlint-cli2 "**/*.md"`
(clean after JD-1.2), `openspec validate --all --strict`, and commitlint on
the pushed commits. Runs on GitHub-hosted runners (`ubuntu-latest`, decided
2026-10-06), with Go from `go.mod` and tools from `mise.toml`.

**Acceptance Criteria:** a green run on `main`; break check: push a
gofmt violation on a branch and CI fails.

### JD-4.3: release-please

**Status:** DONE (uncommitted), `go test ./internal/release` passes; break
check: `include-component-in-tag: true` fails `TestReleasePleaseConfig`, then
restored. Config and workflow copied from azure-public, actions pinned to
commit SHAs.

**ID:** JD-4.3
**Description:** Add `release-please-config.json`,
`.release-please-manifest.json` (`0.0.0`) and `release-please.yml` per the
release rules above, based on `rshade/ax-go`'s config. Add a test (Go or
script) that fails when `include-component-in-tag` or `initial-version` is
wrong.

### JD-4.4: GoReleaser

**Status:** DONE (uncommitted), `goreleaser check` passes and the snapshot
build writes six archives and `checksums.txt`; the binary reports
`0.1.0-SNAPSHOT-<sha>` because `main.version` is now injected (before, it
would have been `unknown`). `actionlint` is clean.

**ID:** JD-4.4
**Description:** `.goreleaser.yaml` for the CLI binary, with version
injected so `go-decide --version` (and `__schema`'s `version`) report the
tag (`ax.ResolveVersion` reads build info; confirm what it needs).
`release.yml` per the release rules.

**Acceptance Criteria:** the snapshot build and `goreleaser check` from rule
9 succeed locally and write archives and `checksums.txt`.

### JD-4.5: Install and version docs

**Status:** TODO

**ID:** JD-4.5
**Description:** README:
`go install github.com/rshade/go-decide/cmd/go-decide@latest`, the release
archives, what `schema_version` promises (shapes are pinned per
version, the tool name and thresholds are not API), and that the thresholds
are placeholders until #6. v0.1.0 ships with them (decided 2026-10-06), so
the README and `--help` say so plainly.

### JD-4.6: Cut v0.1.0 and change the workflow

**Status:** TODO

**ID:** JD-4.6
**Description:** Merge the release PR, confirm the GitHub release has
binaries, then switch to branch-and-PR: update `/pick-issue` Phases 3 and 6
(worktree and PR, as its own note says), CLAUDE.md, and the operator's
"no branches before v0.1.0" preference. Turn on branch protection requiring
CI.

**Acceptance Criteria:**
`go install github.com/rshade/go-decide/cmd/go-decide@v0.1.0` works from a
clean machine; the release lists archives and checksums.

---

## Phase 5: After v0.1.0, Tier B (JD-5.x)

### JD-5.1: Spike: does the fast path hold on real past decisions? (timebox 1d)

**Related Issues:** [#9](https://github.com/rshade/go-decide/issues/9).
Needs 20 or more real past decisions with known outcomes, in the
`eval` input format, which only the operator can supply. Gates #6 and #7.
`eval` (JD-0.2) is the measuring tool; if JD-3.1 says go, run it for each
provider.

### JD-5.2: Tune the confidence thresholds on real decisions

**Related Issues:** [#6](https://github.com/rshade/go-decide/issues/6).
Blocked by JD-5.1. Use `eval`'s per-threshold rows. Changing
`DefaultThresholds()` changes no output shape, but document the data and
precision/recall, and keep scores described as ranking signals unless
`eval` shows calibration.

### JD-5.3: Fast-path pre-screen for the decide skill

**Related Issues:** [#7](https://github.com/rshade/go-decide/issues/7).
Blocked by JD-5.1 and JD-5.2. The debate stays in the skill; this repo
supplies the pre-screen and its typed result.

### JD-5.4: Batching and pseudonymized identifiers

**Related Issues:** [#8](https://github.com/rshade/go-decide/issues/8).
Cap batch size, split larger input, keep per-record error attribution.
Probe evidence: about 80 records per request worked.

### JD-5.5: Clef as a second provider (only if JD-3.1 says go)

**Description:** A provider option in `jevclient` (or a sibling package),
an explicit base URL and credential per provider, per-provider input
limits in `Question.Validate`, CONTEXT.md and the `jev-client` spec updated
on purpose, and `eval` able to compare providers. Images stay out of scope.
Goes through OpenSpec (it changes the client API and a Hard No).

---

### JD-5.6: Serve the commands as MCP tools

**Related Issues:** [#21](https://github.com/rshade/go-decide/issues/21). Mount
`ax-go`'s `mcp.NewCommand` in `newRoot`. Uncertain and escalate must come back
as results, not errors; decide whether `eval` is excluded (it fans out into
paid calls). `mcp-server` appears in `__schema`, so share #19's `SchemaVersion`
3 bump if it lands before v0.1.0. Goes through OpenSpec.

### JD-5.7: Ship the decide skill in this repository

**Related Issues:** [#22](https://github.com/rshade/go-decide/issues/22).
First decide the source of truth: move it here from `rshade/agent-skills`
(recommended), or vendor a copy. Ships unchanged
in `skills/decide/`; the pre-screen step is #7. CONTEXT.md's "no debate
protocol" Hard No is reworded on purpose: no debate logic in Go.

## Issue Index (all ROADMAP issues)

| # | Title | State | Labels | Disposition | Task |
| --- | --- | --- | --- | --- | --- |
| [#1](https://github.com/rshade/go-decide/issues/1) | `jevclient`: typed Jev client core | closed 2026-09-30 | | done | JD-0.1 |
| [#2](https://github.com/rshade/go-decide/issues/2) | `decision`: validated Probability, sealed Result | closed 2026-09-30 | | done | JD-0.1 |
| [#3](https://github.com/rshade/go-decide/issues/3) | `decision`: validate spec before any API call | closed 2026-09-30 | | done | JD-0.1 |
| [#4](https://github.com/rshade/go-decide/issues/4) | ask and score commands with versioned JSON schemas | closed | | done | JD-0.1 |
| [#5](https://github.com/rshade/go-decide/issues/5) | eval command for confidence separation and calibration | closed 2026-10-02 | roadmap/next | done (unpromoted pick) | JD-0.2 |
| [#6](https://github.com/rshade/go-decide/issues/6) | tune confidence threshold on real decisions | open | roadmap/next, effort/medium | Tier B | JD-5.2 |
| [#7](https://github.com/rshade/go-decide/issues/7) | fast-path pre-screen for the decide skill | open | roadmap/future, effort/medium | Tier B | JD-5.3 |
| [#8](https://github.com/rshade/go-decide/issues/8) | batching and pseudonymized identifiers | open | roadmap/future, effort/medium | Tier B | JD-5.4 |
| [#9](https://github.com/rshade/go-decide/issues/9) | spike: does the fast path hold on real past decisions? | open | roadmap/future, spike, timebox/1d | Tier B; body cites a stale `.specify/` path | JD-1.4, JD-5.1 |
| [#10](https://github.com/rshade/go-decide/issues/10) | spike: FinFocus scoring belongs in finfocus repos | closed 2026-09-30 | | done | none |
| [#11](https://github.com/rshade/go-decide/issues/11) | spike: keep `kataras/jev`, do not write a client | closed 2026-09-30 | | done; revisit if JD-3.1 needs a second client | JD-3.1 |
| [#12](https://github.com/rshade/go-decide/issues/12) | cover key redaction on retry and failure log lines | open | enhancement, effort/small | Tier A; no roadmap label | JD-2.1 |
| [#13](https://github.com/rshade/go-decide/issues/13) | document logging policy in the jevclient package doc | open | documentation, effort/small | Tier A; no roadmap label | JD-2.2 |
| [#16](https://github.com/rshade/go-decide/issues/16) | put the root probes behind a probe build tag | open | enhancement, effort/small, roadmap/current | Tier A | JD-1.1 |
| [#17](https://github.com/rshade/go-decide/issues/17) | make markdownlint clean across the repo | open | documentation, effort/small, roadmap/current | Tier A | JD-1.2 |
| [#18](https://github.com/rshade/go-decide/issues/18) | spike: is Clef a drop-in System One provider? | open | spike, timebox/1d, roadmap/current | Tier A; gates the release | JD-3.1 |
| [#19](https://github.com/rshade/go-decide/issues/19) | rename the module, binary and repo to go-decide | open | enhancement, effort/medium, roadmap/current | Tier A; repo already renamed | JD-3.2 (decided), JD-3.3 |
| [#20](https://github.com/rshade/go-decide/issues/20) | CI and release pipeline for v0.1.0 | open | enhancement, effort/large, roadmap/current | Tier A; release config done | JD-4.1 to JD-4.6 |
| [#21](https://github.com/rshade/go-decide/issues/21) | serve the commands as MCP tools with ax-go's mcp-server | open | enhancement, effort/medium, roadmap/next | Tier B | JD-5.6 |
| [#22](https://github.com/rshade/go-decide/issues/22) | ship the decide skill in this repository | open | enhancement, effort/small, roadmap/next | Tier B | JD-5.7 |

---

## Decisions (2026-10-06)

1. **v0.1.0 waits for the Clef spike** (JD-3.1).
2. **The name is `go-decide`**: module `github.com/rshade/go-decide`,
   binary `go-decide` (JD-3.2, JD-3.3).
3. **Patches for now:** `bump-patch-for-minor-pre-major` is true; the
   operator bumps minors by hand (release rule 3).
4. **GitHub-hosted runners** for CI and release (JD-4.2).
5. **v0.1.0 ships with the placeholder thresholds**, documented as such
   (JD-4.5); tuning waits for #9 and #6.

## Open Questions

None blocking. Whether Clef support lands after v0.1.0 is JD-3.1's verdict.

---

## Verification Commands (run before the release)

```bash
go build ./... && go vet ./... && test -z "$(gofmt -l .)"
go test -race ./...                       # offline after JD-1.1
HTTPS_PROXY=http://127.0.0.1:9 go test ./...   # proves no network
golangci-lint run ./...
npx markdownlint-cli2 "**/*.md"           # clean after JD-1.2
mise exec -- openspec validate --all --strict
goreleaser check
GORELEASER_CURRENT_TAG=v0.1.0 goreleaser release --snapshot --clean --skip=publish
./dist/*linux_amd64*/go-decide eval --dry-run \
  --decisions testdata/decisions.json --truth testdata/decisions_truth.json
```

---

## Notes

- **Order:** JD-1.1 before JD-4.2 (CI would make live calls), JD-3.3
  before JD-4.3 (the release carries the final name), and JD-3.1 before
  JD-4.6 (v0.1.0 waits for the spike). JD-3.1 and JD-3.3 are independent:
  the spike's probe can run under either name.
- **Spend:** only JD-3.1 and an optional final live `eval` spend money;
  both need the operator's approval.
- **Sources:** Cloudflare's Clef announcement
  (`blog.cloudflare.com/clef-decision-models/`) and Workers AI changelog
  (2026-10-01), the 2026-10-01 live `eval` run, and the issue bodies above.
