# Spec Delta

## ADDED Requirements

### Requirement: Responses can be cached on disk when asked

The client SHALL offer an opt-in response cache in a directory the caller
names, and SHALL write nothing to disk unless the cache is turned on. With the
cache on, a request whose method, scheme, host, path and body match a cached
entry SHALL be answered from the cache without a network call. Only successful
responses that answer every question asked SHALL be stored. Errors, retried
statuses, redirects and a response missing an answer SHALL NOT be stored. The
cache key and the stored entry SHALL NOT contain the credential or any request
header. The directory SHALL be created readable only by its owner, and entries
SHALL be written readable only by their owner. An entry that cannot be read or
parsed SHALL be treated as a miss and replaced, never returned. The client's
other guarantees, including the endpoint restriction, the refusal to follow
redirects and the retry policy, SHALL be the same with the cache on.

#### Scenario: Hit sends nothing

- **WHEN** the same request is made twice with the cache on
- **THEN** the server receives one request and both calls return the same
  answer

#### Scenario: Failure is not cached

- **WHEN** the server returns 500 and then succeeds for the same request
- **THEN** the second call reaches the server and its response is the one
  cached

#### Scenario: Missing answer is not cached

- **WHEN** the server returns 200 with an answer to a different question
- **THEN** nothing is cached and the same request reaches the server again

#### Scenario: Credential stays out of the cache

- **WHEN** a response is cached
- **THEN** no file in the cache directory contains the token

#### Scenario: Corrupt entry

- **WHEN** a cached entry has been truncated
- **THEN** the request reaches the server and the entry is replaced

#### Scenario: Cache off by default

- **WHEN** a client is constructed without the cache option and makes a call
- **THEN** no file is written
