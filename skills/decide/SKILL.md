---
name: decide
description: >
  Three-agent adversarial debate for strategic decisions, with an
  optional go-decide pre-screen. A clear System One answer skips the
  debate. Otherwise two advocates steelman opposing positions while a
  moderator identifies risks and writes a binding consensus. Use when
  choosing between alternatives, evaluating tradeoffs, or making
  high-stakes decisions.
compatibility: >
  The debate needs an agent that can spawn parallel sub-agents and
  research the web. The optional pre-screen calls go-decide ask, or
  the MCP tool go-decide-ask, and skips itself when that tool is
  absent. Debate agents may call go-decide as capped evidence when it
  is available.
---
<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Adversarial Consensus Protocol

Run a structured three-agent debate to reach consensus on a strategic
decision. Two advocates steelman opposing positions while a moderator
asks hard questions, identifies risks, and synthesizes the binding
consensus.

**Why this works**: adversarial structure surfaces arguments
brainstorming misses. Evidence-grounded research prevents epistemic
closure. Forced concessions in Round 2 prevent entrenchment. The
moderator writes the binding consensus after hearing both sides.

When `go-decide` is available, run the optional pre-screen below
before spending a debate. A decided result skips the debate. It does
not approve the choice.

## Pre-flight validation

Before running the debate, verify the input contains a genuine decision
with at least two distinguishable options or courses of action.

If the input is a factual question, explanation request, or
single-option statement, respond directly instead: "This protocol
works best when choosing between alternatives. Could you rephrase
with the options you're weighing, or ask me directly?"

When in doubt, proceed — err on the side of running the debate.

If the user asks to review or label the decision log, go to "Label past
decisions" below instead. Input turned away here is never recorded.

## Phase 0: Research and frame the decision

### Step 0.1: Gather context

Read project documentation and context files to understand the
decision space. Common sources include README, architecture docs,
product docs, recent git history, and any files the user referenced.

### Step 0.2: Identify the two positions

From the decision topic, formulate two clear, opposing positions:

- **Position A**: one side of the decision
- **Position B**: the opposing side

