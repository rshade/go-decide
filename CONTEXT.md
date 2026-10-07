# go-decide Context & Boundaries

## Core Architectural Identity

`go-decide` is a thin, strongly typed Go client and CLI for System One models,
aimed at fast-path decision support. It asks a System One model, Jev by default
or Cloudflare's clef with `--backend clef`. It provides `ask`, `score` and
`eval` commands built on `ax-go`, plus an optional pre-screen for the `decide`
skill. It turns Jev's `choice`, `score` and `noul` answers into values callers
cannot misuse: typed option sets, validated probabilities and a result that
forces the caller to handle low confidence.

The repository is a research workspace (probes, spikes, a draft
`finfocus-spec` proposal) that has started to grow product code: `jevclient/`
(the configured Jev client and its error classification), `clefclient/` (the
same for Cloudflare's clef), `decision/` (the
validated `Probability`, `Thresholds`, `Levels` and `Spec` types, the sealed
`Result` and `ScoreResult`, and `Choose` and `Rate`), `eval/` (metrics over
labelled results) and the CLI in `internal/cli` and `cmd/go-decide`, which
ships `ask`, `score` and `eval`.

## Technical Boundaries ("Hard No's")

- **No debate protocol.** Multi-agent adversarial debate stays in the
  `decide` skill. `go-decide` only supplies a fast-path pre-screen.
- **No auto-approval.** Jev ranks well but is not calibrated (Brier 0.236),
  and clef separates contested decisions less sharply.
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
- **No token in the repo.** `TYPESAFE_API_KEY`, `CLOUDFLARE_AUTH_TOKEN` and
  `CLOUDFLARE_ACCOUNT_ID` live in the environment or an ignored `.env`. Never
  commit them.
- **No third-party gateways.** Talk to `https://api.typesafe.ai` (or an
  explicit `TYPESAFE_BASE_URL`) for Jev, and to
  `https://api.cloudflare.com` (or an explicit `CLOUDFLARE_BASE_URL`) for
  clef. Cloudflare is the one other permitted host because it serves the clef
  model itself, not as a proxy. Do not depend on `openjevai/jev` or route data
  through unverified proxies.
- **No silent fallback between backends.** A run uses the backend it was
  asked for and only that backend's credentials. A failure there is an error,
  never a retry against the other model.
- **No FinFocus core coupling.** FinFocus core forbids external API clients.
  Any recommendation scoring belongs in a plugin or behind a
  `finfocus-spec` RPC, not in this module.
- **No copied spike code.** Probe and spike tests are throwaway evidence;
  production code is rewritten.
- **No name collision.** `gojev` is taken (`taigrr/gojev`, `wawan93/gojev`).
  Bare `decide` is the debate skill this tool escalates to. The published
  name is `go-decide`.

## Data Source of Truth

- The Jev API contract: `docs/reference/typesafe-openapi.json`, refreshed
  from `https://api.typesafe.ai/openapi.json`.
- Empirical behavior: the dated probe and spike reports under `docs/`.
- Decision-spec and output schemas: versioned JSON schemas in this repo,
  exposed through `ax-go` `__schema` and pinned by golden tests.

## Interaction Model

- **Inbound:** CLI commands (`ask`, `score` and `eval`), the same `ask` and
  `score` as MCP tools through `mcp-server`,
  and a Go library API (`Choose[T ~string]`, `Rate` and typed option sets).
  Input is a decision spec validated before use.
- **Outbound:** for Jev, HTTPS `POST /v1/systemone` with bearer auth, through
  `kataras/jev` (pinned) built only by `jevclient.NewClient`. `GET /v1/models`
  is the only other endpoint. For clef, HTTPS `POST
  /client/v4/accounts/{id}/ai/run/@cf/cloudflare/clef` with bearer auth,
  through the same pinned client, built only by `clefclient.NewClient`.
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
4. Embeds a credential or an endpoint other than TypeSafe's or Cloudflare's.
5. Changes an output schema without a version bump and golden test update.
6. Falls back from one backend to the other.
7. Turns a failure into a result case, or lets a result that was never
   produced read as decided.
