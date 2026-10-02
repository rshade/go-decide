# Proposal

## Why

The thresholds in `DefaultThresholds()` (0.9 confident, 0.5 floor) are
placeholders. The only evidence behind them is the 2026-09-29 spike, run from a
throwaway test. Roadmap #5 adds an `eval` command that re-runs that
measurement on any labelled decision set. #6 can then tune the thresholds on
real past decisions, and anyone can check whether the fast path still holds
after a model change.

## What Changes

- Add an `eval` command. It reads a decision set and a truth file in the
  format of `testdata/decisions.json` and `testdata/decisions_truth.json`,
  validates both completely, asks Jev one choice question per decision, and
  prints a report:
  - accuracy on the decisions that have a correct option
  - AUC for detecting contested decisions from 1 minus the pick confidence
  - Brier score of the pick confidence against "safe to fast-path"
  - precision and recall per threshold
  - one row per decision
- Add a new `eval` library package that computes those metrics from labelled
  results. It is pure, so it is unit-tested without Jev, and #6 can reuse it.
- Add an opt-in response cache to `jevclient`. With `--cache-dir`, `eval`
  stores each successful Jev response on disk under a hash of the request, so
  a rerun of the same set costs nothing. The cache never stores the API key.
- Support `--dry-run` on `eval`: validate both files, report the decision
  count by class, and send nothing.
- Bump `SchemaVersion` from 1 to 2, because `__schema` now lists a new
  command. The `ask` and `score` shapes are unchanged apart from the version
  number, and the v1 golden files stay frozen. Consumers that check
  `schema_version == 1` will see 2. That is the intended signal, not a break
  in shape.

## Capabilities

### New Capabilities

- `eval-metrics`: the labelled-result model and the accuracy, AUC, Brier and
  per-threshold precision/recall computations.
- `eval-command`: the `eval` command, its input files and their validation,
  its report, dry run and failure behavior.

### Modified Capabilities

- `jev-client`: adds an opt-in, on-disk response cache keyed on the request,
  which never stores the credential and caches only successful responses.
- `cli-output-contract`: `__schema` lists `eval`, the dry-run and exit-code
  rules cover `eval`, and golden files for an output that first appears in a
  later version start at that version.

## Impact

- Code: new `eval/` package, new `internal/cli/eval.go`, an option in
  `jevclient`, new golden files for every case at v2, and a `deps_test.go`
  entry so that `eval` imports only `decision` and `contract`.
- Docs: `docs/jev-decide-cli.md` (the command, its flags and its exit codes),
  `README.md` usage, `CONTEXT.md` (eval is no longer only planned) and
  `ROADMAP.md` #5.
- Spend: one Jev call per decision, which is 40 calls for the bundled set.
  The spike's run of 120 calls cost under $0.01. Nothing is sent before both
  files validate, and nothing is sent for a cached request.
- Out of scope: threshold tuning (#6), option-order and repeat-stability
  checks, the "clear winner" and "disagreement" yes/no questions, concurrent
  requests and batching (#8).
