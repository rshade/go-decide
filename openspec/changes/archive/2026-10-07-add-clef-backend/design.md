# Design

## Context

`jevclient.NewClient` returns a `*jev.Client` from `kataras/jev` (pinned). The
CLI reaches it through `Env.NewClient`, and `decision` and `eval` work on the
answers only. See `proposal.md` for why a second backend is wanted and
`docs/clef-2026-10-06.md` for the measured wire differences.

Two facts shape the approach:

- clef accepts the same `state` and `questions` bodies as System One. It
  differs in the URL path, a required `model`, the auth credential and a
  `{result, success, errors, messages}` envelope around the answers.
- `kataras/jev` encodes questions with a private `encoding/json/v2` encoder.
  `jev.Questions` cannot be marshalled from outside the package: `json.Marshal`
  emits Go field names and Cloudflare rejects it with code 5006.

## Goals / Non-Goals

**Goals:**

- A caller can pick `clef` with one flag and get the same typed results.
- The safety guarantees of `jevclient` hold for `clefclient` by construction,
  not by a second copy that can drift.
- Nothing silently falls back from one backend to the other.

**Non-Goals:**

- Automatic failover, voting or comparison across backends in one run.
- Tuned clef thresholds (issue #6).
- Any change to the question and answer types in `decision/` or to `eval/`.

## Decisions

### Adapt `jev.Client` with a `RoundTripper`

`clefclient.NewClient` builds a `*jev.Client` whose base URL is Cloudflare's
host and whose HTTP client has a `RoundTripper` that rewrites the path
`/v1/systemone` to the clef run path and replaces a 200 body with the
envelope's `result`. A non-success envelope (`success: false`) is turned into
an error response the classifier understands. This is the approach the probe
proved.

Alternatives:

- **Own HTTP client and encoder.** Needs a copy of jev's question encoding,
  which is private, so it would drift from the question types the whole
  library is built on.
- **Fork or patch jev.** Adds a maintenance burden for a path rewrite.
- **A new backend interface in `decision`.** Rejected: `decision` only sees
  answers today, and the sealed result types must not learn about vendors.

### Export the adapter, not just the client

`clefclient.NewTransport` and `clefclient.BaseURL` are public. A caller with its
own System One client (FinFocus's scorer plugin has one) points it at
`BaseURL(account)`, uses `NewTransport` as its HTTP transport and sends
`model: clef`, and needs no second client. `NewClient` is built from the same
two pieces, so there is one adaptation to keep right. The package already
imports from another module: Go's `internal` rule only restricts direct
imports. A consumer needs Go 1.27.1 and pulls `kataras/jev`, `ax-go` and
`x/time`. The transport holds no token and does not retry or follow
redirects, so the caller's client must refuse redirects itself.

### Share the client guarantees through `internal/clientkit`

Base URL validation, the no-redirect client, the redacting logger and the
opt-in response cache move from `jevclient` into `internal/clientkit`, with the
environment variable names as parameters. `jevclient` and `clefclient` both
build on it. The move changes no behaviour, and the existing `jevclient` tests
stay as they are to prove it. `clefclient` and `jevclient` still import only
`ax-go`'s `contract` package, and `deps_test.go` covers both.

Alternative: copy the files into `clefclient`. Faster, but the redirect and
redaction rules are security properties and two copies will diverge.

### Own error codes with a `clef.` prefix

`clefclient.Classify` maps Cloudflare failures to `clef.*` codes that mirror
the `jev.*` table, with the same exit codes, and carries Cloudflare's error
code and the `cf-ai-req-id` value in the context. Prefixes keep each code set
closed and each documented table complete (`docs/clef-errors.md`, checked by a
test like `jevclient/docs_test.go`). Renaming the `jev.*` codes to something
neutral would break existing consumers.

Only HTTP 400 with code 5006 has been observed so far. The 401, 403 and 429
mappings follow Cloudflare conventions and are confirmed against the live API
in the task that writes the error docs.

### Select the backend in the CLI, not in the library

`--backend` is read by `ask`, `score` and `eval`. `Env` keeps `NewClient` for
Jev and gains `NewClefClient`, so tests inject fakes for either. The CLI picks
the constructor and the classifier from the flag value and never tries the
other one. The backend name is validated before any request, as part of the
existing validate-before-spend rule, and also on a dry run.

### Let the caller choose how `decision` classifies failures

`decision.Choose` and `decision.Rate` call `jevclient.Classify` themselves, on
a failed call and on an invalid answer. With clef that would report Cloudflare
failures under `jev.*` codes. Both functions gain a `WithClassifier` option
(a `func(context.Context, error) error`) that defaults to
`jevclient.Classify`, so existing callers see no change and the CLI passes
`clefclient.Classify` for the clef backend. The classifier travels to the
internal helpers in the context, which keeps their signatures stable.

Alternatives: return unclassified errors and classify in the CLI (changes the
documented behaviour of the library), or have the CLI re-map `jev.*` codes to
`clef.*` (loses the Cloudflare fields).

### Output schema version 4

Every output gains a `backend` field, which is a shape change, so
`SchemaVersion` goes from 3 to 4 with a new `*.v4.json` golden set and the v3
files untouched. `__schema` lists the flag.

### Per-backend default thresholds live in the CLI

A small table maps backend to default thresholds. Both start at Jev's
placeholders (0.9 and 0.5), marked as placeholders in the docs, because the
probe is one run on 40 decisions and does not justify different numbers. The
table exists so #6 can tune each backend without a schema change.

### Live tests and CI

`clefclient/live_test.go` skips unless the Cloudflare variables are set, like
the Jev live tests. A `live.yml` workflow runs the live tests only on
`workflow_dispatch`, never on `pull_request`, using repository secrets. The
secret names are assumed to be `TYPESAFE_API_KEY`, `CLOUDFLARE_ACCOUNT_ID` and
`CLOUDFLARE_AUTH_TOKEN`; the listing API refused this token, so confirm them
in the repository settings. The unit suite never uses the network.

## Risks / Trade-offs

- [The adapter depends on jev's path and response decoding] → Pin jev, test
  the rewrite against `httptest`, and keep the probe as a live check.
- [clef answers vary more under option reordering on contested decisions] →
  Documented in the research note; the backend is opt-in and its thresholds
  are separate.
- [Cloudflare errors beyond 5006 are unobserved] → Provoke a bad-token call
  once during implementation and record the real status and code.
- [Moving code out of `jevclient` could change its guarantees] → The existing
  `jevclient` tests are the safety net and must pass unmodified.
- [Cloudflare bills in neurons, not tokens] → The cache and the dry run apply
  to clef the same way; spend reporting stays out of scope.

## Open Questions

- Whether Cloudflare 5xx statuses besides 429 are safe to retry. Left
  unretried until observed, which is the cautious default.
