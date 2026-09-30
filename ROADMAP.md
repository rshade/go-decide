# jev-decide Strategic Roadmap

## Vision

A typed, safe-by-construction Go client and CLI for Jev decision support:
fast answers when Jev is confident, explicit escalation when it is not. See
[CONTEXT.md](CONTEXT.md) for boundaries.

Research so far (2026-09-28 and 2026-09-29) supports gating and labelling,
not ranking or auto-approval: see `docs/probe-2026-09-28.md`,
`docs/probe-round2-2026-09-28.md` and `docs/spike-decisions-2026-09-29.md`.

## Immediate Focus (v0.1.0)

- [ ] #1 Typed Jev client core [L]
- [ ] #2 Validated Probability and sealed Result types [M]
- [ ] #3 Validate decision spec before any API call [M]

## Near-Term Vision (v0.2.0)

- [ ] #4 ask and score commands with versioned JSON schemas [M]
- [ ] #5 eval command for confidence separation and calibration [M]
- [ ] #6 Tune confidence threshold on real decisions [M]

## Future Vision (Long-Term)

- [ ] #7 Fast-path pre-screen for the decide skill [M]
- [ ] #8 Batching and pseudonymized identifiers [M]
- [ ] #9 Spike: does the fast path hold on real past decisions? (timebox 1d)
- [ ] #11 Spike: keep kataras/jev or write our own client? (timebox 2h)

## Completed Milestones

### 2026-Q3

- [x] #10 `spike`: FinFocus scoring belongs in finfocus repos. Closed 2026-09-29.
- Research probes: recommendation scoring, duplicate detection, decision
  spike, client survey (in `docs/`, uncommitted)

## Boundary Safeguards

- No debate protocol: it stays in the `decide` skill.
- No auto-approval from Jev scores; low confidence escalates.
- No silent zero values or bare-float probabilities.
- Validate input before spending on the API.
- No committed API token; only `api.typesafe.ai` or an explicit base URL.
- No FinFocus core coupling; scoring goes through a plugin or spec RPC.
