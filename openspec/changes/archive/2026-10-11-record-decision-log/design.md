# Design

## Context

See proposal.md for why. `cli.Run` executes the command tree through
`ax.Execute` with an injectable `Env` (stdio, `Getenv`, clients), and
`mcp-server` serves every command it does not exclude as a tool named from
its command path (`go-decide-ask`). Over MCP, a tool call must not make the
server read a file (`--spec` is refused while serving). ax-go supplies a
global `--yes` and `ax.Confirm`, which returns a validation error naming
`--yes` (exit 2) in machine mode, and `ax.TraceIDFromContext` for the call's
trace ID.

`eval` reads a decision set and a truth file, each decoded with unknown
fields disallowed, and validates each decision by building the choice
question `ask` would send (`internal/cli/evalinput.go`).

## Goals / Non-Goals

**Goals:**

- One validated, append-only, owner-only JSONL log, written by go-decide and
  covered by unit tests, with the skill calling it at fixed points.
- Export without a projection script: `eval --log` reads the log.

**Non-Goals:**

- Writing the log through the ax-go logger, or shipping it anywhere.
- Merging logs across machines, pseudonymising them (#8), or auto-labelling.
- Changing the skill's frontmatter `description`, whose trigger rate cannot
  be measured until #48 exists.

## Decisions

### 1. go-decide owns the recorder, and the constitution says so

The debate runs inside the skill served by `go-decide mcp-server`, so the
binary is present at every point a record is written. A Go recorder validates
every record, enforces the file mode and the repository check in code, and is
unit-tested. `CONTEXT.md` verification item 3 is amended to allow exactly
this log: opt-in, written only by `record`, read only by `record pending` and
`eval --log`. Nothing else in the library or CLI persists state.

Alternative: the skill appends lines with shell and `jq`. Rejected after
verification found a privacy bug in that prose (the repository check missed a
directory that did not exist yet), which no test could have caught, and it
needs a shell that an MCP-only agent may not have.

### 2. Commands, flags and tools

`record` is a parent with four subcommands. Each becomes a tool,
`go-decide-record-decision` and so on. Inputs are inline flags, because a
tool call may not make the server read a file the caller names:

| Command | Flags |
| --- | --- |
| `record decision` | `--title`, `--context`, `--constraint` (repeated), `--instructions`, `--option name=description` (repeated, the parser `ask` uses) |
| `record run` | `--id`, `--path`, one of `--pre-screen` (the envelope's `data` as a JSON object), `--pre-screen-error <code>`, `--pre-screen-skipped <reason>`; for `debate`: `--recommendation`, `--evidence-mode`, `--go-decide-calls`, `--citation` (repeated JSON object) |
| `record outcome` | `--id`, `--class`, `--correct-option`, `--why` |
| `record pending` | `--older-than-days` (default 30) |

The envelope data and citations are JSON-in-a-flag because the agent already
holds them as JSON. Each is decoded with unknown fields disallowed and
checked: an outcome of `decided`, `uncertain` or `escalate`; a citation's
`agent` (`advocate_a`, `advocate_b`, `moderator`), `round` (1 or 2), `asked`
and `options`.

### 3. The path comes only from the process environment

go-decide reads `DECIDE_LOG` through `Env.Getenv`. Over MCP that is the
server's environment (`claude mcp add go-decide -e DECIDE_LOG=… -- go-decide
mcp-server`), so a tool caller can never choose the file. Unset or empty:
the write commands return `recording: false` and touch nothing, and
`record pending` returns the same with an empty list.

### 4. Appending, not logging

The ax-go logger's events return no error, and zerolog sends a failed write
to its global error handler, which suits diagnostics but not evidence. The
store marshals the line, then: `os.MkdirAll(dir, 0o700)`,
`os.OpenFile(path, O_WRONLY|O_APPEND|O_CREATE, 0o600)`, one `Write` of the
line and its newline, and a checked `Close`. Any error fails the call, and
the output never says the record was written. A mutex serialises writes
within one server process. An existing file keeps its mode.

### 5. Repository check on the nearest existing directory

Before any `MkdirAll`, the store walks up from the log's directory to the
nearest directory that exists, then up from there looking for a `.git`
entry (a directory, or a file in a worktree). Inside a work tree, it calls
`ax.Confirm`, so the write proceeds only with `--yes`. No `git` binary is
needed, and a directory that does not exist yet cannot slip past the check.

### 6. Identity, time and trace

go-decide sets `v: 1`, `kind`, `recorded_at` (UTC, RFC 3339, from a new
`Env.Now` that tests fix), and `trace_id` from `ax.TraceIDFromContext`. A
decision id is the UTC date plus four hex characters from `crypto/rand`,
redrawn while it matches an existing `decision` line. A missing file is an
empty log. The `trace_id` is the recording call's own: it equals the `ask`
call's `meta.trace_id` when both run under one trace, as when the client
propagates a W3C `traceparent` in each call's `_meta` or the CLI runs with
`TRACEPARENT`.

### 7. One validation path shared with eval

The decision checks in `buildEvalItem` (option names, the choice question
`ask` would send) and `correctOption` are reused by `record decision` and
`record outcome`, so a record that is written is one `eval --log` accepts.
`record run` and `record outcome` read the log to require an existing
`decision` line for the id. A malformed line in the log fails the read with
its line number rather than being skipped.

### 8. eval --log

`eval --log` reads the log, keeps ids with an `outcome` line, and projects
the first `decision` line's `decision` and the last `outcome` line's `truth`
in file order, so ties in `recorded_at` cannot reorder labels. The projection
feeds the existing validation and question building. `--log` with
`--decisions` or `--truth` fails with exit 2, and so does a log with no
labelled decision.

### 9. Output and schema version 7

`record decision|run|outcome` print `{schema_version, recording, kind, id,
log}`; `id` and `log` are absent when recording is off. `record pending`
prints `{schema_version, recording, older_than_days, decisions}`, each
decision with `id`, `recorded_at`, `instructions` and `decision` only. All
exit 0 on success; validation failures exit 2. `id` is marked
non-deterministic. The v7 goldens, not yet released, are regenerated with
the new commands, outputs and `--log` flag.

### 10. The skill calls the tools; the reference documents the format

`SKILL.md` keeps its hooks at the same points and calls
`go-decide-record-decision`, `-run`, `-pending` and `-outcome` (or the
binary). When a result says `recording: false`, the skill says nothing. The
reference drops its shell and `jq` recipes and keeps the record format, what
to pass at each point, labelling guidance, how to set `DECIDE_LOG` for the
server, and `eval --log`. Its `jsonl` example is now checked by decoding it
with the store's own reader.

## Risks / Trade-offs

- [Two processes appending at once could interleave] → each line is one
  `O_APPEND` write, which local filesystems apply whole for lines this size;
  one server process also holds a mutex. A network filesystem is not
  supported, and the docs say so.
- [The operator remembers the model's answer while labelling] → `record
  pending` returns no run data, the skill shows nothing else first, and
  `contested` gives an honest label. #9's write-up states the limit.
- [Real decisions are sensitive and stored in plain text] → opt-in,
  owner-only, outside repositories unless `--yes`, and never a credential or
  environment value.
- [Opt-in means #9's corpus starts late] → the README, the CLI docs and
  `record pending`'s "recording off" result say how to turn it on.
- [A constitution change sets a precedent] → the amendment names this log
  and the commands that touch it, so any other persistence is still a Hard
  No.
- [`eval` re-asks with its own instruction, not the pre-screen's] → #9
  measures the question `eval` asks; `instructions` is kept for a later
  replay of the original framing.

## Migration Plan

No logs exist yet. Rollback is a revert; a log written in the meantime stays
valid JSONL that nothing reads.
