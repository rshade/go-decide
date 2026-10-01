# jev-decide command line

`jev-decide` puts a choice (`ask`) or an ordered rubric (`score`) to Jev and
prints a typed outcome. Only a decided outcome is one to act on.

## Commands

```sh
jev-decide ask   --spec decision.json
jev-decide ask   --state "all tests passed" --instructions "Ship it?" \
                 --option ship="safe to release" --option hold="wait"
jev-decide score --spec incident.json
jev-decide score --state "checkout fails" --level minor="cosmetic" \
                 --level major="cannot buy"
jev-decide __schema
```

Both commands take the same thresholds: `--floor` (default 0.5) and
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

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Decided. |
| 1 | Internal failure, or a response that broke the contract. |
| 2 | Invalid input, including any spec that fails validation. |
| 3 | Network failure or timeout. |
| 4 | Authentication failure. |
| 10 | Uncertain: for a person. |
| 11 | Escalate: for the full `decide` debate. |

Codes 1 to 4 are the shared `ax-go` codes; see `docs/jev-errors.md` for the Jev
error codes behind them. Codes 5 to 9 are left free.

## Output version

`schema_version` is 1. The output of every command, and `__schema`, is pinned
by golden files in `internal/cli/testdata/golden/`, one per version. A change to
a shape fails the tests until `SchemaVersion` is increased and golden files for
the new version are added. Earlier golden files stay, and a test fails if one is
edited or removed. Regenerate the current version's files with
`go test ./internal/cli -run Golden -update`.
