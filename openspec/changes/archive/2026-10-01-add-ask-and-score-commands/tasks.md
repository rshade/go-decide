# Tasks

## 1. Score API in decision

- [x] 1.1 Add `Levels` (ordered, 2 to 10 names, no empty or duplicate name)
  with `FieldError` on `Levels`. Verify: table tests for 1, 2, 10, 11 levels,
  an empty name and a duplicate name.
- [x] 1.2 Add `Rate` and the sealed score result (decided, uncertain, escalate)
  built on `classify`, with the nearest level rounding halves down and errors
  for a non-finite, out-of-range or incomplete answer. Verify: `httptest`
  tests through `jevclient` for each result, an in-between score, a score
  outside the rubric and a missing probability.
- [x] 1.3 Add a sealed-type test and an example for the score result, matching
  the choice ones. Verify: `go test ./decision/...` passes and the sealed test
  fails to allow a fourth implementation.

## 2. Decision spec

- [x] 2.1 Add the spec type and JSON decoder (unknown fields and trailing data
  rejected, order kept, `FieldError` naming the field). Verify: table tests for
  valid choice and score specs, an unknown field and malformed JSON.
- [x] 2.2 Reject duplicate option and level names on the ordered lists, and run
  the existing question validation on the converted spec. Verify: tests for
  duplicates, empty names, over 255 options, over 10 levels and an oversized
  state, each checking the field name.
- [x] 2.3 Merge flags into a spec, rejecting a field given both ways. Verify:
  tests for flags only, a flag filling a missing field, and a conflict naming
  the field.
- [x] 2.4 Fixture tests reuse the `testdata/decisions.json` shapes through a
  converter. Verify: every decision in the file loads or fails as expected.

## 3. Commands and output

- [x] 3.1 Add `cmd/jev-decide` and `internal/cli`: a `run` function taking
  args, streams, env and an injected client, which builds the root command and
  calls `ax.Execute` with `WithVersion`. Register each command's output with
  `schema.WithNonDeterministicFields`. Verify: a test runs `__schema` through
  `run` and finds both commands, and a test shows `go list -deps ./decision
  ./jevclient` has no OpenTelemetry or gRPC.
- [x] 3.2 Implement `ask` with threshold flags and the versioned envelope.
  Verify: `httptest` tests for decided, uncertain and escalate output, custom
  and invalid thresholds, and a levels spec given to `ask`.
- [x] 3.3 Implement `score` the same way. Verify: the same set of tests for
  score, plus an in-between score.
- [x] 3.4 Record the outcome in `run` and map it to 0, 10 or 11 after
  `ax.Execute` returns, keeping `Execute`'s code for failures, which print the
  error envelope on stderr. Verify: tests for every code, that standard output
  is empty on failure, and that no error envelope is written for an uncertain
  or escalate outcome.
- [x] 3.5 Add a test that every invalid spec makes zero HTTP calls through the
  commands. Verify: a table of invalid specs asserts a hit count of zero.

## 4. Golden pinning and docs

- [x] 4.1 Add golden files for each output and `__schema`, version 1, and the
  tests that compare them and reject an edited older golden. Verify: changing a
  field name makes the test fail naming the version.
- [x] 4.2 Add `docs/` notes: exit codes with a test that fails if one is
  undocumented, the spec format and the output version policy. Verify: the
  documentation test passes and markdownlint is clean.
- [x] 4.3 Update `README.md` usage, tick `ROADMAP.md` #4, and correct the
  `CLAUDE.md` note that only `contract` is imported. Verify: markdownlint is
  clean.
- [x] 4.4 Run `go test ./decision/... ./jevclient/... ./internal/... ./cmd/...`
  (never the repo root), `golangci-lint`, the formatter and `go mod tidy`.
  Verify: all pass with no findings. Write `PR_MESSAGE.md` with `Closes #4` and
  validate it with commitlint.

## 5. Dry run and review follow-ups

- [x] 5.1 Honor `--dry-run` in `ask` and `score`: validate, then print a
  `DryRunOutput` and exit 0 without building the client or sending a request.
  Verify: tests assert zero server hits, the listed names in order, success
  with no API key, and exit 2 for an invalid spec in a dry run.
- [x] 5.2 Pin the dry-run output with golden files for both commands and
  document it in `docs/jev-decide-cli.md`. Verify: the golden tests pass and
  markdownlint is clean.
- [x] 5.3 Cover the review gaps: invalid and custom thresholds through `score`,
  the `ask` and `score` flags in the `__schema` test, a network failure exiting
  3, and `scoreFromAnswer` branches the client normally prevents. Verify: the
  new tests pass and `decision` coverage rises.
