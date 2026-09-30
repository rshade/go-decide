# Design

## Context

See proposal.md for motivation. Observed state:

- `kataras/jev` v0.1.0 (one commit, MIT, about 4k lines) is used only by the
  throwaway probe tests. It decodes and validates 2xx responses itself
  (`ErrResponse` for missing answers, type mismatches, values outside 0..1,
  choices not in the options, non-finite scores), refuses redirects, stamps
  `RequestID` on errors, and exposes `APIError{Status, Header, Body, Message,
  RequestID}` with `errors.Is` sentinels and `RetryAfter()`.
- Its defaults are not ours. `DefaultRetryPolicy()` retries 408, 429 and all
  5xx, plus connection failures and timeouts, twice. Its base URL check
  accepts `http` for any host. Its `extractMessage` reads `detail.message`
  but discards `detail.error_type`.
- The OpenAPI snapshot documents only 200 and 422, and 422 uses a list
  `detail`. The object form `{"detail": {"error_type", "message"}}` for
  401, 429 and 529 comes from the vendor docs. Only 401 has been seen live;
  429 and 529 fixtures will be synthetic.
- `ax-go` v0.7.0 requires Go 1.27.1. This module declares `go 1.27`, so
  `go get` raises the directive. `ax-go`'s `contract` package is
  import-isolated and provides `Error` with retryable and retry-after
  fields, so the adapter does not need the Cobra and telemetry runtime.
- The repository root holds `probe_*_test.go` and `jev_*_test.go` in one
  package. Production code goes in a subpackage so it never shares a package
  with the probes.

## Goals / Non-Goals

**Goals:**

- One constructor that gives every caller the same credential, endpoint and
  retry policy, and one classifier that gives every failure the same shape.
- Keep our own code small enough that replacing `kataras/jev` later is a
  one-package change.

**Non-Goals:**

- Our own request or answer types. Callers use `kataras/jev`'s `Request`,
  `Questions`, `Noul`, `Choice`, `Score` and answer types directly. Sealed
  results and `Probability` are #2.
- Wrapping the dependency's `SystemOne` call. Callers get the configured
  `*jev.Client` and call it themselves.
- CLI commands, JSON output schemas, batching (later issues).

## Decisions

**1. Adopt `kataras/jev`, do not write a client.** It matches the user's
direction to not maintain a client, and it is the only client live-tested
in the probes. `anilsenay/jev` scores the same on the survey but is
untested here. Alternatives: own client (rejected, duplicates validated
decoding), `anilsenay/jev` (untested), keep it behind our own interface
(rejected for now: an interface over a moving `v0.x` API is more code to
maintain than the one package that imports it).

**2. Adapter package `jevclient/` exports `NewClient` and `Classify`,** plus
`WithTimeout` (the per-attempt timeout, also what the timeout tests need) and
the nine `Code*` constants.
`NewClient(opts ...Option) (*jev.Client, error)` reads the environment,
validates the base URL, sets the retry policy, then delegates to
`jev.New`. `Classify(ctx, err) error` turns a failure into a
`contract.Error`. Alternative: a wrapper type with its own `Ask` method
(rejected: it would re-export the dependency's types and force us to track
its API).

**3. Base URL policy is ours.** Parse `TYPESAFE_BASE_URL` before calling
`jev.New` and require `https`, or `http` with a loopback IP address (so
`httptest` works). The name `localhost` is refused because a hosts file can
point it elsewhere. Redirect refusal already comes from the dependency, and
a test pins it so an upstream change cannot silently remove it.

**4. Retry policy: 429 and 529 only.** Start from `jev.DefaultRetryPolicy()`
and replace `Statuses` with 429 and 529, keep `RespectRetryAfter`, set
`MaxRetries` to 2, `MaxRetryAfter` to 10 seconds and `MaxElapsed` to 30
seconds, so the ceiling always sits inside the budget. Tests assert that
invariant and that the budget stops a further attempt. These
statuses mean the request was not processed, so a retry should not double
bill. That assumption is unconfirmed with the vendor; the retry list is a
single constant so it can be narrowed. Connection and timeout retries are
off by default: a request that timed out may have been processed and billed.
A 5xx other than 529 is reported, not retried. Alternative: keep the
dependency default (rejected: retries 408 and every 5xx and timeouts without
a decision about spend).

**5. `error_type` comes from parsing `APIError.Body` in `Classify`.** Try
`detail` as an object first, then as a list, then fall back to the dependency's
`Message`. Drop this parsing if upstream exposes the field; an upstream PR is
worth opening but is not a dependency of this change.

**6. Error codes are a closed set in one file.** For example
`jev.unauthorized`, `jev.invalid_request`, `jev.rate_limited`,
`jev.overloaded`, `jev.status`, `jev.timeout`, `jev.connection`,
`jev.invalid_response` (a 2xx body that breaks the contract). A request the
client rejects before sending maps to `jev.invalid_request`; anything else is
`jev.internal`. `retryable` is true for 429, 529, timeout and
connection failures. `retry_after_seconds` is `RetryAfter()` rounded up,
reported only for
retryable errors.
Context cancellation returns the context error unchanged.

**7. Token redaction is applied in `Classify`.** Replace any occurrence of the
key value in the message, body and context fields, since the dependency keeps
the response body in `APIError.Body`. The key is read from the environment
when `Classify` runs, which is documented on the function.

**9. Logs are redacted by our own logger.** The dependency redacts request
headers, but at debug level it logs response bodies as received, so a server
that echoes the key would leak it. `NewClient` installs a logger built like the
dependency's (`TYPESAFE_LOG_LEVEL`, stderr, same levels) that replaces the key
in every string attribute. An error returned by `SystemOne` and never passed to
`Classify` still holds the raw body, so `Classify` is the supported path.

**8. Tests use `httptest` only.** A table of status/body/header cases
covers `Classify`; separate tests cover base URL policy, redirect refusal,
retry bounds and the missing-answer path (a 200 with an answer omitted). The
live check in the client package skips unless `TYPESAFE_API_KEY` is in the
environment. The root probe tests are live by design and also read `.env`.

## Risks / Trade-offs

- **Dependency is one commit, 5 stars, `v0.1.0`** → pin the exact version,
  keep our surface to two functions, record the risk in `docs/`, and review the
  diff on every bump.
- **429 and 529 shapes never observed live** → tests use documented shapes;
  first real occurrence should be captured into a fixture. `Classify` falls
  back to the body text when the shape is unexpected.
- **Retrying 429 and 529 may still bill** → unconfirmed; the retry list is one
  constant and `MaxRetries` is an option.
- **`ax-go` is pre-1.0 and raises the Go directive to 1.27.1** → pin the
  version and use only `contract`, the smallest surface.
- **A raw error that skips `Classify` may hold an echoed key** → document
  `Classify` as the supported path; the redacting logger covers logs either way.
- **The key changes between `NewClient` and `Classify`** → documented on
  `Classify`; both read the same variable in normal use.
- **Anyone can call `jev.New` directly and bypass our policy** → document
  `NewClient` as the only supported constructor; enforcement of that in the
  CLI is #4.

## Open Questions

- Whether the vendor bills a 429 or 529. Safe to answer later; it only changes
  the retry constant.
- Whether to open an upstream PR for `error_type`. Does not change this change.
