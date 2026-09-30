# Tasks

## 1. Field-naming error and option cap

- [x] 1.1 Add `FieldError{Field, Reason}` with `Unwrap`, and the
  `ErrInvalidQuestion` sentinel, in `decision/`. Verify: a unit test asserts
  `errors.As` yields the field and `errors.Is` matches the sentinel.
- [x] 1.2 Make `NewOptions` reject more than 255 options with a `FieldError` on
  `Options` that unwraps to `ErrInvalidOptions`, and name the constant for the
  cap. Verify: table tests for 254, 255 (accepted) and 256 (rejected), and the
  existing options tests still pass.
- [x] 1.3 Add a test documenting that `Options` cannot hold duplicate names.
  Verify: the test passes and states why (map keys).

## 2. Question validation

- [x] 2.1 Add `Question[T].Validate()` rejecting a nil, typed-nil, empty (`""`,
  `{}`, `[]`) and unencodable `State`, each as a `FieldError` on `State`.
  Verify: table-driven test per case, checking the field name and sentinel.
- [x] 2.2 Add the size estimate (state JSON bytes plus instructions plus option
  names and descriptions, divided by 4) and reject above the named 32,000
  constant, reporting the estimate and limit. Verify: table tests just under and
  clearly over the limit, and a test that a question near the limit is accepted.
- [x] 2.3 Call `Validate()` from `Choose` after the nil-client check and before
  the thresholds check. Verify: `TestChooseRejectsInvalidInputBeforeAnyRequest` gains
  nil-state, empty-state and oversized-state rows asserting `ErrInvalidQuestion`
  and zero server hits, and the existing valid-question tests are unchanged.

## 3. Docs and delivery

- [x] 3.1 Update `README.md` (validation paragraph and `FieldError` usage), and
  tick `ROADMAP.md` item #3. Verify: `markdownlint` passes on both files.
- [x] 3.2 Add the `Question.Validate` and `FieldError` example to
  `decision/example_test.go`. Verify: `go test ./decision/...` runs it.
- [x] 3.3 Run `go test ./decision/... ./jevclient/...` (never the repo root, it
  spends money), `make lint` and the formatter. Verify: all pass with no
  findings.
- [x] 3.4 Write `PR_MESSAGE.md` with `Closes #3` and validate it with
  commitlint. Verify: `cat PR_MESSAGE.md | npx commitlint` passes.
