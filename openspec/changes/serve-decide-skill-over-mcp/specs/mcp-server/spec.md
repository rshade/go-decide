# Spec Delta

## ADDED Requirements

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
