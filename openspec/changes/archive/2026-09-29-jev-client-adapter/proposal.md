# Proposal

## Why

Issue #1 asked for a typed Jev client, and issue #11 asked whether to keep
`kataras/jev` or write our own. Exploration answered both: `kataras/jev`
v0.1.0 already encodes all three question types, accepts `state` as string,
object or array, exposes the request ID, refuses redirects, and rejects every
malformed 2xx response (missing answer, wrong type, value outside 0..1, choice
not in the options, non-finite score). It is also the only client live-tested
in the probes. Writing another client would mean maintaining code that
duplicates it. What the dependency lacks is our policy: safe defaults for
spend and base URL, and errors that speak `ax-go`.

## What Changes

- Add `github.com/kataras/jev` (pinned to an exact version) and
  `github.com/rshade/ax-go` (pinned; minor bumps may break) as dependencies.
- Add a thin adapter with one constructor, `NewClient`, that builds a
  `kataras/jev` client with an explicit retry policy, an https-only base URL
  policy (loopback excepted), and the token taken from `TYPESAFE_API_KEY`
  only.
- Add an error classifier that turns `kataras/jev` errors into `ax.Error`
  values, preserving status, request ID, `retryable` and
  `retry_after_seconds`, and extracting `error_type` from the
  `{"detail": {"error_type", "message"}}` body that `kataras/jev` does not
  expose.
- Record the adoption decision and the dependency risk (one commit, five
  stars, `v0.1.0`) in `docs/`, and mark the issue #11 spike as decided.
- Rewrite the requirements of issue #1: request and response types are no
  longer ours to build.
- No live API call runs in default `go test`.

## Capabilities

### New Capabilities

- `jev-client`: constructing a Jev client from the environment with an
  explicit retry policy, a validated base URL and no token leakage.
- `jev-error-mapping`: classifying transport and API failures into typed
  `ax.Error` values, including `error_type`, request ID and retry hints.

### Modified Capabilities

None. `openspec/specs/` is empty.

## Impact

- New non-test Go code in a subpackage (the repository root holds throwaway
  `probe_*_test.go` files and stays isolated from it).
- `go.mod`: `kataras/jev` moves from a test-only probe dependency to a
  production dependency; `ax-go` is added. `kataras/jev` requires Go 1.27,
  which matches this module; the `ax-go` Go floor is confirmed in task 1.
- Issues: #1 is rewritten and #11 is closed as decided. Both are GitHub edits
  that need the owner's approval and are not part of this change's code.
- Later work builds on this: #2 (`Probability`, sealed `Result`), #3 (spec
  validation) and #4 (`ask` and `score` commands).
- Non-goals: request and response types, a wrapper over `kataras/jev`'s
  answer types, batching, pseudonymization, CLI commands.
