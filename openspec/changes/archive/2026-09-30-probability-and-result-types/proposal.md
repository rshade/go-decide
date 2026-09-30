# Proposal

## Why

Issue #2 asks for types that make misuse hard. Today a caller of the Jev
client gets a bare `float64` confidence and decides for itself what "confident
enough" means, so nothing stops code from acting on a shaky answer.
`CONTEXT.md` forbids that: no bare-float probabilities, no auto-approval, and
no way to act on an answer without handling the low-confidence cases. The
spikes also showed the useful signal is the choice confidence (about 0.98 for
clear decisions against 0.58 for contested ones), so a decision needs more than
a yes or no: confident, worth a human look, or not trustworthy at all.

## What Changes

- Add a validated `Probability` type. It rejects NaN, infinities and values
  outside 0 to 1 at construction, and its zero value is not a valid probability.
- Add `Thresholds` with two configurable levels, a floor and a confident level.
  Construction fails unless both are valid and the floor is below the confident
  level. Defaults are placeholders pending issue #6.
- Add a typed option set for choice questions, generic over a string-kinded
  option type, validated before any API call is made.
- Add `Choose[T ~string]`, which asks Jev one choice question and returns a
  sealed `Result[T]`: `Decided`, `Uncertain` or `Escalate`, chosen by comparing
  the answer's confidence with the thresholds.
- Only `Decided` exposes an actionable choice. `Uncertain` and `Escalate`
  expose the leading pick under a name that says it is not for acting on.
- Add `Match`, which takes one handler per case, so a caller cannot forget one
  at compile time.
- Failures stay errors: `Choose` returns transport and API failures classified
  by `jevclient.Classify`. They are not `Result` cases.
- Decisions taken with the user in this session: two thresholds (floor and
  confident level); `Uncertain` is the band between them, meant for human
  review; `Escalate` is below the floor, meant for the full `decide` debate.

## Capabilities

### New Capabilities

- `probability`: a validated probability value and the two-level thresholds
  built from it.
- `choice-decision`: asking a choice question over a typed option set and
  getting back a sealed result that cannot be acted on without handling low
  confidence.

### Modified Capabilities

None. `jev-client` and `jev-error-mapping` keep their requirements; this change
uses them.

## Impact

- New Go package `decision/`, importing `jevclient` and `kataras/jev`. No new
  module dependencies.
- No change to `jevclient`, the root probe tests, or any output format.
- Enables #3 (decision-spec validation feeds `Options`), #4 (the `ask` command
  prints a `Result`) and #6 (tunes the placeholder thresholds).
- Placeholders that #6 must replace: confident level 0.9 (the spike's
  suggestion) and floor 0.5 (chosen below the 0.58 mean confidence of contested
  decisions; the issue gives no value).
- Non-goals: JSON output schemas and golden tests (#4), full decision-spec
  validation (#3), threshold tuning (#6), typed wrappers for `noul` and `score`
  questions, batching (#8).
