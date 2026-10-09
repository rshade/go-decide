# Design

## Context

Pull request #41 (merged) moved the `decide` skill into `skills/decide/` and
added the pre-screen. This change builds on it.
`go-decide mcp-server` is ax-go's `mcp.NewCommand`, mounted in `newRoot`
(`internal/cli/cli.go`). ax-go v0.9.0 adds what this change needs:

- `schema.DeclareResource` with a `Content` field that `resources/read`
  serves and `__schema` never projects (1 MiB cap);
- `schema.DeclarePrompt`, rendered by `prompts/get` with `{{name}}`
  substitution (64 KiB template cap);
- `mcp.WithInstructions`, sent in the `initialize` result (8 KiB cap).

A CLI that declares nothing keeps the v0.8.0 handshake. Declarations sit on a
command node, so `__schema` and `__schema --as=mcp` both list them, and
go-decide's own `__schema` wrapper keeps them because it copies
`doc.Command` whole.

## Goals / Non-Goals

**Goals:**

- One copy of the skill: the repository file is what the binary serves.
- An agent learns the skill exists from the handshake alone.
- Debate agents can use go-decide without the result turning into a verdict.

**Non-Goals:**

- Deciding the anchoring question. The skill records it per run; the decision
  comes from runs (#22 comment, 2026-10-07).
- Logging decisions or runs (#40).
- Declaring ax-go capability classes for `ask` and `score`.
- Changing `ask`, `score` or `eval` output, or what an outcome means.

## Decisions

### A root `skills` package embeds the files

`//go:embed` cannot name a parent directory, so `internal/cli` cannot embed
`skills/decide/` directly. A small package at `skills/` (`skills.go`) embeds
`decide/SKILL.md` and `decide/references/*.md` and exposes them as an
`fs.FS`. It imports only the standard library.

- *Alternative: copy the files under `internal/` with `go generate`.* Two
  copies drift, and a stale copy passes every test that does not compare
  them. Rejected.
- *Alternative: move the skill under `internal/cli/`.* Skill installers look
  for `skills/<name>/SKILL.md` at the repository root. Rejected.

The package is public because it must sit at the root. Its API is one
function, and its package doc says it exists for the binary.

### Resource URIs mirror repository paths

Each embedded file is declared at `go-decide://skills/decide/<path>`, for
example `go-decide://skills/decide/references/debate-prompts.md`. The skill
says "follow `references/debate-prompts.md`", and that relative path resolves
against the `SKILL.md` URI by ordinary URI rules, so the skill text needs no
MCP-specific wording. Resources are declared by walking the embedded tree in
sorted order, so a new reference file is served without a code change.

- *Alternative: a flat scheme such as `go-decide://decide`.* The relative
  references would then need a mapping table in the instructions. Rejected.

All resources are declared on the root command, which is never hidden.

### Instructions come from the skill; the resource teaches

The instructions are built from `SKILL.md`'s frontmatter, parsed with
`go.yaml.in/yaml/v3`. They open with a directive pointer that names the
shapes a choice arrives in ("should we X or Y", "is it worth adopting X",
"help me pick", "thoughts on switching to X"), tells the agent not to answer
from its own judgment first, and points at
`go-decide://skills/decide/SKILL.md`. Then come the skill's `description`
(which already says when to use it) and one sentence about this server's
tools that the skill does not carry: every `ask` and `score` call is a paid
request and a decided outcome is not approval. The wording was tuned in
Claude Code trials, recorded under task 5.2. The same `description`
describes the `SKILL.md` resource and the `decide` prompt. They stay far
under 8 KiB. The skill text stays in the resource, because instructions sit
in every session's context and the skill
is 10 KB.

- *Alternative: a hand-written Go constant.* It restates the description and
  drifts from it. Rejected.
- *Alternative: hand-parse the folded `>` scalar.* YAML folding has enough
  edge cases that a parser is cheaper than a bug. The module was already in
  the build graph through test dependencies.

Because `__schema` carries the prompt and resource descriptions, editing the
skill's `description` changes `__schema` and needs a `SchemaVersion` bump,
like any other output change.

### The prompt is a pointer

`decide` has one required argument, `decision`, and a template of two
sentences: run the `decide` skill on `{{decision}}` by reading the skill
resource and following it. The template is part of `__schema`, so it stays
short.

### Debate tool use lives in the prompts, with a cap of two

`SKILL.md` gains a short section on go-decide during the debate, and
`references/debate-prompts.md` gains a block each agent prompt includes when
the tools are available: how to call, how to cite (backend, outcome,
`confidence` as a ranking score), what a failure means, and the cap. The
citation also carries the instructions sent and every option or level with
its probability. Advocates frame questions for their own side by design, and
the trial debate showed a question that left out the rival position returning
decided at 0.95. The rule exposes the framing instead of forbidding it. Two
calls per agent per round bounds a full debate at twelve paid calls. Like
the thresholds, two is a placeholder until runs show what agents use.

### The anchoring mode is asked for, not defaulted

The brief gains a line, `Model evidence: blind | informed`. If the user did
not name one and the tools are available, the skill asks once it knows the
debate will run, before Round 1. Asking at the brief would waste the question
whenever the pre-screen is decided. There is no default, because a default is
an answer to the open question. Phase 3 reports the mode, so #40 can log it
later.

A dry run has no outcome, so it is not evidence. When the user asked for no
spend, the debate agents make no go-decide calls at all.

### ax-go v0.9.0

Development ran against an ax-go pseudo-version from `main` while the v0.9.0
release pull request (rshade/ax-go#279) was open. The branch moved to the
`v0.9.0` tag before it was pushed, and the tests and goldens ran again with no
change. go-sdk v1.8.0, which v0.9.0 needs, reached `main` separately through
Renovate.

### One schema bump covers both causes

ax-go v0.9.0 adds a global `--strict` flag and a `warnings_as_errors` known
code, which changes `__schema` on its own. The skill declarations change it
again. Both land in version 6. A new golden case, `schema.mcp`
(`__schema --as=mcp`), starts at version 6 and pins the prompt and resource
metadata.

## Risks / Trade-offs

- [The model may not read the resource from the instructions alone] → It
  did not, while the pointer was descriptive ("For such a decision, read
  …"): instructions arrived verbatim, and a strategic question was still
  answered directly. A directive pointer ("do not answer from your own
  judgment first: read …") made it read the skill and follow it, and a small
  choice was still answered directly. Putting the pointer first and naming
  the shapes a choice arrives in raised the read rate to 29 of 30. The
  results are in task 5.2 and on rshade/ax-go#270. #48 makes the check
  repeatable.
- [A sub-agent may not inherit the MCP tools] → The prompt block says to use
  the `go-decide` binary when the tools are absent, and to argue without
  either. The cap applies to both.
- [Asking for the mode adds a question to every debate] → It is asked once,
  only when the debate will run and the tools are available. The user can
  name the mode in the request to skip it.
- [A served skill can lag the repository] → It is the skill of the binary's
  own build, which is the version that matches the tools it describes.

## Migration Plan

Additive for MCP clients. A client that ignores resources, prompts and
instructions sees the same tools. `__schema` consumers move to version 6 and
can keep reading version 5 files. Rollback is a revert.

The operator's steps outside this repository (point
`~/.agents/.skill-lock.json` at `rshade/go-decide`, drop the copy in
`rshade/agent-skills`) are unchanged by this design.

## Open Questions

- Which anchoring mode gives better debates. Answered by runs, not here.
- Whether two calls per agent per round is the right cap.
