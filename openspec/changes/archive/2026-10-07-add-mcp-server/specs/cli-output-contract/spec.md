# Spec Delta

## MODIFIED Requirements

### Requirement: Output shapes are discoverable through __schema

The system SHALL expose the command tree, including flags and the output schema
version of each command, through the `__schema` command, and SHALL mark the
output fields that vary between identical calls as non-deterministic.

#### Scenario: Schema lists both commands

- **WHEN** `__schema` is run
- **THEN** it lists `ask` and `score` with their flags and output version

#### Scenario: Schema lists eval

- **WHEN** `__schema` is run
- **THEN** it also lists `eval` with its flags and output version

#### Scenario: Schema lists the MCP server

- **WHEN** `__schema` is run at schema version 5
- **THEN** it also lists `mcp-server`, and golden files for version 5 exist
  for every output with the version 4 files unchanged
