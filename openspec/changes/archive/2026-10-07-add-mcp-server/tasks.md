# Tasks

## 1. Failing tests first

- [x] 1.1 Add an in-process test that starts `mcp.Serve` on a loopback port
  with faked `Env.NewClient` and `Env.NewClefClient`, and connects the SDK
  client. Cover tool listing, `ask` decided, uncertain and escalate (not
  `IsError`, payload has `outcome`), a validation failure (`IsError`, exit-2
  envelope, zero backend requests) and a dry run. Verify: the new tests fail
  because there is no `mcp-server` command yet.
- [x] 1.2 Add tests for `eval` being absent and still working on the command
  line, `--backend clef` as an input, missing credentials for the chosen
  backend with no fallback, a redacted credential in a result, and `--spec -`
  failing instead of hanging. Verify: they fail for the same reason.

## 2. Mount the server

- [x] 2.1 Mount `mcp.NewCommand` in `newRoot` with `Env.Version`, and call
  `mcp.Exclude` on `eval`. Verify: the tests from 1.1 and 1.2 pass.
- [x] 2.2 Switch outcome recording off for `mcp-server` and make `Run` return
  the plain code for it. Verify: a test shows exit 0 after a session that
  returned an uncertain outcome, and `go test -race ./internal/cli` passes.
- [x] 2.3 Add tests for the refusal of a non-loopback address (exit 2) and
  the version reported in the handshake. Verify: they pass.

## 3. Output contract

- [x] 3.1 Bump `SchemaVersion` to 5 and add the `*.v5.json` golden files with
  `go test ./internal/cli -run Golden -update`, leaving the v4 files
  untouched. Verify: `git diff --stat` shows no v4 file changed and the golden
  tests pass.
- [x] 3.2 Add the SDK module to the forbidden list in
  `internal/cli/deps_test.go`. Verify: that test passes, and fails if a
  library package imports `ax-go/mcp`.

## 4. Documentation

- [x] 4.1 Add "Running as an MCP server" to `docs/jev-decide-cli.md` and the
  README: the tools, a client configuration example, the spend warning,
  `--dry-run`, outcomes as results, credentials from the environment, loopback
  HTTP and no `--spec -`. Update the `CONTEXT.md` inbound line and the
  `CLAUDE.md` architecture section. Verify: markdownlint passes and the docs
  tests pass.

## 5. Final verification

- [x] 5.1 Break check: remove the mount and confirm the MCP tests fail, then
  restore it. Verify: the failure names the missing tool.
- [x] 5.2 Run `go build ./...`, `go test -race ./...`,
  `golangci-lint run ./...`, `mise exec -- markdownlint-cli2 "**/*.md"` and
  `mise exec -- openspec validate --all --strict`. Verify: all pass.
