# Spec Delta

## ADDED Requirements

### Requirement: The decision log is served as tools

The `mcp-server` command SHALL serve `record decision`, `record run`,
`record outcome` and `record pending` as the MCP tools
`go-decide-record-decision`, `go-decide-record-run`,
`go-decide-record-outcome` and `go-decide-record-pending`, with their flags as
inputs. A record call SHALL make no backend request and need no credential.

#### Scenario: Record tools are listed

- **WHEN** a client lists the tools of `go-decide mcp-server`
- **THEN** the four `go-decide-record-*` tools are listed beside
  `go-decide-ask` and `go-decide-score`

#### Scenario: Recording costs nothing

- **WHEN** a client calls `go-decide-record-decision` with no credentials set
- **THEN** the call succeeds or fails on its own input, and no request reaches
  a backend

## MODIFIED Requirements

### Requirement: Tool calls never read files

A tool call SHALL NOT make the server read a file or its standard input that
the call names. A `spec` input SHALL be refused with a validation error that
names the field, before any request is sent, and the error SHALL NOT echo any
file content. Decisions are given inline. The only file a tool call may read
or write is the decision log named by the server process's `DECIDE_LOG`, and
only through a `go-decide-record-*` tool.

#### Scenario: Spec file refused

- **WHEN** a client calls `go-decide-ask` with `spec` set to a readable file
- **THEN** the call is flagged as an error with `validation_error` and the
  `spec` field, no request is sent, and the file content is not returned

#### Scenario: Command line unaffected

- **WHEN** `go-decide ask --spec decision.json` runs on the command line
- **THEN** the spec is read and the question is asked as before

#### Scenario: Record tools use only the server's log

- **WHEN** a client calls `go-decide-record-pending`
- **THEN** the server reads only the file named by its own `DECIDE_LOG`, or
  no file when it is unset

### Requirement: The decide skill is served as resources

The server SHALL serve each file of the `decide` skill as an MCP resource of
type `text/markdown`, at a URI that mirrors its repository path under
`go-decide://skills/decide/`. Reading a resource SHALL return the file as
shipped in the build, byte for byte, without a backend request.

#### Scenario: Resources are listed

- **WHEN** a client lists the resources of `go-decide mcp-server`
- **THEN** `go-decide://skills/decide/SKILL.md`,
  `go-decide://skills/decide/references/debate-prompts.md` and
  `go-decide://skills/decide/references/decision-log.md` are listed with type
  `text/markdown`

#### Scenario: Resource content matches the repository file

- **WHEN** a client reads `go-decide://skills/decide/SKILL.md`
- **THEN** the content equals `skills/decide/SKILL.md` byte for byte, and no
  request reaches a backend

#### Scenario: Relative references resolve

- **WHEN** the skill names `references/debate-prompts.md` or
  `references/decision-log.md`
- **THEN** that path resolved against `go-decide://skills/decide/SKILL.md` is
  a listed resource

#### Scenario: Unknown resource

- **WHEN** a client reads a `go-decide://` URI that is not listed
- **THEN** the read fails with a protocol error
