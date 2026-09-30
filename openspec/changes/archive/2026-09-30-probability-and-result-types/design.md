# Design

## Context

See proposal.md for motivation. Observed state:

- `jevclient.NewClient` and `jevclient.Classify` exist (archived change
  `jev-client-adapter`). Failures already have a closed set of `ax-go` error
  codes, and `kataras/jev` already rejects a malformed 2xx response: missing
  answer, wrong type, confidence or probability outside 0 to 1, a choice that
  is not one of the options, or a missing probability for an option.
- `kataras/jev` returns a `ChoiceAnswer` with `Choice string`,
  `Confidence float64` and `Probabilities map[string]float64`. A `Choice`
  question takes `Criteria` as a map of option name to description.
- Go 1.27.1 allows generic types and functions but not generic methods, so the
  typed option set must be a type parameter on a function or a type, not on a
  method of the client.
- The spikes found the useful signal is the choice confidence: 0.98 on clear
  decisions against 0.58 on contested ones, separating around 0.9. Jev is not
  calibrated (Brier 0.236), so a high confidence means "clear enough to skip
  the debate", never "approved".

## Goals / Non-Goals

**Goals:**

- Make it impossible to obtain an actionable choice from a low-confidence
  answer, and impossible to forget a case when consuming a result.
- Keep every invalid input (options, thresholds) failing before any API call.

**Non-Goals:**

