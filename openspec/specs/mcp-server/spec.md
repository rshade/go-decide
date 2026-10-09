# mcp-server Specification

## Purpose

Defines how go-decide is served to agents as Model Context Protocol tools, so
a caller can ask and score decisions without a shell, and without any change
to what an outcome means or what a call costs.

## Requirements

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

### Requirement: Tool calls never read files

A tool call SHALL NOT make the server read a file or its standard input. A
`spec` input SHALL be refused with a validation error that names the field,
before any request is sent, and the error SHALL NOT echo any file content.
Decisions are given inline.

#### Scenario: Spec file refused

- **WHEN** a client calls `go-decide-ask` with `spec` set to a readable file
- **THEN** the call is flagged as an error with `validation_error` and the
  `spec` field, no request is sent, and the file content is not returned

#### Scenario: Command line unaffected

- **WHEN** `go-decide ask --spec decision.json` runs on the command line
- **THEN** the spec is read and the question is asked as before

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

### Requirement: The decide skill is served as resources

The server SHALL serve each file of the `decide` skill as an MCP resource of
type `text/markdown`, at a URI that mirrors its repository path under
`go-decide://skills/decide/`. Reading a resource SHALL return the file as
shipped in the build, byte for byte, without a backend request.

#### Scenario: Resources are listed

- **WHEN** a client lists the resources of `go-decide mcp-server`
- **THEN** `go-decide://skills/decide/SKILL.md` and
  `go-decide://skills/decide/references/debate-prompts.md` are listed with
  type `text/markdown`

#### Scenario: Resource content matches the repository file

- **WHEN** a client reads `go-decide://skills/decide/SKILL.md`
- **THEN** the content equals `skills/decide/SKILL.md` byte for byte, and no
  request reaches a backend

#### Scenario: Relative references resolve

- **WHEN** the skill names `references/debate-prompts.md`
- **THEN** that path resolved against `go-decide://skills/decide/SKILL.md` is
  a listed resource

#### Scenario: Unknown resource

- **WHEN** a client reads a `go-decide://` URI that is not listed
- **THEN** the read fails with a protocol error

### Requirement: The server sends instructions at initialize

The server SHALL send short instructions in its `initialize` result, built
from the skill's own frontmatter `description`, that name the skill resource
and say every `ask` and `score` call is a paid request whose decided outcome
is not approval. A skill without a name or description SHALL fail startup.

#### Scenario: Instructions in the handshake

- **WHEN** a client connects
- **THEN** the `initialize` result carries instructions that contain the
  skill's `description`, `go-decide://skills/decide/SKILL.md`, and say a call
  is a paid request

#### Scenario: Broken frontmatter

- **WHEN** the embedded `SKILL.md` has no frontmatter, or one without a
  `name` or a `description`
- **THEN** the command fails with an internal error instead of serving
  without instructions

### Requirement: A decide prompt is the manual entry point

The server SHALL serve a prompt named `decide` with one required argument,
`decision`. Rendering it SHALL return one user message that contains the
decision and points at the skill resource, and SHALL NOT contain the skill
text itself.

#### Scenario: Prompt renders

- **WHEN** a client gets the `decide` prompt with `decision` set
- **THEN** the result is one user message that contains the decision and
  `go-decide://skills/decide/SKILL.md`

#### Scenario: Missing decision

- **WHEN** a client gets the `decide` prompt without `decision`
- **THEN** the call fails with an invalid-params error and nothing is
  rendered

### Requirement: Serving the skill costs nothing

Listing or reading resources, getting the prompt, and connecting SHALL NOT
send a request to any backend, and SHALL NOT need backend credentials.

#### Scenario: No credentials

- **WHEN** the server runs with no Jev or Cloudflare credentials set
- **THEN** a client can list and read the resources and get the prompt, and
  no request reaches a backend
