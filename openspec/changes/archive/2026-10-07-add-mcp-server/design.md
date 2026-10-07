# Design

## Context

`ax-go` v0.7.0 ships `mcp.NewCommand(root, opts...)`, a reserved `mcp-server`
subcommand that discovers every visible, runnable command as a tool, runs a
`tools/call` back through the command tree in machine mode, and returns the
verbatim stdout on success or the error envelope with `IsError` on a non-zero
exit. The SDK it uses is confined behind `ax-go/mcp`. `go-decide` does not
import it today.

The root command is built in `internal/cli/cli.go`. `cli.Run` runs
`ax.Execute` and, when that succeeds, returns the exit code of an outcome the
command recorded through a shared pointer. That is how `ask` and `score` exit
10 and 11 for `uncertain` and `escalate`. The output schema version is 4
(`internal/cli/output.go`).

## Goals / Non-Goals

**Goals:**

- Serve `ask` and `score` with no per-tool code, over stdio and loopback HTTP.
- Keep outcomes and failures distinct over MCP, as they are on the command
  line.
- Keep the MCP SDK out of `decision`, `jevclient`, `clefclient` and `eval`.

**Non-Goals:**

- Authentication or TLS for the HTTP transport. `ax-go` runs no auth flow, and
  an exposed endpoint needs one in front of it.
- Streaming results, MCP resources or prompts.
- Any change to what an outcome means or to the thresholds.

## Decisions

### Mount in `newRoot`, exclude `eval`

Add `mcp.NewCommand(root, mcp.WithVersion(env.Version))` after the other
commands and call `mcp.Exclude` on `eval`. `eval` stays in `--help` and in
`__schema --as=ax`. Exposing it was rejected: a single tool call would spend
one request per decision with no per-call confirmation, and an agent could
repeat it.

### Outcomes must not reach the process exit code

The server process exits 0 after a normal shutdown and a failure code (1 to
4, 2 for a refused start) when it cannot run. A tool call never ends the
server: its own exit code only decides whether that one result is flagged as
an error. The outcome codes 10 and 11 exist only on the command line and must
never be the server's exit code.

A tool call runs `ask` through the same command tree, so the `uncertain` or
`escalate` it records lands in the shared pointer. When the server ends,
`Run` would return 10 or 11 for a session that never failed, and concurrent
calls would write that pointer at once. The recorder is concurrency safe, and
`Run` returns the plain code for `mcp-server` instead of the recorded outcome.
The tool result is already correct: a command that returns nil exits 0, so
`ax-go` does not set `IsError`. A test pins both the result and the exit code,
and the suite runs with `-race`.

### Schema version 5

`mcp-server` is a new command in `__schema`, so `SchemaVersion` moves from 4
to 5. Every output carries the version, so every golden set gets a `v5` file
and the `v4` files stay as they are.

### Tests drive a real server over loopback HTTP

Stdio cannot be driven from inside a test. As `ax-go`'s own tests do, start
`mcp.Serve` on a loopback port with `Env.NewClient` and `Env.NewClefClient`
faked through `httptest`, and call it with the SDK client. The SDK becomes a
direct test dependency. Tests never read `.env` and send nothing to a real
backend.

### Keep the SDK out of the libraries

`internal/cli` is the only package that imports `ax-go/mcp`. The dependency
test in `internal/cli/deps_test.go` adds the SDK module to its list of
modules the library packages must not pull in.

### `--spec -` is not supported over MCP

On stdio the standard input is the protocol channel, so a tool that reads a
spec from it would consume protocol bytes. Tool calls give the spec inline or
by file path. The docs say so, and a test shows `--spec -` fails with a
validation error instead of hanging.

## Risks / Trade-offs

- **Spend.** An agent can call `ask` repeatedly, each call paid. Mitigations
  are documentation, `--dry-run`, and excluding `eval`. A per-session cap is
  out of scope and would be a separate change.
- **A decided result still is not approval.** The docs repeat that over MCP
  `decided` means clear enough to skip the debate.
- **New dependency weight.** The SDK and its own dependencies enter `go.mod`
  and every release binary. It is confined to `internal/cli`, and the version
  is the one `ax-go` already pins.
- **Version.** `ax-go` refuses `dev` and `unknown`, but `cli.Run` resolves the
  version first (stamped, module version, or a build pseudo-version), so the
  server always has one and never refuses to start for it. A local build
  reports a pseudo-version.
- **Flag drift.** Tool inputs mirror the command's flags, so a renamed flag
  changes a tool schema. The `__schema` goldens pin the flags, which covers it.

## Open Questions

None that change what is built. If the SDK client needs a newer Go than
1.27.1 or adds a conflicting dependency, stop and report before choosing a
workaround.
