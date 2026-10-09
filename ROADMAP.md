# go-decide Strategic Roadmap

## Vision

A typed, safe-by-construction Go client and CLI for Jev decision support:
fast answers when Jev is confident, explicit escalation when it is not. See
[CONTEXT.md](CONTEXT.md) for boundaries.

Research so far (2026-09-28 and 2026-09-29) supports gating and labelling,
not ranking or auto-approval: see `docs/probe-2026-09-28.md`,
`docs/probe-round2-2026-09-28.md` and `docs/spike-decisions-2026-09-29.md`.

The GitHub repo, module path and binary are `go-decide`
(`github.com/rshade/go-decide`). The local directory stays `gojev`.
The task breakdown for v0.1.0 is in [TASKS.md](TASKS.md).

## Immediate Focus (v0.1.0)

v0.1.0 work is complete: the Clef spike (#18) is decided, and the placeholder
thresholds stay until #6.

*No active items. Next candidates are in Near-Term Vision.*

## Near-Term Vision (v0.2.0)

- [ ] #6 Tune confidence threshold on real decisions [M]
  (thresholds are placeholders until then; needs #9's real decisions)
- [x] #22 Ship the `decide` skill in this repository and serve it over MCP [L]
  Implemented as the OpenSpec change `serve-decide-skill-over-mcp` on ax-go
  v0.9.0: `mcp-server` serves the skill as resources, a `decide` prompt and
  instructions, and debate agents may cite go-decide as capped evidence.
- [ ] #40 Record decisions at decision time for later outcome labelling [M]
  (the corpus #9 needs; the `decide` skill writes the log)
- [ ] #47 Confirm the `@` mention and `decide` prompt in interactive Claude
  Code [S]

## Future Vision (Long-Term)

- [ ] #8 Batching and pseudonymized identifiers [M]
- [ ] #9 Spike: does the fast path hold on real past decisions? (timebox 1d)
  *Parked 2026-10-07: needs 20+ outcome-labelled records from the #40
  decision log. Past design docs leak their answers. Gates #6.*
- [ ] #48 Repeatable trigger-rate check for the server instructions [M]

## Completed Milestones

### 2026-Q4

- [x] #7 `skill`: fast-path pre-screen with ask. Closed 2026-10-09. [M]
- [x] #21 `cli`: serve ask and score as MCP tools. Closed 2026-10-07. [M]
- [x] #20 `ci`: CI and release pipeline for v0.1.0. Closed 2026-10-07. [L]
- [x] #19 `repo`: rename to go-decide. Closed 2026-10-07. [M]
- [x] #18 `spike`: Clef as a System One provider. Closed 2026-10-07.
- [x] #17 `docs`: markdownlint clean across the repo. Closed 2026-10-07. [S]
- [x] #16 `test`: root probes behind the `probe` build tag. Closed 2026-10-07. [S]
- [x] #13 `jevclient`: document the logging policy. Closed 2026-10-07. [S]
- [x] #12 `jevclient`: cover key redaction in retry logs. Closed 2026-10-07. [S]
- [x] #5 `eval`: eval command, metrics, response cache. Closed 2026-10-02. [M]
- [x] #4 `cli`: ask and score commands, versioned JSON. Closed 2026-10-01. [M]

### 2026-Q3

- [x] #1 `jevclient`: typed Jev client core. Closed 2026-09-30. [L]
- [x] #2 `decision`: validated Probability, sealed Result. Closed 2026-09-30. [M]
- [x] #3 `decision`: validate spec before any API call. Closed 2026-09-30. [M]
- [x] #10 `spike`: FinFocus scoring belongs in finfocus repos. Closed 2026-09-30.
- [x] #11 `spike`: keep `kataras/jev`, do not write a client. Closed 2026-09-30.
- Research probes: recommendation scoring, duplicate detection, decision
  spike, client survey (in `docs/`)

## Boundary Safeguards

- No debate protocol: it stays in the `decide` skill.
- No auto-approval from Jev scores; low confidence escalates.
- No silent zero values or bare-float probabilities.
- Validate input before spending on the API.
- No committed API token; only `api.typesafe.ai` or an explicit base URL.
- No FinFocus core coupling; scoring goes through a plugin or spec RPC.
