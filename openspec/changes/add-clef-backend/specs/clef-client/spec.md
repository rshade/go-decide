# Spec Delta

## Purpose

Defines how a Cloudflare clef client is constructed from the environment, so
that every caller gets the same safe defaults for credentials, endpoint and
retry spend as the Jev client without writing their own configuration.

## ADDED Requirements

### Requirement: Credentials come only from the environment

The client SHALL read its bearer token from `CLOUDFLARE_AUTH_TOKEN` and its
account from `CLOUDFLARE_ACCOUNT_ID`, and SHALL NOT accept either from a file
in the repository. Construction SHALL fail with a configuration error, before
any network call, when either is empty or unset, or when the account ID is not
32 hexadecimal characters. The token SHALL NOT appear in log output or in any
error text.

#### Scenario: Token missing

- **WHEN** a client is constructed with `CLOUDFLARE_AUTH_TOKEN` unset
- **THEN** construction returns a configuration error that names the variable
  and no HTTP request is sent

#### Scenario: Account missing

- **WHEN** a client is constructed with `CLOUDFLARE_ACCOUNT_ID` unset
- **THEN** construction returns a configuration error that names the variable
  and no HTTP request is sent

#### Scenario: Account ID cannot reshape the path

- **WHEN** `CLOUDFLARE_ACCOUNT_ID` contains a slash, a dot or a query
- **THEN** construction returns a configuration error and no HTTP request is
  sent

#### Scenario: Token never appears in logs

- **WHEN** a call is logged at any level and the server echoes the token in a
  response body
- **THEN** the log output contains no token value

### Requirement: Endpoint is restricted to Cloudflare or an explicit safe base URL

The client SHALL send each request as a POST to
`https://api.cloudflare.com/client/v4/accounts/{account_id}/ai/run/@cf/cloudflare/clef`
unless `CLOUDFLARE_BASE_URL` is set, in which case the same path follows that
base. A base URL SHALL be accepted only when its scheme is `https`, or `http`
to a loopback IP address, and SHALL NOT carry credentials, a query or a
fragment. The client SHALL NOT follow redirects. Every request SHALL carry the
model name `clef`.

#### Scenario: Default endpoint

- **WHEN** a client is constructed with no `CLOUDFLARE_BASE_URL`
- **THEN** requests go to the Cloudflare account path above

#### Scenario: Plain http to a remote host is refused

- **WHEN** `CLOUDFLARE_BASE_URL` is `http://example.com`
- **THEN** construction returns a configuration error and no HTTP request is
  sent

#### Scenario: Redirect is not followed

- **WHEN** the server answers with a 3xx redirect to another host
- **THEN** the client returns an error and sends no request to the redirect
  target

#### Scenario: Model is always sent

- **WHEN** a question set is sent
- **THEN** the request body carries `"model": "clef"`, `state` and `questions`

### Requirement: The Cloudflare envelope is unwrapped and validated

The client SHALL treat a response as successful only when the HTTP status is
200 and the envelope reports `success: true`, and SHALL then return the answers
from `result`. A successful response that omits a requested answer, answers
with a different type, or reports a probability or confidence outside 0 to 1
SHALL be an error and SHALL NOT become a zero value.

#### Scenario: Answers are read from the result

- **WHEN** the server answers 200 with `success: true` and answers under
  `result`
- **THEN** the caller receives the answers as if they had come from System One

#### Scenario: Success flag false

- **WHEN** the server answers 200 with `success: false`
- **THEN** the call fails with the envelope's error text, never an answer

#### Scenario: Missing answer

- **WHEN** a requested question is absent from `result.answers`
- **THEN** the call fails with an invalid-response error

### Requirement: The Cloudflare adaptation is reusable by another client

The package SHALL export the pieces that adapt Cloudflare to the System One
wire format, so a caller that already has its own System One client can reach
clef without this package's client: a base URL for an account, and an HTTP
transport that rewrites the System One path to the clef run path and unwraps
the result envelope.

#### Scenario: A plain HTTP client reaches clef

- **WHEN** a plain `net/http` client uses the exported transport and posts a
  System One request to the exported base URL plus `/v1/systemone`
- **THEN** the server receives the clef run path with the same body and
  Authorization header, and the client reads a flat System One response

#### Scenario: Recorded response

- **WHEN** a real recorded clef response with a noul, a choice and a score
  answer is served
- **THEN** all three answers read as typed System One answers

### Requirement: The exported adaptation is restricted and self-contained

The transport SHALL refuse any other path or method, and the base URL SHALL be
refused for an account ID that is not 32 hexadecimal characters. Importing the
package from another module SHALL need no more than the module's declared
dependencies.

#### Scenario: Other paths are refused

- **WHEN** a request to `/v1/models`, or a GET to `/v1/systemone`, goes
  through the transport
- **THEN** it fails and nothing is sent

#### Scenario: Bad account ID is refused

- **WHEN** the exported base URL is requested for an account ID that is not 32
  hexadecimal characters
- **THEN** it is refused

### Requirement: Retry policy is explicit and bounded

The client SHALL retry only HTTP 429, at most twice and within a 30 second
budget, honoring `Retry-After` up to 10 seconds. It SHALL NOT retry timeouts
or connection failures, because the request may already have been billed.

#### Scenario: Rate-limited call is retried within bounds

- **WHEN** the server answers 429 with a short `Retry-After` and then 200
- **THEN** the call succeeds and the attempts stay within the limit

#### Scenario: Timeout is not retried

- **WHEN** an attempt times out
- **THEN** the call fails after that attempt and no second request is sent
