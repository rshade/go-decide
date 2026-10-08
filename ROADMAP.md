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

Ship the current feature set safely under its final name. v0.1.0 waits for
the Clef spike verdict (#18) and keeps the placeholder thresholds.

- [ ] #16 Put the root probes behind a `probe` build tag [S]
- [ ] #17 Make markdownlint clean across the repo [S]
- [ ] #12 Cover key redaction on retry and failure log lines [S]
- [ ] #13 Document the logging policy in the `jevclient` package doc [S]
- [ ] #18 Spike: is Clef a drop-in System One provider? (timebox 1d)
  *Spike promoted 2026-10-06 - timebox timebox/1d; decide by 2026-10-07*
  Implemented as the OpenSpec change `add-clef-backend` (a `clefclient`
  package and `--backend clef`); see `docs/clef-2026-10-06.md`.
- [ ] #19 Rename the module, binary and repo to `go-decide` [M]
- [ ] #20 CI and release pipeline for v0.1.0 [L]
  (release-please and GoReleaser landed in `8223de5`; CI, docs and the
  release remain)

## Near-Term Vision (v0.2.0)

- [ ] #6 Tune confidence threshold on real decisions [M]
  (thresholds are placeholders until then; needs #9's real decisions)
- [x] #21 Serve the commands as MCP tools with `ax-go`'s `mcp-server` [M]
  Implemented as the OpenSpec change `add-mcp-server`; `eval` is not a tool.
- [ ] #22 Ship the `decide` skill in this repository [S]

## Future Vision (Long-Term)

- [x] #7 Fast-path pre-screen for the decide skill [M]
  Implemented in `skills/decide/`: after the decision is framed, the skill
  calls `ask` (or the MCP tool `go-decide-ask`) and branches on decided,
  uncertain and escalate. A decided result skips the debate and is not
  approval. Thresholds stay the placeholders until #6.
- [ ] #8 Batching and pseudonymized identifiers [M]
- [ ] #9 Spike: does the fast path hold on real past decisions? (timebox 1d)

## Completed Milestones

### 2026-Q4

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
