# jev-decide Strategic Roadmap

## Vision

A typed, safe-by-construction Go client and CLI for Jev decision support:
fast answers when Jev is confident, explicit escalation when it is not. See
[CONTEXT.md](CONTEXT.md) for boundaries.

Research so far (2026-09-28 and 2026-09-29) supports gating and labelling,
not ranking or auto-approval: see `docs/probe-2026-09-28.md`,
`docs/probe-round2-2026-09-28.md` and `docs/spike-decisions-2026-09-29.md`.

## Immediate Focus (v0.1.0)

- [x] #4 ask and score commands with versioned JSON schemas [M]
  (implemented in `internal/cli`, `cmd/jev-decide` and `decision/`; closes
  with its PR)

## Near-Term Vision (v0.2.0)

- [x] #5 eval command for confidence separation and calibration [M]
  (implemented in `eval/`, `internal/cli/eval.go` and the `jevclient`
  response cache; closes with its commit; promoted from Near-Term). The
  bundled 40-decision set is a smoke test: contested AUC 0.985 on 2026-10-01,
  with 2 contested decisions above 0.9. #6 needs real past decisions.
- [ ] #6 Tune confidence threshold on real decisions [M]
  (thresholds are placeholders until then)
- [ ] #12 Cover key redaction on retry and failure log lines [S]
- [ ] #13 Document the logging policy in the `jevclient` package doc [S]

## Future Vision (Long-Term)

- [ ] #7 Fast-path pre-screen for the decide skill [M]
- [ ] #8 Batching and pseudonymized identifiers [M]
- [ ] #9 Spike: does the fast path hold on real past decisions? (timebox 1d)

## Completed Milestones

### 2026-Q3

- [x] #1 `jevclient`: typed Jev client core. Closed 2026-09-30. [L]
- [x] #2 `decision`: validated Probability, sealed Result. Closed 2026-09-30. [M]
- [x] #3 `decision`: validate spec before any API call. Closed 2026-09-30. [M]
- [x] #10 `spike`: FinFocus scoring belongs in finfocus repos. Closed 2026-09-30.
- [x] #11 `spike`: keep `kataras/jev`, do not write a client. Closed 2026-09-30;
  see `docs/jev-clients.md`.
- Research probes: recommendation scoring, duplicate detection, decision
  spike, client survey (in `docs/`)

## Boundary Safeguards

- No debate protocol: it stays in the `decide` skill.
- No auto-approval from Jev scores; low confidence escalates.
- No silent zero values or bare-float probabilities.
- Validate input before spending on the API.
- No committed API token; only `api.typesafe.ai` or an explicit base URL.
- No FinFocus core coupling; scoring goes through a plugin or spec RPC.
