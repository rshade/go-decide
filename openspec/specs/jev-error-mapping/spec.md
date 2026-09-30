# jev-error-mapping Specification

## Purpose

Defines how failures from the Jev API and the network are reported to
callers and agents: one typed error shape with a stable code, retry hints and
a request ID for support, so callers can decide what to do without parsing
message text.

## Requirements

### Requirement: API failures become typed errors with a stable code

The system SHALL report every failed call as a typed error that carries a
stable machine-readable code, the HTTP status, the server request ID from
`x-typesafe-request-id` when present, and a human-readable message. The
mapping SHALL cover 401, 422, 429 and 529, and SHALL give any other non-2xx
status a distinct generic code.

#### Scenario: Unauthorized

- **WHEN** the server answers 401
- **THEN** the error has an authentication code, status 401, the request ID, and
  is not retryable

#### Scenario: Validation failure

- **WHEN** the server answers 422 with a list of field errors
- **THEN** the error has a validation code, status 422, and a message that names
  the failing fields

#### Scenario: Overloaded

- **WHEN** the server answers 529
- **THEN** the error has an overloaded code, status 529, and is retryable

#### Scenario: Unexpected status

- **WHEN** the server answers 418
- **THEN** the error has the generic status code and status 418

### Requirement: Server error_type is preserved

When the response body has the shape `{"detail": {"error_type", "message"}}`,
the error SHALL expose `error_type` and `message` from it. When `detail` is
instead a list of validation entries, or the body is not JSON, the error SHALL
still be produced, with a message derived from the body and no `error_type`.

#### Scenario: Object detail

- **WHEN** a 429 body is `{"detail": {"error_type": "rate_limit", "message":
  "slow down"}}`
- **THEN** the error carries error type `rate_limit` and message `slow down`

#### Scenario: List detail

- **WHEN** a 422 body has `detail` as a list of `{loc, msg, type}` entries
- **THEN** the error carries no error type and its message lists each field and
  message

#### Scenario: Non-JSON body

- **WHEN** a 502 arrives with an HTML body
- **THEN** the error is still produced, with a truncated message and no error
  type

### Requirement: Retry hints are reported

The error SHALL report whether the call may be retried and, for a retryable
error, when the server sent `Retry-After` or `Retry-After-Ms`, the delay in
whole seconds rounded up.

#### Scenario: Retry-After present

- **WHEN** a 429 carries `Retry-After: 3`
- **THEN** the error is retryable and reports a retry delay of 3 seconds

#### Scenario: Retry-After absent

- **WHEN** a 529 carries no retry header
- **THEN** the error is retryable and reports no retry delay

#### Scenario: Non-retryable status carries a retry header

- **WHEN** a 401 carries `Retry-After: 5`
- **THEN** the error is not retryable and reports no retry delay

### Requirement: Contract violations and rejected requests have their own codes

The system SHALL report a successful response that breaks the API contract with
a code distinct from the API status codes, and SHALL report a request that the
client rejects before sending anything as an invalid-request error that costs no
API call.

#### Scenario: Malformed success response

- **WHEN** a 200 response omits a requested answer
- **THEN** the error has an invalid-response code and carries no retry hint

#### Scenario: Request rejected before sending

- **WHEN** a request with no questions is submitted
- **THEN** the error has the invalid-request code, is not retryable, and no
  HTTP request is sent

### Requirement: Transport failures are distinguished from API failures

The system SHALL report timeouts and connection failures with codes distinct
from API status codes, and SHALL report a caller's context cancellation as
that cancellation and not as a Jev failure.

#### Scenario: Per-attempt timeout

- **WHEN** the server does not answer within the per-attempt timeout
- **THEN** the error has a timeout code and is retryable

#### Scenario: Caller cancels

- **WHEN** the caller's context is canceled during a call
- **THEN** the returned error is the context cancellation and is not retryable

### Requirement: Errors never contain the credential

No error, message, context field or retained body SHALL contain the bearer
token, including when the server echoes request headers in an error body.

#### Scenario: Server echoes the authorization header

- **WHEN** an error body contains the bearer token
- **THEN** the token is redacted from every field of the resulting error
