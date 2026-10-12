# Proposal

## Why

Spike #9 asks whether go-decide's fast path holds on real decisions. It is
parked because no honest corpus exists. The synthetic set is a favourable
test, and the real decisions in `openspec/changes/archive/*/design.md` leak
their answers, because they were written after the choice. A corpus like that
can only be collected going forward, by recording each decision before anyone
knows the answer. #6 (threshold tuning) waits on #9, so nothing else on the
roadmap moves until this log exists.

go-decide writes the log itself. The debate runs inside the skill that
`go-decide mcp-server` serves, so go-decide is present for every step of a
run. Writing from Go makes each record validated, owner-only and testable,
where instructions an agent follows in a shell cannot be tested.

## What Changes

- New `record` commands, also served as MCP tools:
  - `record decision`: the decision as the pre-screen asks it (title,
    context, constraints, instructions, options), validated by `eval`'s
    decision rules, written before the pre-screen or the debate runs.
  - `record run`: the path taken (`fast`, `debate` or `stopped`), the
    pre-screen's envelope data, error code or reason it did not run, and for
    a debate the recommendation, evidence mode, call count and citations.
  - `record outcome`: an `eval` truth entry, validated against the
    decision's options.
  - `record pending`: decisions past a threshold (30 days) with no outcome,
    without any run data, for labelling.
- go-decide assigns each line's id, `recorded_at` and ax-go's `trace_id`, and
  appends it with a single append-mode write. A failed write is an error.
- Recording is opt-in, because real decisions can be sensitive. go-decide
  reads and writes the log only when its own `DECIDE_LOG` names the file; no
  flag or tool input can choose the path. The file is `0600`, a new
  directory `0700`, and a path inside a git working tree needs `--yes`.
- `eval --log <file>` evaluates the labelled decisions in a log directly.
- The `decide` skill calls the record tools at fixed points, and labels
  outcomes through `record pending` without showing the run first.
  `skills/decide/references/decision-log.md` documents the record format and
  labelling guidance.
- `SchemaVersion` moves from 6 to 7: the `record` commands and outputs,
  `eval --log`, and the new skill resource, with a new golden file for every
  output and the v6 files untouched.
- **BREAKING (constitution)**: `CONTEXT.md` verification item 3 changes from
  "no persistence in the Go library or CLI" to allow this one opt-in log,
  written only by `record`.

## Capabilities

### New Capabilities

- `decision-log`: what go-decide records about each decision, when and
  where; its opt-in and privacy rules; how outcomes are labelled without
  showing the model's answer; and when the skill records.

### Modified Capabilities

- `mcp-server`: the `record` tools are served; the record tools may use the
  server's own decision log and no other file; the skill resources include
  `references/decision-log.md`.
- `cli-output-contract`: `__schema` at version 7 lists the `record`
  commands, `eval --log` and the new resource, with v7 golden files.
- `eval-command`: `eval --log` reads a decision log.

## Impact

- New `internal/cli` files for the log store and the `record` commands;
  `internal/cli/eval.go` and `evalinput.go` (`--log`, shared validation);
  `internal/cli/cli.go` (registration, `Env.Now`); `output.go`
  (`SchemaVersion` 7 and the record output types); new `*.v7.json` goldens.
- `skills/decide/SKILL.md` and `skills/decide/references/decision-log.md`.
  The frontmatter `description` does not change, so the server instructions
  and #48's trigger rate stay put.
- `CONTEXT.md` (item 3 and the Hard No's bullet), `docs/jev-decide-cli.md`
  (`record`, `eval --log`, version 7), `README.md` (turning recording on for
  the MCP server), `CLAUDE.md` (architecture and current version).
- No new dependency and no spend: `record` makes no backend request and needs
  no key.
- Unblocks #9 once 20 or more labelled records exist, which then unblocks #6.
