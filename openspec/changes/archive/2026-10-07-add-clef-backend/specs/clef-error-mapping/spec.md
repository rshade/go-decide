# Spec Delta

## Purpose

Defines how failures from Cloudflare's clef endpoint and the network are
reported to callers and agents: one typed error shape with a stable `clef.*`
code and retry hints, so callers can decide what to do without parsing text.

## ADDED Requirements

### Requirement: Cloudflare failures become typed errors with a stable code

The system SHALL report every failed clef call as a typed error that carries a
stable machine-readable `clef.*` code, the HTTP status, the Cloudflare error
code and message from the first `errors` entry when present, and the
`cf-ai-req-id` value when present.

#### Scenario: Invalid request

- **WHEN** the server answers 400 with `errors: [{"code": 5006, "message": "..."}]`
- **THEN** the error has the invalid-request code, the status, Cloudflare code
  5006 and its message, and is not retryable

#### Scenario: Failure inside a 200

- **WHEN** the server answers 200 with `success: false` and an error entry
- **THEN** the error is typed from that entry, not reported as a success

### Requirement: Each class of Cloudflare failure has its own code

The mapping SHALL cover 401 and 403 (unauthorized), 400 and 422 (invalid
request), 429 (rate limited), timeouts, connection failures and a response
that breaks the contract, and SHALL give any other non-2xx status a distinct
generic code. The exit code of each class SHALL match its Jev counterpart.

#### Scenario: Unauthorized

- **WHEN** the server answers 401 or 403
- **THEN** the error has the unauthorized code, the status, and is not retryable

#### Scenario: Rate limited

- **WHEN** the server answers 429
- **THEN** the error has the rate-limited code and is retryable

#### Scenario: Unexpected status

- **WHEN** the server answers 418
- **THEN** the error has the generic status code and status 418

### Requirement: Secrets are redacted and codes are documented

The token SHALL be replaced with `[REDACTED]` in the message and every string
of the error context, including when the server echoes it. Every code the
system can emit SHALL appear in `docs/clef-errors.md`.

#### Scenario: Server echoes the token

- **WHEN** an error body contains the token
- **THEN** the error message and context contain `[REDACTED]` and no token

#### Scenario: Undocumented code

- **WHEN** a code is added without a row in `docs/clef-errors.md`
- **THEN** the documentation test fails and names the code
