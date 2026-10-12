# Tasks

## 1. Schema version and served reference

- [x] 1.1 Extend `skills/skills_test.go` and the resource test in
  `internal/cli/mcp_test.go` so `references/decision-log.md` is embedded,
  listed, resolves against the `SKILL.md` URI and reads back byte for byte.
  Verify both fail without the file and pass with it.
- [x] 1.2 Set `SchemaVersion` to 7 in `internal/cli/output.go`. The v7
  goldens are regenerated in 3.4, after the commands exist.

## 2. Log store

- [x] 2.1 Write failing tests for a store in `internal/cli` (fake `Getenv`,
  fixed `Env.Now`, a temporary directory): `DECIDE_LOG` unset touches no
  file; the first append creates the file `0600` and its directory `0700`;
  each append adds one line with `v`, `kind`, `id`, `recorded_at` and
  `trace_id`; an existing file keeps its earlier lines byte for byte; an
  unwritable path returns an error; a malformed line fails a read with its
  line number; a missing file reads as empty. Verify they fail to compile.
- [x] 2.2 Write failing tests for the repository check: a log inside a git
  work tree, including `<repo>/not-yet/logs/decisions.jsonl` where neither
  directory exists, is refused without `--yes` (exit 2 naming `--yes`) and
  creates nothing; with `--yes` it is written; a `.git` file (worktree)
  counts; a path outside any work tree needs no `--yes`.
- [x] 2.3 Implement the store (design decisions 3 to 6): path from
  `Env.Getenv("DECIDE_LOG")`, nearest-existing-directory walk and `.git`
  search, `ax.Confirm`, `MkdirAll` `0700`, a single `O_APPEND` write under a
  mutex, a checked `Close`, decision ids from `crypto/rand` redrawn on a
  clash, `recorded_at` from `Env.Now`, `trace_id` from
  `ax.TraceIDFromContext`. Add `Env.Now` with a `time.Now` fallback. Verify
  2.1 and 2.2 pass, including under `go test -race`.

## 3. record commands

- [x] 3.1 Extract the decision and correct-option checks from
  `evalinput.go` into helpers both `eval` and `record` call, with no change
  to `eval`'s behaviour. Verify the existing `eval` tests and goldens pass
  unchanged.
- [x] 3.2 Write failing command tests through `cli.Run`: `record decision`
  writes a line whose `decision` has exactly `eval`'s fields and returns the
  id and log path; one option or a repeated name exits 2 and writes nothing;
  recording off exits 0 with `recording: false`. `record run`: unknown id,
  a `fast` path with a non-decided outcome, two pre-screen sources, debate
  fields on a non-debate path, and a citation with an unknown field or no
  `asked` each exit 2 and write nothing; a valid `debate` run keeps its
  citations; a second run line leaves the first. `record outcome`: `dominant`
  with an option the decision lacks exits 2 naming `correct-option`;
  `contested` writes `correct_option: null`; relabelling appends. `record
  pending`: only decisions past the threshold with no outcome, with no run
  field in the output, and `recording: false` with no file read when unset.
- [x] 3.3 Implement `record` and its four subcommands (design decisions 2,
  7 and 9), with output types in `output.go` and `id` marked
  non-deterministic. Verify 3.2 passes and the `deps_test.go` import rule
  still holds.
- [x] 3.4 Add golden cases since 7 for each `record` output (written,
  recording off, pending), regenerate the v7 goldens with
  `go test ./internal/cli -run Golden -update`, and verify no v6 or earlier
  file changed and `schema.v7.json` and `schema.mcp.v7.json` differ from v6
  only by the `record` commands, `eval --log`, the new resource and the
  version.
- [x] 3.5 Add MCP tests with no credentials: the four `go-decide-record-*`
  tools are listed, have no path input, write to the server's `DECIDE_LOG`
  only, and make no backend request. Verify they pass.
