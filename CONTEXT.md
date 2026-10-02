# jev-decide Context & Boundaries

## Core Architectural Identity

`jev-decide` is a thin, strongly typed Go client and CLI for TypeSafe AI's Jev
(System One) model, aimed at fast-path decision support. It provides `ask`,
`score` and `eval` commands built on `ax-go`, plus an optional pre-screen for
the `decide` skill. It turns Jev's `choice`, `score` and `noul` answers into
values callers cannot misuse: typed option sets, validated probabilities and
a result that forces the caller to handle low confidence.

The repository is a research workspace (probes, spikes, a draft
`finfocus-spec` proposal) that has started to grow product code: `jevclient/`
(the configured Jev client and its error classification), `decision/` (the
validated `Probability`, `Thresholds`, `Levels` and `Spec` types, the sealed
`Result` and `ScoreResult`, and `Choose` and `Rate`), `eval/` (metrics over
labelled results) and the CLI in `internal/cli` and `cmd/jev-decide`, which
ships `ask`, `score` and `eval`.

## Technical Boundaries ("Hard No's")

- **No debate protocol.** Multi-agent adversarial debate stays in the
  `decide` skill. `jev-decide` only supplies a fast-path pre-screen.
- **No auto-approval.** Jev ranks well but is not calibrated (Brier 0.236).
  Output must never authorize an action without a human or a higher-cost
  path. Low confidence escalates; it is never silently defaulted. A decided
  result means clear enough to skip the debate, not approved.
- **No silent zero values.** A missing or failed answer is a typed error,
  never a zero probability or an empty choice.
- **No bare floats for probabilities.** Use the validated probability type.
- **No choice below the confident level.** Only a decided result exposes a
  choice to act on. An uncertain or escalate result exposes its leading option
  under a name that says it is not a decision, and a failure is an error, never
  a result.
- **No unvalidated spend.** Decision-spec input is validated before any API
  call is made.
- **No token in the repo.** `TYPESAFE_API_KEY` lives in the environment or
  an ignored `.env`. Never commit it.
- **No third-party gateways.** Talk to `https://api.typesafe.ai` (or an
  explicit `TYPESAFE_BASE_URL`). Do not depend on `openjevai/jev` or route
  data through unverified proxies.
- **No FinFocus core coupling.** FinFocus core forbids external API clients.
  Any recommendation scoring belongs in a plugin or behind a
  `finfocus-spec` RPC, not in this module.
- **No copied spike code.** Probe and spike tests are throwaway evidence;
  production code is rewritten.
- **No name collision.** `gojev` is taken (`taigrr/gojev`, `wawan93/gojev`).
  The published name is `jev-decide`.

## Data Source of Truth

- The Jev API contract: `docs/reference/typesafe-openapi.json`, refreshed
  from `https://api.typesafe.ai/openapi.json`.
- Empirical behavior: the dated probe and spike reports under `docs/`.
- Decision-spec and output schemas: versioned JSON schemas in this repo,
  exposed through `ax-go` `__schema` and pinned by golden tests.

## Interaction Model

- **Inbound:** CLI commands (`ask`, `score` and `eval`)
  and a Go library API (`Choose[T ~string]`, `Rate` and typed option sets).
  Input is a decision spec validated before use.
- **Outbound:** HTTPS `POST /v1/systemone` with bearer auth, through
  `kataras/jev` (pinned) built only by `jevclient.NewClient`. `GET /v1/models`
  is the only other endpoint.
- **Output:** versioned JSON with a sealed result type, chosen by the answer's
  confidence and two thresholds. Decided: at or above the confident level.
  Uncertain: from the floor up to the confident level, for a person. Escalate:
  below the floor, for the `decide` debate. The CLI exits 0, 10 and 11 for
  these three; failures use the `ax-go` codes 1 to 4. Both thresholds (0.9
  and 0.5) are placeholders until #6 tunes them.

## Verification

A proposed feature violates the boundaries if it:

1. Lets a caller act on a Jev answer without handling the uncertain and
   escalate cases.
2. Spends API budget before validating input.
3. Adds debate, persistence or orchestration logic that belongs in the
   `decide` skill.
4. Embeds a credential or a non-TypeSafe endpoint.
5. Changes an output schema without a version bump and golden test update.
6. Turns a failure into a result case, or lets a result that was never
   produced read as decided.
