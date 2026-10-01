# Proposal

## Why

`decision.Choose` is only reachable from Go. Roadmap #4 gives `jev-decide` its
command line: `ask` and `score`, so scripts and agents can get a typed answer
with a stable, versioned JSON shape and an exit code that says whether the
answer can be acted on. It also brings the decision-spec input format that
`CONTEXT.md` calls for, validated before any API call.

## What Changes

- Add a `jev-decide` binary (`cmd/jev-decide`) with `ask` and `score`
  commands, run through `ax-go`'s `ax.Execute` for mode resolution, the
  error envelope, `__schema` and the shared persistent flags.
- Add a decision-spec format, read from a JSON file or stdin and adjustable
  with flags, validated before any request. Duplicate option and level names
  are rejected here, which closes the gap left in #3.
- Add a typed score API in `decision`: a validated ordered level set and a
  sealed score result with the same three outcomes as `Choose`.
- Add versioned JSON output for both commands, exposed through `__schema` and
  pinned by golden tests. Any output change requires a version bump.
- Add exit codes: decided 0, uncertain 10, escalate 11, and the `ax-go` codes
  for failures.

## Capabilities

### New Capabilities

- `decision-spec`: the JSON and flag input format, and its validation before
  any request.
- `score-decision`: the typed score levels and sealed score result in the
  `decision` package.
- `ask-command`: the `ask` command over a choice question.
- `score-command`: the `score` command over an ordered rubric.
- `cli-output-contract`: the versioned JSON output, `__schema` exposure, golden
  pinning and exit codes shared by both commands.

### Modified Capabilities

None.

## Impact

- Code: new `cmd/jev-decide/`, `internal/cli/`, decision-spec and score
  code in `decision/`, golden files under `testdata/`. Importing the root `ax`
  package adds cobra, OpenTelemetry and gRPC to `go.mod`, which reverses the
  earlier decision to import only `contract`; `CLAUDE.md` is updated to match.
  The library packages `decision` and `jevclient` keep importing only
  `contract`, so library users do not pay for it.
- Docs: `README.md` usage, `docs/` schema notes, `ROADMAP.md` #4.
- Out of scope: the `eval` command (#5), threshold tuning (#6), batching (#8)
  and any recommendation scoring, which belongs in the finfocus repos.
