# Design

## Context

`Choose` already validates client, options and thresholds up front, each with a
sentinel error (`ErrNilClient`, `ErrInvalidOptions`, `ErrInvalidThresholds`) and
a test that asserts zero server hits. `Question[T].State` is `any` and is sent
as given. `NewOptions` takes `map[T]string`. See proposal.md for motivation.

The API's documented limit is 32k for state plus the longest question. The unit
is not stated; context is 64k tokens and the round-2 probe sized batches in
tokens (about 375 per record), so the limit is read as tokens. There is no local
tokenizer.

## Goals / Non-Goals

**Goals:**

- Every check runs before the client is touched and costs nothing.
- Errors carry the field name in a form callers can read with `errors.As`.
- The size guard never rejects a question the API would accept.

**Non-Goals:**

- A decision-spec document type, JSON loader or schema. Duplicate names are
  checked there later, when input is an ordered list.
- Exact token counting, or tuning the estimate against the API.
- Changing thresholds validation or the client.

## Decisions

**`Question.Validate()` is exported and `Choose` calls it.** Callers, and the
future CLI, can check a question without a client. `Choose` checks the nil
client, then the question (`Validate` covers options, then state), then
thresholds. Alternative: validate only inside `Choose`. That gives the CLI
nothing to reuse and hides the rules.

**Typed `*FieldError{Field, Reason string}` with `Unwrap() error`.** The field is
data (`errors.As`), and the message reads `decision: invalid question: State:
is nil`. `Unwrap` returns a sentinel: the existing `ErrInvalidOptions` for the
option-count rule, so current `errors.Is` callers keep working, and a new
`ErrInvalidQuestion` for state and size. Alternative: embed the field in a
formatted message only. Rejected because it makes callers parse strings, against
the project's typed-errors principle.

**State emptiness is checked after JSON encoding, not by type switch.** Marshal
`State` once; reject `null`, `""`, `{}` and `[]`. That covers typed nils, empty
maps and slices and empty structs-as-objects with one rule, and the same bytes
feed the size estimate. A marshal failure is a `FieldError` on `State` too,
since the client would fail on it anyway. Alternative: `reflect` on the value.
More code, and it can disagree with what actually goes on the wire.

**Size estimate is (state JSON bytes + instructions + option names and
descriptions) / 4 tokens, rejected only above 32,000.** Chosen because JSON
is denser than prose, so bytes/4 underestimates tokens and the guard errs toward
sending. "Longest question" is the one question we send: instructions plus the
criteria (option names and descriptions), summed. The reason string reports the
estimate and the limit. Alternatives: a tokenizer dependency (rejected, no
official one, and the limit is documented as approximate) or a byte ceiling
(rejected, the unit would be wrong).

**The 255 cap lives in `NewOptions`.** The cap is a property of the option set,
and `Options` is only constructible there, so `Choose` needs no second check.

**Duplicate names are documented, not checked.** `map[T]string` cannot hold
duplicates. The spec records this as a scenario so the requirement from the issue
is accounted for, and the check moves to the spec loader.

## Risks / Trade-offs

- [Estimate too low, so an oversized question still reaches the API] → The API
  rejects it with a classified error, and no worse than today.
- [Estimate wrongly rejects a valid question] → Divisor 4 is a lower bound on
  tokens for JSON; the 32k limit is one the API enforces anyway.
- [Rejecting `""`, `{}` and `[]` is stricter than the API] → These carry no
  content to decide on, and the roadmap already asks for a nil-state rejection.
  Loosen it if a real caller needs an empty object.
- [Limits change without notice per the API docs] → Both limits are named
  constants in one place, easy to update.

## Open Questions

- None that affect the specs or tasks.
