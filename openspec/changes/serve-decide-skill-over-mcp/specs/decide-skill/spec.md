# Spec Delta

## Purpose

Defines what the `decide` skill shipped in `skills/decide/` promises about its
use of go-decide inside the debate, so model output stays evidence an agent
weighs and never becomes the verdict.

## ADDED Requirements

### Requirement: Debate agents may call go-decide as evidence

When `go-decide-ask` and `go-decide-score` (or the `go-decide` binary) are
available, the skill SHALL allow the advocates and the moderator to call them
during the debate. Without them, the debate SHALL run unchanged.

#### Scenario: Tools connected

- **WHEN** the debate runs with the go-decide MCP tools connected
- **THEN** each agent prompt says the agent may call `go-decide-ask` or
  `go-decide-score` on a question it frames itself

#### Scenario: Tools absent

- **WHEN** neither the MCP tools nor the binary is available
- **THEN** the agent prompts do not mention them and the debate runs as before

### Requirement: Model output is evidence, never the verdict

The skill SHALL require an agent that cites a go-decide result to cite it as
one piece of evidence with its backend and outcome, and SHALL NOT let it decide
the debate. `confidence` SHALL be described as a ranking score, never as a
probability, and a `leading` option SHALL never be called a choice.

#### Scenario: Cited result

- **WHEN** an advocate cites a go-decide result in a position paper
- **THEN** the citation names the backend and the outcome, calls `confidence`
  a ranking score, and the moderator's consensus rests on the arguments, not on
  that result

#### Scenario: Failed call

- **WHEN** an agent's go-decide call fails
- **THEN** the agent reports the failure and argues without it, and does not
  treat the failure as an outcome

### Requirement: Debate tool calls are capped

The skill SHALL cap go-decide calls at two per agent per round, because each
call is a paid request. When the user asked for no spend, debate agents SHALL
make no go-decide call, because a dry run has no outcome to cite.

#### Scenario: Cap reached

- **WHEN** an agent has made two go-decide calls in a round
- **THEN** it makes no further call in that round

#### Scenario: No spend

- **WHEN** the user asked for no spend
- **THEN** no debate agent calls go-decide, and none cites a dry run as
  evidence

### Requirement: Each run declares and reports its anchoring mode

The skill SHALL run in one of two modes: `blind` (all three agents write
Round 1 without any go-decide result or call) or `informed` (the pre-screen
result is in the brief and calls are allowed from Round 1). It SHALL NOT pick
a mode silently, and SHALL report the mode with the results.

#### Scenario: Mode named by the user

- **WHEN** the user names `blind` or `informed`
- **THEN** the run uses that mode without asking

#### Scenario: Mode not named

- **WHEN** the go-decide tools are available, the debate will run, and the
  user named no mode
- **THEN** the skill asks for the mode before Round 1 starts

#### Scenario: Blind Round 1

- **WHEN** the run is `blind`
- **THEN** no Round 1 agent prompt contains the pre-screen result, no agent
  calls go-decide in Round 1, and calls are allowed from Round 2

#### Scenario: Mode reported

- **WHEN** the run presents its results
- **THEN** the results state the anchoring mode used
