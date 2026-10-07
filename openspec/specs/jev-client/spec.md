# jev-client Specification

## Purpose

Defines how a Jev client is constructed from the environment, so that every
caller gets the same safe defaults for credentials, endpoint and retry spend
without writing their own configuration.

## Requirements

### Requirement: Credential comes only from the environment

The client SHALL read its bearer token from `TYPESAFE_API_KEY` and SHALL NOT
accept a token from any file in the repository. Construction SHALL fail with a
configuration error when the variable is empty or unset, before any network
call is made. The token SHALL NOT appear in log output.

#### Scenario: Token missing

- **WHEN** a client is constructed with `TYPESAFE_API_KEY` unset
- **THEN** construction returns a configuration error and no HTTP request is
  sent

#### Scenario: Token never appears in logs

- **WHEN** a call is logged at any level and the server echoes the token in a
  response body
- **THEN** the log output contains no token value

### Requirement: Endpoint is restricted to TypeSafe or an explicit safe base URL

The client SHALL send requests to `https://api.typesafe.ai` unless
`TYPESAFE_BASE_URL` is set. A base URL SHALL be accepted only when its scheme
is `https`, or when its scheme is `http` and its host is a loopback IP address.
A host name such as `localhost` does not qualify. The client SHALL NOT follow
redirects, so the bearer token cannot be sent to another host.

#### Scenario: Default endpoint

- **WHEN** a client is constructed with no `TYPESAFE_BASE_URL`
- **THEN** requests go to `https://api.typesafe.ai/v1/systemone`

#### Scenario: Plain http to a remote host is refused

- **WHEN** `TYPESAFE_BASE_URL` is `http://example.com`
- **THEN** construction returns a configuration error and no HTTP request is
  sent

#### Scenario: Plain http to loopback is allowed

- **WHEN** `TYPESAFE_BASE_URL` is `http://127.0.0.1:8080`
- **THEN** construction succeeds and requests go to that address

#### Scenario: Loopback host name is refused

- **WHEN** `TYPESAFE_BASE_URL` is `http://localhost:8080`
- **THEN** construction returns a configuration error and no HTTP request is
  sent

#### Scenario: Redirect is not followed

- **WHEN** the server answers with a 3xx redirect to another host
- **THEN** the client returns an error and sends no request to the redirect
  target

### Requirement: Retry policy is explicit and bounded

The client SHALL apply a retry policy that is set by this project and not
inherited silently from a dependency default. The policy SHALL retry only
statuses that mean the request was not processed (429 and 529), SHALL honor a
server `Retry-After` up to a fixed ceiling that does not exceed the total time
bound, and SHALL bound total retries and total elapsed time for one call.

#### Scenario: Rate-limited call is retried within bounds

- **WHEN** the server answers 429 with a short `Retry-After` and then 200
- **THEN** the call succeeds after waiting, and the number of attempts is within
  the configured limit

#### Scenario: Retries are exhausted

- **WHEN** the server keeps answering 529 past the retry limit
- **THEN** the call returns an error that reports the final status and that the
  failure is retryable

#### Scenario: Retry-After above the ceiling is not honored

- **WHEN** the server answers 429 with a `Retry-After` longer than the ceiling
- **THEN** the client backs off on its own schedule instead of waiting that long

#### Scenario: Time budget stops retries

- **WHEN** waiting for the next retry would exceed the total time bound
- **THEN** the client stops and returns the last error without another attempt

#### Scenario: Non-retryable status is not retried

- **WHEN** the server answers 401 or 422
- **THEN** the client makes exactly one attempt

### Requirement: A missing answer is an error, never a zero value

The client SHALL return an error, and no partial result, when a successful
response omits a requested answer, returns an answer of a different type than
the question, or returns a probability or confidence outside 0 to 1.

#### Scenario: Requested answer absent

- **WHEN** a request asks for question `safe` and the 200 response has no `safe`
  answer
- **THEN** the call returns an error and no answer value

#### Scenario: Out-of-range probability

- **WHEN** a `noul` answer of 1.4 is returned
- **THEN** the call returns an error and no answer value

#### Scenario: Out-of-range confidence

- **WHEN** a `choice` or `score` answer reports a confidence of 1.2
- **THEN** the call returns an error and no answer value

### Requirement: Adapter tests make no live API call by default

The tests for the client package SHALL NOT contact `api.typesafe.ai` and
SHALL NOT read `.env`. A test that needs the live API SHALL be skipped unless
`TYPESAFE_API_KEY` is set in the environment. Live probes elsewhere in the
repository are separate and may read `.env` by design.

#### Scenario: Adapter tests offline

- **WHEN** the client package tests run with no network access and no
  `TYPESAFE_API_KEY` in the environment
- **THEN** all tests pass or are skipped, and none fail from a network error

### Requirement: Responses can be cached on disk when asked

The client SHALL offer an opt-in response cache in a directory the caller
names, and SHALL write nothing to disk unless the cache is turned on. With the
cache on, a request whose method, scheme, host, path and body match a cached
entry SHALL be answered from the cache without a network call.

#### Scenario: Hit sends nothing

- **WHEN** the same request is made twice with the cache on
- **THEN** the server receives one request and both calls return the same
  answer

#### Scenario: Cache off by default

- **WHEN** a client is constructed without the cache option and makes a call
- **THEN** no file is written

### Requirement: Only complete successful responses are cached

Only successful responses that answer every question asked SHALL be stored.
Errors, retried statuses, redirects and a response missing an answer SHALL NOT
be stored.

#### Scenario: Failure is not cached

- **WHEN** the server returns 500 and then succeeds for the same request
- **THEN** the second call reaches the server and its response is the one
  cached

#### Scenario: Missing answer is not cached

- **WHEN** the server returns 200 with an answer to a different question
- **THEN** nothing is cached and the same request reaches the server again

### Requirement: Cache entries are private and never hold the credential

The cache key and the stored entry SHALL NOT contain the credential or any
request header. The directory SHALL be created readable only by its owner, and
entries SHALL be written readable only by their owner.

#### Scenario: Credential stays out of the cache

- **WHEN** a response is cached
- **THEN** no file in the cache directory contains the token

### Requirement: A corrupt cache entry is a miss

An entry that cannot be read or parsed SHALL be treated as a miss and
replaced, never returned.

#### Scenario: Corrupt entry

- **WHEN** a cached entry has been truncated
- **THEN** the request reaches the server and the entry is replaced

### Requirement: The cache does not change the client's other guarantees

The client's other guarantees, including the endpoint restriction, the refusal
to follow redirects and the retry policy, SHALL be the same with the cache on.

#### Scenario: Redirect still refused with the cache on

- **WHEN** the cache is on and the server answers with a redirect
- **THEN** the client returns an error, follows nothing and stores nothing
