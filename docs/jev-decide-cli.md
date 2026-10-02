# jev-decide command line

`jev-decide` puts a choice (`ask`) or an ordered rubric (`score`) to Jev and
prints a typed outcome. Only a decided outcome is one to act on. `eval`
measures how well Jev's confidence separates clear decisions from contested
ones on a labelled set.

## Commands

```sh
jev-decide ask   --spec decision.json
jev-decide ask   --state "all tests passed" --instructions "Ship it?" \
                 --option ship="safe to release" --option hold="wait"
jev-decide score --spec incident.json
jev-decide score --state "checkout fails" --level minor="cosmetic" \
                 --level major="cannot buy"
jev-decide eval  --decisions testdata/decisions.json \
                 --truth testdata/decisions_truth.json
jev-decide __schema
```

All three commands take the same thresholds: `--floor` (default 0.5) and
`--confident` (default 0.9). The defaults are placeholders until they are tuned
on real decisions. The token comes from `TYPESAFE_API_KEY`, never a flag.

## Decision spec

A spec is one JSON document, read from a file with `--spec file.json` or from
standard input with `--spec -`:

```json
{
  "state": "The build passed all 412 tests.",
  "instructions": "Is this build ready to release?",
  "options": [
    {"name": "ship", "description": "Release it now."},
    {"name": "hold", "description": "Needs more work."}
  ]
}
```

A score spec has `levels`, lowest first, instead of `options`. `state` may be a
string, an object or a list. A field the format does not define is rejected.

The flags `--state`, `--instructions`, `--option name=description` and
`--level name=description` fill in or replace the document. A field given both
ways is an error, never resolved silently. Repeated flags keep their order.

The spec is validated before any request, and a failure costs nothing: the
state must not be empty, an option or level name must be present and not
repeated, a choice takes 2 to 255 options, a rubric takes 2 to 10 levels, and a
state clearly over the API's 32k limit is rejected. The error names the field.

## Output

A result is one JSON envelope on standard output. A failure is an error
envelope on standard error and leaves standard output empty. An uncertain or
escalate outcome is not a failure: the full result is printed, with no error
envelope.

`ask` data:

| Field | Meaning |
| --- | --- |
| `schema_version` | Version of this shape. |
| `outcome` | `decided`, `uncertain` or `escalate`. |
| `choice` | The option to act on. Present only when decided. |
| `leading` | The leading option. Present only when uncertain or escalate. |
| `confidence` | From 0 to 1. Varies between identical calls. |
| `probabilities` | Probability of every option. Varies between identical calls. |
| `thresholds` | The `floor` and `confident` levels used. |

`score` data has the same fields, with `level` (decided only) and `nearest`
(uncertain or escalate) in place of `choice` and `leading`, and a fractional
`score` counting levels from 0. The nearest level rounds halves down.

## Dry run

`--dry-run` validates the spec and thresholds and stops: no request is sent, no
API key is needed and nothing is billed. It prints an envelope whose data has
`schema_version`, `dry_run` (always true), `kind` (`choice` or `score`), `names`
(the options or levels in the order given) and `thresholds`, and exits 0. An
invalid spec fails with exit code 2 just as it does without the flag.

## eval

`eval` reads a decision set (`--decisions`) and a truth file (`--truth`) in
the format of `testdata/decisions.json` and `testdata/decisions_truth.json`.
It asks Jev one choice question per decision, in file order. The question's
state is the decision's `title`, `context`, `constraints` and `options`.
Option fields beyond `name` and `description` (such as `pros` or
`cost_usd_per_month`) are sent as they are. `--instructions` replaces the
default "Given the stated constraints, which option should the team choose?".

A decision entry may have only `id`, `title`, `context`, `constraints` and
`options`, and a truth entry only `id`, `class`, `correct_option` and `why`.
Both files are validated before any request. The ids must match one to one.
`class` is `dominant`, which needs a `correct_option` that is one of the
decision's options, or `contested`, which has `correct_option: null`. Each
decision must also pass the checks `ask` applies to a spec. A failure exits 2
and names the field, as in `truth[dec-001].correct_option`.

A result is safe to fast-path only when the decision is dominant and Jev
picked its correct option. The report's data has:

| Field | Meaning |
| --- | --- |
| `schema_version` | Version of this shape. |
| `metrics.accuracy` | Correct picks over dominant decisions. `null` with none. |
| `metrics.contested_auc` | How well 1 minus the pick confidence detects contested decisions. `null` unless both classes are present. |
| `metrics.brier` | Mean squared gap between pick confidence and safe (1) or not (0). |
| `outcomes` | `decided`, `uncertain` and `escalate` counts under the thresholds, and `decided_unsafe`: decided but not safe. |
| `thresholds` | The `floor` and `confident` levels used. |
| `by_threshold` | One row per threshold (0.05 to 0.95 by 0.05, plus the run's two): `selected`, `safe`, `unsafe`, `precision` and `recall`; `null` when undefined. |
| `decisions` | One row per decision: `id`, `class`, `pick`, `confidence`, `correct_option`, `safe` and `outcome`. |

A `pick` is what Jev picked, not a decision; its `outcome` says whether it
cleared the confident level. Everything derived from confidence varies between
identical calls. `eval` measures and never approves: it exits 0 whenever it
prints a report, whatever the numbers. A failed call fails the whole run with
no partial report, and the error names the decision.

`--cache-dir dir` stores each successful response in `dir`, so a rerun of the
same set with the same instructions sends nothing, and a rerun after a failure
only pays for what was not answered. Entries never expire: delete the
directory to measure again after a model change. Without the flag nothing is
written. `--dry-run` validates both files and prints `decisions`, `dominant`,
`contested` and `thresholds` without a key or a request.

The bundled set is 40 synthetic, constraint-shaped decisions. Treat it as a
smoke test. Tuning the thresholds needs past decisions with known outcomes.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Decided, or an `eval` report was printed. |
| 1 | Internal failure, or a response that broke the contract. |
| 2 | Invalid input, including any spec that fails validation. |
| 3 | Network failure or timeout. |
| 4 | Authentication failure. |
| 10 | Uncertain: for a person. |
| 11 | Escalate: for the full `decide` debate. |

Codes 1 to 4 are the shared `ax-go` codes; see `docs/jev-errors.md` for the Jev
error codes behind them. Codes 5 to 9 are left free.

## Output version

`schema_version` is 2. Version 2 added `eval`; the `ask` and `score` shapes are
unchanged from version 1. The output of every command, and `__schema`, is pinned
by golden files in `internal/cli/testdata/golden/`, one per version. A change to
a shape fails the tests until `SchemaVersion` is increased and golden files for
the new version are added. An output that first appears in a later version has
golden files from that version on. Earlier golden files stay, and a test fails
if one is edited or removed. Regenerate the current version's files with `go
test ./internal/cli -run Golden -update`.
