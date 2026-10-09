# Proposal

## Why

An agent that connects to `go-decide mcp-server` gets two tools and no idea
when to use them. The `decide` skill that knows (frame the decision,
pre-screen with `ask`, escalate to a debate) only reaches an agent that
installed it from this repository by hand. ax-go v0.9.0 (rshade/ax-go#270)
serves declared prompts, static resources and server instructions, so the
binary can carry the skill itself.

## What Changes

- Embed `skills/decide/SKILL.md` and `skills/decide/references/*.md` in the
  binary, and serve each as an MCP resource whose URI mirrors its path
  (`go-decide://skills/decide/SKILL.md`). The repository file stays the only
  copy.
- Send short server instructions at `initialize` that tell the agent when to
  read the skill resource, and that every `ask` and `score` call is a paid
  request.
- Declare a short `decide` prompt that takes the decision as an argument and
  points at the skill resource. It is the user's manual entry point
  (`/mcp__go-decide__decide` in Claude Code), not a copy of the skill.
- Let the advocates and the moderator call `go-decide-ask` and
  `go-decide-score` (or the CLI) during the debate, as evidence: capped per
  agent per round, never the verdict, and never quoted as a probability.
- Make anchoring a per-run experiment rather than a design decision. Each run
  states whether the advocates see model output before writing their Round 1
  positions (`informed`) or only after (`blind`), and reports which mode it
  used.
- Bump ax-go from v0.7.0 to v0.9.0 (built against ax-go `main` until the tag
  exists), which also adds ax-go's global `--strict` flag and the
  `warnings_as_errors` known code to `__schema`.
- Move `SchemaVersion` from 5 to 6, with a new golden file for every output,
  a new `__schema --as=mcp` golden, and the v5 files untouched.
- Document the served skill in the README, `docs/jev-decide-cli.md`,
  `CONTEXT.md` and `CLAUDE.md`.

## Capabilities

### New Capabilities

- `decide-skill`: what the `decide` skill shipped in `skills/decide/` promises
  about its use of go-decide during the debate: the fast-path pre-screen,
  advocate and moderator tool calls as capped evidence, and the anchoring mode
  each run declares and reports.

### Modified Capabilities

- `mcp-server`: the server also serves the skill as resources, a `decide`
  prompt, and server instructions, all from the embedded files.
- `cli-output-contract`: `__schema` at version 6 lists the declared prompt and
  resources (metadata only) and ax-go's `--strict` flag.

## Impact

- New package `skills` (an `embed.FS` over `skills/decide/`), because
  `//go:embed` cannot reach a parent directory from `internal/cli`. It imports
  only the standard library.
- `internal/cli/cli.go` (declarations and instructions on the root and the
  `mcp-server` command), `internal/cli/output.go` (`SchemaVersion`),
  `internal/cli/golden_test.go`, `internal/cli/mcp_test.go`, new
  `*.v6.json` goldens.
- `skills/decide/SKILL.md` and `skills/decide/references/debate-prompts.md`
  (advocate tool use and the anchoring mode).
- `go.mod` and `go.sum`: ax-go v0.9.0, and the go-sdk v1.8.0 it requires.
  This supersedes Renovate's ax-go v0.8.0 pull request (#43) and the go-sdk
  one (#42).
- Stacked on #41 (`issue-7`), which moves the skill into this repository and
  adds the pre-screen. This change rebases onto `main` once #41 merges.
- No new spend: serving a resource or a prompt makes no backend request. The
  debate's tool calls are paid, which is why they are capped.
