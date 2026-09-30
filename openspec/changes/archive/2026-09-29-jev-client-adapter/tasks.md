# Tasks

## 1. Dependencies and package skeleton

- [x] 1.1 Add `github.com/kataras/jev` at an exact pinned version and
      `github.com/rshade/ax-go` at a pinned version (import only its `contract`
      package); verify `go mod tidy` leaves both in `go.mod` and the `go`
      directive is 1.27.1 or higher as `ax-go` requires
- [x] 1.2 Create the `jevclient/` package with a package doc comment and empty
      `NewClient` and `Classify` signatures from design decision 2; verify `go
      build ./...` and `go vet ./...` pass and the root probe tests still
      compile

## 2. Client construction (spec: jev-client)

- [x] 2.1 Implement base URL validation (https, or http to loopback, no
      credentials, query or fragment) with a table test covering the default, a
      remote `http` URL, loopback `http`, and a malformed URL; verify `go test
      ./jevclient -run BaseURL` passes
- [x] 2.2 Implement `NewClient` reading `TYPESAFE_API_KEY` and
      `TYPESAFE_BASE_URL`, failing before any request when the key is empty;
      verify a test with an `httptest` server asserts no request arrives on a
      missing key and that a bearer header arrives on success
- [x] 2.3 Set the explicit retry policy (429 and 529 only, honor `Retry-After`
      to a ceiling, bounded retries and elapsed time, no connection or timeout
      retry) in one constant block; verify tests for 429 then 200, 529 past the
      limit, and a single attempt on 401 and 422
- [x] 2.4 Pin redirect refusal and missing-answer behavior with tests against
      the dependency: a 302 to a second `httptest` server receives no request,
      and a 200 that omits a requested answer, returns a mismatched type, or
      returns `noul` 1.4 yields an error and no value; verify `go test
      ./jevclient` passes offline
- [x] 2.5 Document `NewClient`, its environment variables and its retry and
      endpoint policy in the package doc and `README.md`; verify the documented
      example compiles as an `Example` test and markdownlint passes

## 3. Error classification (spec: jev-error-mapping)

- [x] 3.1 Define the closed set of error codes and the status-to-code table for
      401, 422, 429, 529 and other statuses; verify a table test asserts code,
      status and retryable for each
- [x] 3.2 Parse `APIError.Body` for object-form and list-form `detail` and
      non-JSON bodies, falling back to the dependency's message; verify table
      tests for `{"detail":{"error_type","message"}}`, a 422 field list, and an
      HTML 502
- [x] 3.3 Map request ID, `RetryAfter()` (rounded up to whole seconds), timeouts
      and connection failures, and pass context cancellation through unchanged
      and not retryable; verify tests for `Retry-After: 3`, no retry header, a
      server slower than the timeout, and a canceled context
- [x] 3.4 Redact the API key from message, body and context fields of every
      error; verify a test where the server echoes the bearer token in its error
      body and the resulting error contains no occurrence of it
- [x] 3.5 Document the error codes and the `ax.Error` fields they set in `docs/`
      with one example per code; verify markdownlint passes and each documented
      code appears in the code table

## 4. Records, live probe and issue housekeeping

- [x] 4.1 Record the adoption decision, the dependency risk (one commit, five
      stars, `v0.1.0`) and the unconfirmed 429/529 billing assumption in
      `docs/jev-clients.md`, and update `ROADMAP.md`, `CONTEXT.md` and
      `CLAUDE.md` to say `kataras/jev` is adopted and #11 is decided; verify
      markdownlint passes on every edited file
- [x] 4.2 Add one live check that runs `NewClient` and a single `noul` call
      when `TYPESAFE_API_KEY` is set in the environment; verify it reports SKIP
      with the variable unset and passes against the real API with it set
- [x] 4.3 Run format, vet, lint, the full test suite and
      `openspec validate jev-client-adapter --strict`; verify all exit zero
- [x] 4.4 With the owner's approval, rewrite the requirements of issue #1 to
      match this scope and close #11 as decided; verify both on GitHub. These
      are outward-facing edits and must not run without a direct yes

## 5. Verification follow-ups

- [x] 5.1 Set `MaxRetryAfter` explicitly to 10 seconds, inside the 30 second
      budget; verify `TestRetryPolicyIsExplicitAndBounded`,
      `TestRetryAfterAboveCeilingFallsBackToBackoff` and
      `TestRetryBudgetStopsFurtherRetries` pass
- [x] 5.2 Install a logger that redacts the API key and honors
      `TYPESAFE_LOG_LEVEL`; verify `TestLogsNeverContainTheAPIKey`,
      `TestNoLogsUnlessLevelIsSet` and `TestInvalidLogLevelIsAConfigurationError`
- [x] 5.3 Refuse `localhost` by name and accept only loopback IP addresses;
      verify `TestValidateBaseURL`
- [x] 5.4 Cover confidence outside 0 to 1 for `choice` and `score`, and a retry
      header on a non-retryable status; verify
      `TestConfidenceOutsideZeroToOneIsRejected` and
      `TestClassifyOmitsRetryAfterWhenNotRetryable`
- [x] 5.5 Bring specs, design, README and docs in line with the code; verify
      `openspec validate jev-client-adapter --strict` and markdownlint pass
- [x] 5.6 Fix the eight lint findings in the probe tests; verify
      `golangci-lint run ./...` reports 0 issues
