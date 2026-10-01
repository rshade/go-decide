# Design

## Context

Today only Go callers reach `decision.Choose`. `jevclient.NewClient` builds the
client from the environment and `jevclient.Classify` maps failures to
`ax-go` `contract.Error` values with the shared exit codes (0 success, 1
internal, 2 validation, 3 network, 4 auth). `Question.Validate` and
`NewOptions` already validate a choice before any request. See proposal.md for
motivation.

Findings that shape the approach:

- `ax-go`'s `__schema` reflects the cobra command tree (flags, and locators for
  non-deterministic output fields via `WithNonDeterministicFields`). It does not
  publish a JSON Schema of the output data. The output version therefore has to
  travel in the payload and be pinned by golden files.
- `ax.Execute` runs the cobra tree with mode resolution (`--format`, then
  `AGENT_MODE`, then TTY detection), adds the persistent flags `--format`,
  `--dry-run`, `--yes` and `--idempotency-key`, adds `__schema` itself, writes a
  returned error as the error envelope on stderr, and returns its exit code. It
  returns 0 whenever the command returns nil, and it links OpenTelemetry and
  gRPC (about 410 packages against 103 for the isolated packages).
- Jev's `score` question returns a fractional score over at most 10 levels, plus
  a probability per level and a confidence, so a score result has no analogue in
  the choice `Result`.

## Goals / Non-Goals

**Goals:**

- One decision-spec format, validated before any request, for both commands.
- A score API as typed and hard to misuse as `Choose`.
- Output and exit codes that scripts can rely on across versions.

**Non-Goals:**

- `eval` (#5), threshold tuning (#6), batching and pseudonyms (#8).
- A JSON Schema document for the output, or a stability promise beyond the
  version bump rule.
- Reading configuration files or environment beyond what `jevclient` does.

## Decisions

**Layout: `cmd/jev-decide` is thin; logic lives in `internal/cli` and
`decision`.** `internal/cli` builds the cobra tree from an injected client and
writers, so tests drive commands with `httptest` and buffers instead of
processes. Alternative: logic in `main`. Rejected, it cannot be tested cheaply.

**`ax.Execute` runs the commands.** It gives one consistent agent experience
across the rshade CLIs: mode resolution, the error envelope, `__schema` and the
shared flags come for free. This reverses the earlier choice to import only
`contract`, accepting the heavier `go.mod` and binary for the CLI. The
`decision` and `jevclient` packages keep importing only `contract`, so library
users stay light, and a test guards that (`go list -deps` on those two
packages shows no OpenTelemetry or gRPC). Alternative: cobra plus the isolated
packages, hand-wiring exit codes, envelope and `__schema`. Rejected by the
owner in favor of the shared wiring.

**Spec format: one JSON document, flags fill in, conflicts are errors.**
A `--spec <file|->` document plus `--state`, `--instructions`, repeated
`--option name=description` and `--level name=description`. A field present in
both is rejected, naming the field, so a run never depends on hidden precedence.
The document is decoded with unknown fields disallowed. The spec type lives in
`decision`, next to the types it builds, and converts to `Question[T]` with
`T` as plain `string` because names come from input. Alternatives: flags win
(silent override, rejected), or flags only (awkward for large state).

**Duplicate names are checked on the ordered list before building the map.**
`Options` is map-built and cannot hold duplicates, so the spec decoder is where
duplicates are caught, closing the gap noted in the `choice-decision` spec. The
same check serves levels.

**Score API: `Levels` plus a sealed score result, separate from `Result`.**
`Levels` is an ordered validated set (2 to 10 names). `Rate` mirrors `Choose`:
same client, same thresholds, same `classify`. Its result family is sealed the
same way (decided, uncertain, escalate) but carries a fractional value and a
nearest level, and only the decided type exposes an actionable level. The
nearest level is the round of the score with halves rounding down, so an
in-between value never reads as a step up. Alternatives: reuse `Result[T]` with
the rounded level as a choice (rejected: it hides the fractional score and
mislabels an in-between value as a pick), or return a bare float (rejected:
against the no-bare-floats principle).

**`--dry-run` validates and stops.** `ax.Execute` mounts `--dry-run` on every
command and marks the envelope `dry_run: true`, so a command that ignored it
would spend money under a label that says it did not. The commands check
`contract.DryRunFromContext` after validating the spec and thresholds and before
building the client, then print a `DryRunOutput` (schema version, kind, names in
order, thresholds) and exit 0. No key is needed. Alternative: reject the flag.
Rejected, because validating a spec without spending is useful to agents.

**Human mode prints the same JSON in v1.** `ax` resolves human mode on a TTY,
but a separate human rendering would be a second output to version and test.
Both modes emit the JSON envelope; `--format` is accepted and changes nothing
for these commands yet.

**Output: the `contract` envelope with `data.schema_version` (integer) and
`data.outcome` (`decided`, `uncertain`, `escalate`).** Decided carries `choice`
(or `level` and `score`); the other two carry `leading` (or `nearest` and
`score`) so the actionable name is absent by construction. Thresholds used are
echoed. Fields that vary between identical calls (confidence, probabilities)
are marked non-deterministic in `__schema`.

**Version bump enforced by golden files, one per version.**
`testdata/golden/<command>.v<N>.json` and `schema.v<N>.json`. A test renders
the current output and compares it to the golden for the current version, and a
second test compares each committed older golden's own version field against its
file name, so editing an old golden fails. Bumping needs a new golden file, and
old ones stay. Alternative: a hash table of versions. Rejected as harder to
review than a readable golden diff.

**Exit codes: 0 decided, 10 uncertain, 11 escalate; failures use `contract`
codes.** Codes 5 to 9 stay free in case `ax-go` adds shared codes. Uncertain
and escalate are outcomes, not errors, so their full result goes to standard
output and no error envelope is written. `Execute` would turn a returned error
into an error envelope, so the commands return nil and record the outcome in
the `run` function, which maps it to 10 or 11 when `Execute` returned 0. A
failure keeps the exit code `Execute` returned. `docs/` lists every code and a
test fails if one is missing, the same pattern as `docs/jev-errors.md`.

## Risks / Trade-offs

- [Spec and flag surface is large to version] → The input schema is versioned
  the same way as the output, and unknown fields are rejected so it can grow.
- [Round-half-down for the nearest level is a judgment call] → It is stated in
  the spec scenario and covered by a test; the fractional score is always
  reported, so callers can apply their own rule.
- [Heavier dependency tree and binary] → Confined to `cmd/` and `internal/`;
  the library packages are guarded by a test. Telemetry is configured from the
  environment by `ax`, and stays off unless that is set.
- [The 10 and 11 exit codes are new API] → Documented and tested; changing them
  needs a schema version bump.
- [Score thresholds reuse the choice defaults, which are placeholders] →
  Issue #6 tunes them; the output echoes the thresholds used.
- [Uncertain exits non-zero, so `set -e` scripts stop] → Intended: an
  unreviewed uncertain answer should not silently continue.

## Open Questions

- None that affect the specs or tasks.