If the topic naturally implies two sides (e.g., "gRPC vs REST"),
use those directly. If the topic is open-ended (e.g., "pricing
strategy"), research the space and formulate the two strongest
opposing approaches.

### Step 0.3: Identify constraints

Extract hard constraints from the user's message or project docs:
budget limits, timeline requirements, team size, technical
constraints, business requirements, non-negotiable values.

### Step 0.4: Present brief

Generate the debate brief before proceeding. Evaluate against three
conditions:

- Can the two positions be stated without significant overlap?
- Is the question genuine (not leading or presupposing an answer)?
- Are there at least two meaningfully distinct outcomes?

If all conditions pass, present the brief, record the decision (Step
0.5), and run the optional fast-path pre-screen. Start Phase 1 only
when that section says to.

```text
Decision: [one-line summary]
Position A: [name] — [one-sentence description]
Position B: [name] — [one-sentence description]
Constraints: [list any hard constraints]
Model evidence: [blind | informed | not named yet]
```

Include the `Model evidence` line only when go-decide is available.
See "go-decide during the debate" below.

If any condition fails, present the brief and ask the user whether to
reframe or proceed as-is. Proceeding as-is records the decision and
runs the pre-screen too.

### Step 0.5: Record the decision

Only when go-decide is available. Call `go-decide-record-decision`
now, before the pre-screen or Round 1, with the brief as it stands:
the title, context, constraints, instructions and every option the
pre-screen will send. See `references/decision-log.md` for the inputs.
The record holds nothing a run produces, because it is the evidence of
what was known before the answer.

- `recording: false` means recording is off. Say nothing about it.
- Otherwise tell the user the returned `id` and `log` in one line, and
  keep the id for "Record the run".
- A validation error naming `--yes` means the log is inside a git
  working tree. Tell the user it could be committed there, and call
  again with `yes` only after they confirm.
- Skip the record, and say no record was written, when the user asks
  not to record this decision.

## Optional fast-path pre-screen

Run this after Phase 0 and before Phase 1. It asks `go-decide` one
choice question. Skip it, and continue at Phase 1, when the user asked
for the full debate or when neither the `go-decide` binary nor the MCP
tool `go-decide-ask` is available. Never invent an outcome.

The pre-screen does not run the debate, and a result is not approval.

### Build the question

Take the brief from Phase 0.

- `state` is the gathered context and the hard constraints. It must be
  non-empty.
- `instructions` is the one-line decision, phrased as which option to
  choose. Always send it. clef rejects an empty instruction.
- `options` are the options the user named, each with a name and a
  one-sentence description. When the user named more than two, pass
  every one of them. When Phase 0 had to formulate the two positions,
  use those two.

Prefer the MCP tool `go-decide-ask` when it is connected. Otherwise
run:

```sh
go-decide ask --state "..." --instructions "..." \
  --option name="description" --option name="description"
```

Pass `--backend jev` or `--backend clef` only when the user named one.
Do not retry a failure against the other backend. Do not pass an API
key as an argument. Pass `--floor` and `--confident` only when the
user set them. The defaults are placeholders, not tuned thresholds.

The CLI validates the question before any request. Add `--dry-run`
only when the user wants that check without spending. A dry run has
no `outcome`. It is not a result.

### Branch on the outcome

Read `data.outcome` from the JSON envelope. The CLI prints that
envelope on standard output. The MCP tool `go-decide-ask` returns the
same envelope. Branch on `data.outcome`, not on a process exit code.

The pre-screen was not run when the binary and the MCP tool are both
absent. Continue at Phase 1. A command the shell cannot find is that
case, not a failed call.

A call that was made and failed is not an outcome. On the CLI, a
failure prints an error on standard error, leaves standard output
empty, and exits with a code other than 0, 10 or 11. Those three
codes match decided, uncertain and escalate. Over MCP, a failure is
the error envelope flagged as an error. Exit codes 10 and 11 do not
exist there, and the server staying up does not mean the call
succeeded. Show the error and stop. Do not start Phase 1 unless the
user then asks for the debate. Do not treat the failure as escalate,
and do not fill in a choice. Record the run with path `stopped` and
the error code (see "Record the run").

**decided.** Present `choice`, `confidence`, `probabilities` and
`thresholds`. Say it is clear enough to skip the debate, and that it
is not approval. Skip Phase 1, Phase 2 and Phase 3. There was no
debate to summarize. Do not act on the choice. Record the run with
path `fast`. The user can still demand the debate, and the user has
the final say.

**uncertain.** Present `leading`, `confidence`, `probabilities` and
`thresholds`. Call `leading` the leading option, never a choice. This
case is for a person. Ask whether to stop or to run the debate. Do
not start Phase 1 until they answer. If they stop, record the run with
path `stopped`.

**escalate.** Present `leading` and `confidence`, then run Phase 1
unchanged. `leading` is the leading option, not a choice and not a
decision.

`confidence` and `probabilities` rank the options. They are not
calibrated probabilities, and they can change between identical
calls. The default floor of 0.5 and confident level of 0.9 are
placeholders. On the synthetic probe, some decided answers were not
safe to fast-path. Say that when you present a decided result.

## go-decide during the debate

This section applies only when the MCP tools `go-decide-ask` and
`go-decide-score`, or the `go-decide` binary, are available. Without
them, leave go-decide out of every agent prompt and run the debate as
written.

### Choose the model evidence mode

Whether agents argue better with the model's answer in front of them,
or without it, is an open question. Each run uses one mode and reports
it. Do not pick one yourself.

- **blind**: no Round 1 agent prompt contains the pre-screen result,
  and no agent calls go-decide in Round 1. Agents may call it from
  Round 2.
- **informed**: the Round 1 prompts contain the pre-screen result, and
  agents may call go-decide from Round 1.

Use the mode the user named. If they named none, ask once the debate
is going to run, before Round 1: after an escalate, or when the user
chose the debate after an uncertain result or a skipped pre-screen.
Do not ask when the pre-screen decided and the debate is skipped.

### Rules for agents that call go-decide

Give each agent the go-decide block from `references/debate-prompts.md`
when its mode and round allow calls. The block carries these rules:

- **At most two calls per agent per round**, `ask` and `score`
  together. Every call is a paid request.
- **No calls when the user asked for no spend.** A dry run has no
  outcome, so it is not evidence.
- **Evidence, never the verdict.** Cite a result with its backend and
  outcome. A result that favours a position is one argument for it.
  The Moderator's consensus rests on the arguments, not on a model
  answer.
- **Show what was asked.** Agents frame their own questions, and an
  advocate may frame one for its side. The citation carries the
  instructions sent and every option or level with its probability,
  so a reader can see when a question left a debated position out.
  Keep these details when you summarize a paper for Round 2.
- **Read the result honestly.** `confidence` is a ranking score, never
  a probability, and it can change between identical calls. `leading`
  is the leading option, never a choice.
- **A failure is not an outcome.** Report it and argue without it. Do
  not retry against the other backend, and never pass a credential.
- **Prefer the MCP tool**, then the binary. An agent that has neither
  argues without go-decide.

## Phase 1: Round 1 — position papers

Tell the user you are launching Round 1, then launch **three agents
in parallel**. Each agent runs independently and cannot read the
others' output.

- **Advocate A**: steelman Position A. Research externally to find at
  least 2 supporting sources. Write a comprehensive position paper
  with architecture, implementation plan, costs, timelines, risks,
  and mitigations. Cite sources inline.
- **Advocate B**: steelman Position B with the same structure and
  research requirements as Advocate A.
- **Moderator**: critically examine both positions using external
  counter-evidence. Find at least 2 sources that challenge each
  position. List the 10 hardest questions (5 per position), identify
  the 5 biggest risks per approach, propose a hybrid model, and reach
  a preliminary recommendation.

Compose agent prompts following
`references/debate-prompts.md` — Round 1 section. In `informed` mode,
add the pre-screen result and the go-decide block. In `blind` mode,
add neither.

### Synthesize Round 1

After all three agents complete, present to the user:

1. **Where all three agree** — table of consensus points
2. **Where they disagree** — table of tensions with each agent's
   position
3. Brief commentary on the key tensions

Then immediately proceed to Round 2.

## Phase 2: Round 2 — forced convergence

Launch **three agents in parallel**. Each agent receives a summary
of the other agents' arguments and must respond.

- **Revised Advocate A**: respond to the strongest challenges from
  Advocate B and the Moderator. Concede valid points honestly, defend
  where Position A remains strongest, and state a revised position.
- **Revised Advocate B**: same structure as Revised A, responding to
  challenges from Advocate A and the Moderator.
- **Consensus Moderator**: drive toward final consensus. Write the
  binding consensus document: the agreed model, resolved tensions,
  execution timeline, 3 key metrics, the single biggest risk, and a
  one-paragraph elevator pitch.

Compose agent prompts following
`references/debate-prompts.md` — Round 2 section. Add the go-decide
block in either mode when go-decide is available.

## Phase 3: Present results

### Step 3.1: Present the consensus

Show the user:

1. Key concessions from each advocate (what changed their minds)
2. The consensus model — the Moderator's final recommendation
3. Structure or plan details (tiers, architecture, timeline)
4. Key metrics to track
5. Biggest risk and mitigation
6. The model evidence mode (`blind` or `informed`) and how many
   go-decide calls the agents made, when go-decide was available

Then record the run with path `debate`.

### Step 3.2: Ask about persistence

Ask the user if they want to save the consensus. Determine the
appropriate file based on the decision type:

- Business decisions → `biz.md`
- Architecture decisions → `ARCHITECTURE.md` or `ADR-NNN.md`
- Product decisions → `PRODUCT.md`
- Strategy → appropriate project document
- Or a new file if none fits

## Record the run

Only when Step 0.5 wrote a `decision` record. Each time a run ends,
call `go-decide-record-run` with that `id` and the `path`, following
`references/decision-log.md`:

- **`fast`**: the pre-screen decided and the debate was skipped.
- **`stopped`**: the user stopped after an uncertain result or a failed
  call. A failure carries its error code and no outcome.
- **`debate`**: Phase 3 presented the consensus. Add the Moderator's
  recommendation, the model evidence mode when go-decide was
  available, the debate agents' go-decide call count, and one citation
  per go-decide result an agent cited, taken from its citation block.

Pass the pre-screen's envelope `data` as `pre-screen`, its error code
as `pre-screen-error`, or why it did not run as `pre-screen-skipped`.
A debate the user starts after a `fast` or `stopped` run gets a second
`go-decide-record-run` call; go-decide never changes a written line.

## Label past decisions

Run this when the user asks to review or label the decision log,
following the labelling steps in `references/decision-log.md`. Call
`go-decide-record-pending`, with `older-than-days` when the user names
a threshold (30 by default). With `recording: false`, say recording is
off and how to turn it on, and stop. For each listed decision, show
only what the listing holds and ask how it turned out, then call
`go-decide-record-outcome`.

- **Never show a run before the label is recorded**, from the log or
  from memory. The pick, confidence or recommendation would anchor the
  label.
- The label is the user's: `dominant` with the option that proved
  right, or `contested` when reasonable people would still disagree,
  and why. Never suggest or invent one.
- Write nothing for a decision the user skips.

## Orchestrator guidelines

- **Run the pre-screen before Phase 1** when `go-decide` is available
  and the user did not ask for the full debate. Branch on
  `data.outcome`. A failure is not an escalate.
- **Never choose the model evidence mode yourself.** Use the one the
  user named, or ask before Round 1. Report it with the results.
- **Record the decision before any answer**, through the record
  tools, and stay silent when recording is off. Never show a run while
  the user labels its outcome.
- **Always launch all 3 agents per round in a single step** (parallel)
- **Never edit the agents' arguments** — present them faithfully
- **The Moderator's Round 2 consensus is the binding output** — but
  the user has final authority to override
- **Keep your own commentary brief** — the agents did the thinking;
  you synthesize and present
- **If agents do not converge after Round 2**, highlight the remaining
  disagreement and ask the user to break the tie rather than running
  more rounds
- **Adapt prompt specifics to the domain** — a pricing decision needs
  cost numbers; an architecture decision needs technical tradeoffs
- **Include relevant project context** in every agent prompt — agents
  run in isolation and cannot read each other's output
