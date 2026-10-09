# Tasks

## 1. Dependency and schema version

- [x] 1.1 Pin ax-go to a `main` pseudo-version that contains rshade/ax-go#270
  (already in `go.mod`), run `go mod tidy`, and verify `go build ./...` passes
  and the only failing test is the `schema` golden.
- [x] 1.2 Bump `SchemaVersion` to 6, write the `*.v6.json` goldens with
  `go test ./internal/cli -run Golden -update`, and verify `git diff --stat`
  shows no v5 or earlier file changed and the diff of `schema.v6.json`
  against `schema.v5.json` is only `--strict`, `warnings_as_errors` and the
  version number.

## 2. Embed the skill

- [x] 2.1 Write a failing test in a new `skills` package that reads
  `decide/SKILL.md` and `decide/references/debate-prompts.md` from the
  embedded tree and compares each with the file on disk. Verify it fails to
  compile because the package has no API yet.
- [x] 2.2 Add `skills/skills.go` with a package doc and one function
  returning an `fs.FS` over `decide/`. Verify the test from 2.1 passes, and
  `internal/cli/deps_test.go` still passes with `skills` importing only the
  standard library.

## 3. Serve resources, prompt and instructions

- [x] 3.1 Add MCP tests in `internal/cli/mcp_test.go` with no credentials set:
  resources are listed with `text/markdown`, reading `SKILL.md` returns the
  embedded bytes, `references/debate-prompts.md` resolved against the
  `SKILL.md` URI is listed, an unknown `go-decide://` URI fails, the
  `initialize` result's instructions contain the `SKILL.md` URI and say a
  call is paid, `decide` renders with `decision` and fails without it, and the
  fake backend receives no request. Verify they fail for the missing
  declarations.
- [x] 3.2 Declare one resource per embedded file on the root (walked in
  sorted order, URI `go-decide://skills/decide/<path>`), the `decide` prompt,
  and `mcp.WithInstructions` in `newRoot`, with the instructions and
  descriptions built from `SKILL.md`'s frontmatter `description`. Fail
  startup on a declaration or frontmatter error rather than ignoring it.
  Verify the tests from 3.1 pass, and `go test -race ./internal/cli` passes.
- [x] 3.3 Add the `schema.mcp` golden case (`__schema --as=mcp`, since 6),
  regenerate `schema.v6.json` and add `schema.mcp.v6.json`. Verify both list
  the prompt and the two resources, and neither contains the skill text (grep
  for a line of `SKILL.md`).
- [x] 3.4 Break check: remove the resource declarations and confirm the 3.1
  tests fail by name, then restore them.
- [x] 3.5 Document the served skill: an "MCP server" note in
  `docs/jev-decide-cli.md` (resources, prompt, instructions, no spend), a
  README line under the pre-screen section on getting the skill from the
  server, and `CONTEXT.md` and `CLAUDE.md` lines that say the binary serves
  the skill as text and holds no debate logic. Verify markdownlint passes and
  `go test ./internal/cli -run Docs` passes.

## 4. Debate agents use go-decide as evidence

- [x] 4.1 Add a "go-decide during the debate" section to
  `skills/decide/SKILL.md`: when tools are available, the cap of two calls per
  agent per round, cite as evidence with backend and outcome, `confidence` is
  a ranking score, `leading` is not a choice, a failure is reported and not an
  outcome. Verify the section covers every scenario of the `decide-skill`
  spec's first three requirements.
- [x] 4.2 Add the anchoring mode: a `Model evidence: blind | informed` line in
  the Step 0.4 brief, asking for it before Round 1 when the debate will run,
  the tools are available and the user named none, blind Round 1 prompts with
  no pre-screen result and no calls, and the mode in the Phase 3 results.
  Verify against the spec's fourth requirement, scenario by scenario.
- [x] 4.3 Add the tool block to `references/debate-prompts.md` for the Round 1
  and Round 2 prompts, marked as included only when the tools are available
  and, for Round 1, only in `informed` mode. Verify markdownlint passes and
  the skill frontmatter still parses (`name`, `description`).
- [x] 4.4 Rebuild and verify the served resource matches the edited file (the
  test from 2.1 and the read test from 3.1).

## 5. Final verification

