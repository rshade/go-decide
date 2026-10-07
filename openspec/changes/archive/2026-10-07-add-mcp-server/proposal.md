# Proposal

## Why

Agents can only reach `go-decide` by shelling out and parsing exit codes.
`ax-go` already ships an `mcp-server` subcommand that turns a command tree
into MCP tools with no per-tool code, so an agent, or the `decide` skill's
pre-screen (#7), can call `ask` and `score` directly.

## What Changes

- Mount `ax-go`'s `mcp-server` in the root command, so `go-decide mcp-server`
  serves `ask` and `score` over stdio, or over HTTP bound to loopback.
- Keep `eval` out of the tools: one call fans out into one paid request per
  decision.
- Make an `uncertain` or `escalate` outcome a normal tool result whose payload
  carries `outcome`, never an error, and keep the process exit code 0 after a
  session that produced one.
- Add `mcp-server` to the `__schema` output, which bumps `SchemaVersion` from
  4 to 5, with new golden files and the earlier ones untouched.
- Document how to run it, a client configuration example, and the spend
  warning, in `docs/jev-decide-cli.md` and the README.

## Capabilities

### New Capabilities

- `mcp-server`: how `go-decide` is served as MCP tools: which commands are
  tools, how outcomes and failures are reported, what is kept out of tool
  arguments and results, and how the transports are restricted.

### Modified Capabilities

- `cli-output-contract`: `__schema` lists `mcp-server` and the output schema
  version moves to 5.

## Impact

- `internal/cli/cli.go` (the mount and the outcome handling),
  `internal/cli/outcome.go` or its equivalent, `internal/cli/output.go`
  (`SchemaVersion`), `internal/cli/deps_test.go`, the golden files, `go.mod`
  and `go.sum`.
- A new module dependency: the MCP Go SDK, pulled in by `ax-go/mcp`. It must
  stay out of `decision`, `jevclient`, `clefclient` and `eval`.
- Both backends are reachable through the tools, so the Jev and Cloudflare
  credentials and spend rules apply to each.