- JSON output and schemas (#4), full decision-spec validation (#3), tuning the
  placeholder thresholds (#6).
- Typed wrappers for `noul` and `score` questions.
- Mapping local validation errors to `ax-go` error codes. That belongs with the
  CLI that prints them (#4).

## Decisions

**1. One new package, `decision/`.** It holds `Probability`, `Thresholds`,
`Options[T]`, `Result[T]` and `Choose`. They only make sense together, and the
package imports `jevclient` and `kataras/jev` only, so the root probe tests stay
isolated. Alternative: several small packages (rejected: `Result` needs
`Probability`, and cross-package unexported access would force exporting
internals).

**2. `Probability` is a struct with a set flag, not a named `float64`.**
`type Probability struct{ v float64; set bool }`, created by
`NewProbability(f) (Probability, error)`. A named `float64` would let
`Probability(1.7)` compile and would make a never-set value look like a
probability of 0. Here the zero value reports `Valid() == false`, and the
comparison helpers return false when either side is invalid, so an unset value
cannot pass a threshold. Errors wrap `ErrInvalidProbability`. NaN, infinities
and anything outside 0 to 1 fail; 0 and 1 pass.

**3. `Thresholds` holds a floor and a confident level.** Created by
`NewThresholds(floor, confident)`, it requires both to be valid and
`floor < confident`, wrapping `ErrInvalidThresholds` otherwise.
`DefaultThresholds()` returns 0.5 and 0.9. Both are placeholders: 0.9 is the
spike's suggestion, and 0.5 is my choice, below the 0.58 mean confidence of
contested decisions, because the issue gives none. The zero `Thresholds` is
invalid, so a caller cannot pass an unset value and get silent defaults.

**4. `Options[T ~string]` is a validated map of option to description.**
`NewOptions(map[T]string) (Options[T], error)` copies the map and rejects fewer
than two options or an empty name, wrapping `ErrInvalidOptions`. A single
option is not a choice. Empty descriptions are allowed (Jev reads the name
alone). The zero `Options` is invalid. Alternative: accept a plain map in
`Choose` (rejected: nothing would prove it was validated, and #3 needs a
type to attach richer validation to).

**5. `Result[T]` is a sealed set of three interfaces.**
`Decided[T]`, `Uncertain[T]` and `Escalate[T]` are interfaces, each with its own
unexported marker method, implemented by unexported structs. Callers cannot
write a struct literal, so they cannot fabricate a decided result, and a type
switch tells the three apart because each marker is unique. All three share
`Confidence()`, `Leading()` and `Probabilities()`. Only `Decided[T]` has
`Choice()`, so an actionable option exists only on a decided result; the other
two offer `Leading()`, which reads as "not a decision". Alternative: exported
structs with unexported fields (rejected: `Decided[T]{}` would still compile
and pass as a result).

**6. `Match` gives compile-time exhaustiveness.**
`Match(r Result[T], onDecided, onUncertain, onEscalate) R` needs all three
handlers. A nil `Result` (the only way to hold a never-produced result) runs
the escalate handler with an empty escalate value: the safe direction, and never
a decision. Type switches remain possible; `Match` is the recommended path.
Compile-time behavior is tested by building small programs through
`go build -overlay`, which adds virtual files without writing into the
repository: a control with three handlers must build, one that omits a handler
must fail, and a type from outside the package must fail to satisfy `Decided`.
A reflection check on `Match`'s arity backs them up.

**7. Classification is a pure function.**
`classify(confidence, thresholds)` returns the case: at or above the confident
level is decided, at or above the floor is uncertain, otherwise escalate. It
is table-tested on its own, including both boundaries, so the branch logic
never depends on the network.

**8. `Choose` takes the client, a `Question[T]` and options.**
`Choose[T ~string](ctx, *jev.Client, Question[T], ...ChooseOption)
(Result[T], error)`. `Question[T]` holds `State`, `Instructions` and
`Options[T]`. `WithThresholds(Thresholds)` overrides the defaults. `Choose`
validates the options and thresholds first, builds one `jev.Choice` question
named `choice`, calls `SystemOne`, and converts the answer:
confidence and every probability go through `NewProbability`, and the
picked option must be in the set. Anything wrong returns an error wrapping
`jev.ErrResponse` through `jevclient.Classify`. The lookup and conversion live
in `fromResponse` and `fromAnswer`, which take plain `jev.Response` and
`jev.ChoiceAnswer` data, so the branches the client library already rules out
are still tested. Alternative: an interface in
place of `*jev.Client` for faking (rejected: we do not own the type, and
`httptest` against the real client is a truer test).

**9. Errors.** A call failure returns `jevclient.Classify(ctx, err)` unchanged,
including caller cancellation, which `Classify` returns as is. Local validation
failures return the sentinels above with a detail that says what to use instead
(`ErrInvalidOptions`, `ErrInvalidThresholds`), and a nil client returns
`ErrNilClient`, all before any request. There is no `Result` case for a
failure. `State` is not validated here: a nil `State` reaches the API as `null`,
and issue #3 owns that check.

**10. Tests.** Table-driven tests cover `NewProbability`, `NewThresholds`,
`NewOptions` and `classify`. `Choose` is tested against an `httptest` server
through `jevclient.NewClient`, one case per branch plus each failure, and a
counter proves an invalid option set sends no request. A live check reuses the
pattern from `jevclient`: it skips unless `TYPESAFE_API_KEY` is in the
environment.

## Risks / Trade-offs

- **Placeholder thresholds are unvalidated** → both are named placeholders in
  code and docs, and #6 tunes them on real decisions. The floor of 0.5 is my
  choice, not from data.
- **`Decided` can be read as "approved"** → the docs say it means clear enough
  to skip the debate, and `CONTEXT.md` still forbids auto-approval; the type
  gives no way to act without reading `Choice()`, but acting is the caller's
  responsibility.
- **A confident wrong answer stays `Decided`** → confidence is not calibrated
  (Brier 0.236). The type cannot fix that; the fast-path spike (#9) tests it on
  real decisions.
- **`Result` type switches are not compile-checked** → `Match` is the
  documented path and is tested for arity.
- **Hidden struct values behind interfaces hurt `%v` output and equality** →
  each unexported type gets a `String` method so logs stay readable, and callers
  compare through accessors.

## Open Questions

- Which `ax-go` error code the CLI gives local validation errors
  (`ErrInvalidOptions` and friends). Safe to decide in #4.
- Whether `Uncertain` should later carry a reason such as "tie". Does not
  change this change.