- [x] 5.1 Run `go build ./...`, `gofmt -l .`, `go vet ./...`,
  `go test -race ./...`, `golangci-lint run ./...`,
  `mise exec -- markdownlint-cli2 "**/*.md"` and
  `mise exec -- openspec validate --all --strict`. Verify all pass.
- [x] 5.2 Manual check in Claude Code with this build registered as an MCP
  server: the instructions reach the session, `/mcp__go-decide__decide`
  renders, an `@` mention attaches the skill, and the model reads the skill
  resource on its own for a strategic choice with no `@` mention. Record the
  Claude Code version and the steps for the pull request and rshade/ax-go#270.
  The `@` mention needs an interactive session and moved to #47. The results
  are recorded on rshade/ax-go#270.

  Results so far (Claude Code 2.1.295, `claude -p` from an empty directory,
  `--strict-mcp-config`, Skill tool disabled, one run per row):

  | Run | Pointer wording | Question | Read skill | Paid calls |
  | --- | --- | --- | --- | --- |
  | 1 | "For such a decision, read …" | Postgres or SQLite cache | No | 0 |
  | 2 | same | Quote the MCP instructions | Quoted verbatim | 0 |
  | 3 | same | Rails or a Go rewrite | No | 0 |
  | 4 | "do not answer from your own judgment first: read …" | Rails or a Go rewrite | Yes | 1 (uncertain) |
  | 5 | directive | Postgres or SQLite cache | No | 0 |
  | 6 | directive | `/mcp__go-decide__decide` | Yes | 0 (tools off) |
  | 7 | directive | `@` mention of `SKILL.md` | Not attached in `-p` | 0 |

  Run 4 followed the skill: pre-screen, `leading` not a choice, ranking
  score not a probability, then asked stop-or-debate and blind-or-informed.
  The `@` mention still needs an interactive session.

  Tuning (same setup, `--max-turns 3`, trigger means the model read
  `SKILL.md`): six plain choices and three controls (easy), six implicit
  choices and three near-miss controls (hard), and reruns of the misses.

  | Wording | Choices triggered | Controls triggered |
  | --- | --- | --- |
  | v1: description, then directive pointer | 24/32 | 0/9 |
  | v2: v1 plus the shapes a choice arrives in | 11/12 | 0/6 |
  | v3: v2 with the pointer first (shipped) | 29/30 | 0/9 |
  | v4: v3 plus "technical or strategic" | 8/8 | 0/6 |

  v1 missed "split the suite or bigger runners" and "thoughts on switching
  docs to Starlight" in 8 of 12 tries; v3 caught both 12 of 12. v4 gained
  nothing measurable over v3, so the shorter v3 shipped. #48 makes this
  check repeatable.

  End to end with v3 (`.env` loaded, Jev backend):

  - Rails or a Go rewrite: the model read `SKILL.md`, resolved and read
    `references/debate-prompts.md` itself, and the pre-screen returned
    decided (`keep_rails`, 1.0). The debate was skipped, with the caveats.
    The same question had returned uncertain (0.76) in run 4 under a
    different framing.
  - Full debate on per-seat or usage pricing, informed mode: six agents,
    ten go-decide calls, at most two per agent per round. The results named
    the mode and the call count, and the consensus rested on the arguments.
    Agents framed questions for their own side: the usage advocate left
    per-seat out and got decided (0.95), and the revised usage advocate
    described its own option in most detail and got decided (0.99), while
    the moderator's evenly worded Round 2 question escalated (0.46). The
    citation now carries the instructions sent and every option or level
    with its probability, so a reader sees the framing; the framing itself
    stays free.

## Workflow follow-up

- [x] Before pushing: move ax-go from the pseudo-version to `v0.9.0` once
  rshade/ax-go#279 merges and the tag exists, run `go mod tidy`, and re-run
  5.1. `go.mod` must not name a pseudo-version.
- [x] Rebase onto `main` once #41 merges, and drop its commit from this
  branch.
- [x] Remove `spec-first` from #22, and update `ROADMAP.md`'s #22 line.
- [x] Run `openspec-verify-change`, then archive the change in the same pull
  request as the code.
- Follow-ups: #47 (`@` mention), #48 (trigger-rate check), rshade/ax-go#298
  (stdin EOF drops a response). The framing results are on #6. The fields the
  anchoring and cap questions need are on #40.
