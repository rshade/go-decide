# Proposal

## Why

`decision.Choose` validates its options, thresholds and client before any
request, but not the rest of the question. A nil `State` reaches the API as
`null`, more than 255 options and an oversized state are only rejected by the
API after a round trip, and none of the failures name the offending field. The
"no unvalidated spend" boundary in `CONTEXT.md` is not yet met (issue #3).

## What Changes

- Add `Question[T].Validate()`, called by `Choose` before any request, that
  rejects a nil or empty `State`, and a question whose estimated size clearly
  exceeds the 32k limit for state plus the longest question.
- `NewOptions` rejects more than 255 options.
- Add a typed `*FieldError{Field, Reason}` that unwraps to a sentinel, so every
  rejection names the offending field and callers can use `errors.As`.
- Add tests: table-driven cases per rule, and an `httptest` counter proving an
  invalid question makes zero HTTP calls.
- Duplicate option names are not representable in `Options[T]` (it is built from
  a map). The spec records this; a duplicate check returns with the decision-spec
  loader, which is out of scope here.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `choice-decision`: the option-set requirement gains a 255 upper bound, and a
  new requirement validates the question (state and size) before any request,
  with errors that name the field.

## Impact

- Code: `decision/options.go`, `decision/choose.go`, a new error type and size
  estimate in `decision/`, and their tests. No new dependencies.
- API: `Question[T].Validate()` and `FieldError` are new exports. `Choose` can
  now fail for a nil `State`, which previously succeeded locally and failed
  remotely.
- Docs: `README.md` mentions validation, and `ROADMAP.md` item #3 is ticked.
- Out of scope: a decision-spec document type or JSON loader, and tuning the
  size estimate against a real tokenizer.