- [x] 3.6 Document `record` in `docs/jev-decide-cli.md`: each subcommand,
  its flags and output, opt-in through `DECIDE_LOG` in the go-decide
  process (the server's environment over MCP), the file mode and `--yes`
  rule, and version 7. Update `CLAUDE.md` (architecture and current
  version 7). Verify markdownlint and `go test ./internal/cli -run Docs`
  pass.

- [x] 3.7 From verification: make `--dry-run` on `record decision`, `run`
  and `outcome` check the input against the log and write nothing (it
  appended a line before), with a `record.dryrun` golden since 7 and the
  behaviour in `docs/jev-decide-cli.md`. Add a test that no line of the log
  holds a backend key or any other environment value. Verify the dry-run
  test fails with the check removed.
- [x] 3.8 From verification: refuse `--pre-screen` data that lacks, or holds
  as null, any field `ask` prints (including both thresholds, and an empty
  `probabilities`), and store the data as given rather than re-marshalled;
  refuse a citation with no `result`, `confidence` or option `probability`.
  Both had been written as zeros. Verify the new `TestRecordRunRefusesBadInput`
  cases fail without the checks.
- [x] 3.9 From an MCP smoke test: name inputs without a `--` prefix in
  `record` error messages, describe `pre-screen` and `citation` as a JSON
  object passed as a string, say "missing or empty" for an absent citation
  field, and make every invalid decision name its input in `context.field`
  (`option` and `context`, not `Options` or `State`; an unnamed `option`
  named nothing). Verify the field-naming cases fail without the fix.

## 4. eval --log

- [x] 4.1 Write failing tests: a log with one labelled and one unlabelled
  decision gives one item; the later of two `outcome` lines wins; no labelled
  decision exits 2 naming `--log`; `--log` with `--decisions` or `--truth`
  exits 2; nothing is sent in any of these.
- [x] 4.2 Implement `--log` (design decision 8) on the shared reader and
  validation, add an `eval.log` dry-run golden since 7, and document `--log`
  in the eval section of `docs/jev-decide-cli.md`. Verify 4.1, the goldens
  and markdownlint pass.

## 5. Skill, reference and constitution

- [x] 5.1 Rewrite `skills/decide/references/decision-log.md`: turning
  recording on (`DECIDE_LOG` in go-decide's environment, with the
  `claude mcp add -e` form), the record format with one `jsonl` example
  between an MD013 disable/enable pair, what to pass to each
  `go-decide-record-*` tool at each point, labelling guidance, and
  `eval --log`. Remove every shell and `jq` recipe.
- [x] 5.2 Replace `internal/cli/decisionlog_test.go` with a test that reads
  the reference's example through the store's reader, checks the `decision`
  line has no run field, and checks the labelled projection passes `eval`'s
  validation. Break check: an unknown field in the example's `decision`
  fails the test by name.
- [x] 5.3 Change the `SKILL.md` hooks to call the record tools (or the
  binary): Step 0.5 calls `go-decide-record-decision` and stays silent on
  `recording: false`; "Record the run" calls `go-decide-record-run` with the
  path, the pre-screen data, error or skip reason, and the debate fields;
  "Label past decisions" calls `go-decide-record-pending` and
  `go-decide-record-outcome` and never shows a run first. Remove
  `printenv`, `jq` and shell references. Verify the frontmatter is unchanged
  and the skill tests pass.
- [x] 5.4 Amend `CONTEXT.md`: verification item 3 and the Hard No's bullet
  allow the opt-in decision log written only by `record` and read only by
  `record pending` and `eval --log`. Update the `README.md` line on turning
  recording on for the MCP server. Verify markdownlint passes.

## 6. Walk and final verification

- [x] 6.1 Build the binary and, with `DECIDE_LOG` in the session scratchpad,
  run `record decision`, `record run` (with the decided envelope from
  `ask.decided.v7.json`), `record pending --older-than-days 0` (no run
  data), `record outcome`, and `eval --dry-run --log`. Verify exit codes,
  modes `0600`/`0700`, no environment value in the log, a refused in-repo
  path with nothing created, and no file with `DECIDE_LOG` unset.
- [x] 6.2 Run `go build ./...`, `gofmt -l .`, `go vet ./...`,
  `go test -race ./...`, `golangci-lint run ./...`,
  `mise exec -- markdownlint-cli2 "**/*.md"` and
  `mise exec -- openspec validate --all --strict`. Verify all pass.

## Workflow follow-up

- Run `openspec-verify-change`, then archive the change in the same pull
  request as the code.
- Tick #40 in `ROADMAP.md` with the archived change's path.
- `PR_MESSAGE.md` closes #40 and calls out the `CONTEXT.md` amendment. Note
  on #9 that recording is available once `DECIDE_LOG` is set for go-decide,
  and that its revisit condition is 20 labelled records.
