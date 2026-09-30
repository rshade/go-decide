# Tasks

## 1. Package and probability types

- [x] 1.1 Create the `decision/` package with a package doc comment that says
      what `Decided` does and does not mean; verify `go build ./...` and
      `go vet ./...` pass
- [x] 1.2 Test first, then implement `NewProbability` with the set flag,
      `Valid`, `Float64` and comparison helpers that are false for an unset
      value; verify a table test covers 0, 1, 0.5, -0.01, 1.01, NaN, +Inf, -Inf
      and the zero value, and that `errors.Is(err, ErrInvalidProbability)` holds
- [x] 1.3 Test first, then implement `Thresholds`, `NewThresholds` and
      `DefaultThresholds`; verify a table test covers valid levels, equal
      levels,
      floor above confident, an unset level, the zero `Thresholds`, and the
      defaults of 0.5 and 0.9

## 2. Options and classification

- [x] 2.1 Test first, then implement `Options[T ~string]` and `NewOptions`;
      verify a table test covers two options, zero, one, an empty name, that the
      input map is copied, and that empty descriptions are accepted
- [x] 2.2 Test first, then implement the pure `classify` function; verify a
      table test covers a confidence just below, at and above each level, the
      extremes 0 and 1, and that no valid thresholds make a confidence below the
      confident level decided

## 3. Sealed results

- [x] 3.1 Test first, then implement the `Decided`, `Uncertain` and `Escalate`
      interfaces with unexported implementations and the shared accessors;
      verify a test shows a type switch tells the three apart, that only
      `Decided` has `Choice()` (a type assertion on the other two fails), and
      that `Probabilities()` returns a copy
- [x] 3.2 Test first, then implement `Match`; verify tests show exactly one
      handler runs per case, a nil `Result` runs the escalate handler, and
      reflection confirms `Match` requires three handlers
- [x] 3.3 Add a `String` method to each result implementation; verify a test
      shows the printed form names the case and the leading option

## 4. Choose

- [x] 4.1 Test first, then implement `Question[T]`, `ChooseOption`,
      `WithThresholds` and the up-front checks in `Choose`; verify tests with an
      `httptest` server show an invalid option set, zero `Options` and an
      invalid `Thresholds` each fail with the matching sentinel and the server
      receives no request
- [x] 4.2 Test first, then implement the request and answer conversion in
      `Choose`; verify `httptest` cases return decided at 0.95, uncertain at
      0.7,
      escalate at 0.3, and both boundaries, each carrying the leading option and
      a probability for every option
- [x] 4.3 Cover the failure paths; verify tests show a 401 returns the
      unauthorized error, a 200 with no answer returns the invalid-response
      error, an answer naming an option outside the set is an error, and a
      canceled context returns the cancellation, each with no result
- [x] 4.4 Add a live check that skips unless `TYPESAFE_API_KEY` is in the
      environment and asks one real choice question; verify it reports SKIP with
      the variable unset and passes against the real API with it set
- [x] 4.5 Document the package in its doc comment, add a `README.md` section
      with a `Match` example that compiles as an `Example` test, and note that
      the thresholds are placeholders; verify the example compiles and
      markdownlint passes

## 5. Records and verification

- [x] 5.1 Update `CLAUDE.md`, `CONTEXT.md` and `ROADMAP.md` to say the types
      exist and the thresholds are placeholders for #6; verify markdownlint
      passes on every edited file
- [x] 5.2 Run format, vet, lint, the full test suite and
      `openspec validate probability-and-result-types --strict`; verify all
      exit zero
- [x] 5.3 With the owner's approval, comment on issue #2 with what shipped;
      the `Closes #2` line in `PR_MESSAGE.md` closes it when the change merges.
      The issue was closed by hand on 2026-09-30, which the owner corrected:
      issues are closed by the commit message, never by hand

## 6. Verification follow-ups

- [x] 6.1 Add `ErrNilClient` and make the options and thresholds errors say
      what to use instead; verify
      `TestChooseRejectsInvalidInputBeforeAnyRequest`
      and `TestChooseRejectsANilClient` pass
- [x] 6.2 Extract `fromResponse` so the missing-answer branch is testable;
      verify `TestFromResponseRejectsAResponseWithoutAUsableChoiceAnswer` passes
      and statement coverage of `decision` is 100%
- [x] 6.3 Prove the compile-time guarantees by building programs through
      `go build -overlay`; verify `TestCallersMustHandleAllThreeCases` and
      `TestNoCodeOutsideThePackageCanCreateAResult` pass, and fail when the
      sealing markers are removed
- [x] 6.4 Record that `Choose` does not validate `State` in `ROADMAP.md` and
      `CLAUDE.md` so issue #3 picks it up; verify markdownlint passes
