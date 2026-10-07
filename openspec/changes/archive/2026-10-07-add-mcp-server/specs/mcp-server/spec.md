# Spec Delta

## Purpose

Defines how go-decide is served to agents as Model Context Protocol tools, so
a caller can ask and score decisions without a shell, and without any change
to what an outcome means or what a call costs.

## ADDED Requirements

### Requirement: ask and score are served as tools

The `mcp-server` command SHALL serve `ask` and `score` as MCP tools named
`go-decide-ask` and `go-decide-score`, with the same flags as inputs,
including `--backend` and `--dry-run`. It SHALL serve over stdio by default.

#### Scenario: Tools are listed

- **WHEN** a client lists the tools of `go-decide mcp-server` over its default
  stdio transport
- **THEN** `go-decide-ask` and `go-decide-score` are listed with their
  flags as inputs

#### Scenario: Backend is an input

- **WHEN** a client calls `go-decide-ask` with `backend` set to `clef`
- **THEN** the call uses the clef backend and the result names `clef`

### Requirement: eval is not a tool

The server SHALL NOT expose `eval`, because one call sends one paid request
per decision. `eval` SHALL stay available on the command line.

#### Scenario: eval is absent

- **WHEN** a client lists the tools
- **THEN** `eval` is not listed, and calling it fails as an unknown tool

#### Scenario: eval still works on the command line

- **WHEN** `go-decide eval --dry-run` runs
- **THEN** it validates and exits 0 as before

### Requirement: Outcomes are results, not errors

A call that ends in `decided`, `uncertain` or `escalate` SHALL return the
shared success envelope with its `outcome`, and SHALL NOT be marked as an
error. The process exit code SHALL stay 0 after such a call.

#### Scenario: Uncertain is a result

- **WHEN** `ask` produces an uncertain outcome over MCP
- **THEN** the call is not flagged as an error and its payload has
  `outcome: uncertain`

#### Scenario: Escalate is a result

- **WHEN** `ask` produces an escalate outcome over MCP
- **THEN** the call is not flagged as an error and its payload has
  `outcome: escalate`

#### Scenario: Shutdown exit code

- **WHEN** a session that returned an uncertain outcome ends normally
- **THEN** the process exits 0, not 10

### Requirement: Failures are errors and cost nothing before validation

A failed call SHALL return the shared error envelope flagged as an error. An
invalid input SHALL fail before any request is sent to a backend. `--dry-run`
SHALL validate and return without a request.

#### Scenario: Validation failure

- **WHEN** `ask` is called with an empty `state`
- **THEN** the call is flagged as an error, the envelope carries the
  `validation_error` code, and no request reached a backend

#### Scenario: Dry run

- **WHEN** `ask` is called with `dry-run` set
- **THEN** the result is the dry-run envelope and no request is sent

### Requirement: Credentials never travel in tool calls

The server SHALL read credentials only from its environment, as the command
line does, and SHALL use only the selected backend's credentials with no
fallback. No tool input SHALL accept a credential, and no tool result, error
or log line SHALL contain one.

#### Scenario: Missing credentials

- **WHEN** `ask` is called for a backend whose credentials are not set
- **THEN** the call fails with that backend's configuration error and no
  request is sent to either backend

#### Scenario: Secrets are redacted

- **WHEN** a Jev or clef error response echoes the credential
- **THEN** the tool result shows `[REDACTED]`, and neither the result nor the
  server log contains the value

### Requirement: The HTTP transport stays on loopback

The server SHALL bind HTTP to a loopback address by default, and SHALL refuse
a non-loopback address with exit 2 unless it is explicitly allowed.

#### Scenario: Public bind refused

- **WHEN** `go-decide mcp-server` is started for HTTP on a non-loopback
  address without the allow flag
- **THEN** it exits 2 before listening

### Requirement: The server reports its build version

The server SHALL report the same version as `--version`, and SHALL NOT report
`dev` or `unknown`.

#### Scenario: Version in the handshake

- **WHEN** a client connects
- **THEN** the server info carries the version that `--version` prints
