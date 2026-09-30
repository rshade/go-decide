# Jev Go client tracker

Community Go clients for TypeSafe AI's Jev (System One) API. None is
official; TypeSafe ships Python and JS SDKs only. Everything here is from a
2026-09-28 review of cloned source. Re-check before relying on it, since
every repo is under three weeks old.

Refresh live metadata (stars, tags, last push, proxy version):

```bash
./scripts/refresh-jev-clients.sh          # print
./scripts/refresh-jev-clients.sh --save   # also write docs/snapshots/<date>.tsv
```

The repo list lives in `scripts/jev-clients.txt`.

## API facts (from the official OpenAPI spec)

Spec copy: `docs/reference/typesafe-openapi.json` (OpenAPI 3.1, v0.2.0,
fetched 2026-09-28 from `https://api.typesafe.ai/openapi.json`).

- Base URL `https://api.typesafe.ai`, HTTP bearer auth. Only two paths:
  `POST /v1/systemone` and `GET /v1/models`.
- Request: `{state, model, questions: {<id>: Question}}`, at least one
  question. `state` is a string, object or array.
- Question types: `noul` (yes/no), `choice` (up to 255 options), `score`
  (up to 10 ordered levels, indexes from 0). Each has `instructions` and
  optional `criteria`.
- Answers are keyed by question id. `noul` has no confidence field. `choice`
  and `score` return `probabilities` and `confidence`; `score` is a float
  that can fall between levels.
- Response: `{model, answers, usage: {input_tokens, output_tokens}}`.
- Model: `jev-1.13.0` is the only real model. `jev-latest` (default) and
  `jev-preview` are aliases. No "Kev" model on the hosted API.
- Errors: HTTP status plus JSON `{"detail": {"error_type", "message"}}`.
  Documented 401, 422, 429, 529. Header `x-typesafe-request-id`.
- Limits: 1,200 requests per minute, 250k tokens per second, 64k context,
  32k for `state` plus the longest question. Input $0.042 per Mtok, output
  free. Limits change without notice.
- Official SDK env vars: `TYPESAFE_API_KEY`, `TYPESAFE_BASE_URL`,
  `TYPESAFE_DEFAULT_MODEL`, `TYPESAFE_LOG_LEVEL`.
- Keys come from `console.typesafe.ai/keys`. The Master Customer Agreement
  has not been read for third-party client restrictions.

Discrepancies to confirm with a token: an unauthenticated POST returned 403
though the docs list 401, and the docs disagree on default retry counts.

## Comparison

Scores are 1-5 impressions from one review, not benchmarks.

| Client | Kind | Go | Deps | API | Tests | Docs | Maint. |
| --- | --- | --- | --- | --- | --- | --- | --- |
| kataras/jev | library | 1.27 | 1 | 5 | 4 | 5 | 3 |
| anilsenay/jev | library | 1.22 | 0 | 5 | 4 | 5 | 3 |
| Gaurav-Gosain/jev-go | library | 1.27.1 | 0 | 5 | 4 | 4 | 2 |
| mattn/go-jev | library + CLI | 1.27 | 0 | 3 | 2 | 4 | 3 |
| stefafafan/jev | CLI only | 1.27 | 0 | n/a | 4 | 4 | 4 |
| taigrr/gojev | harness + CLI | 1.27.1 | 95 | 4 | 2 | 4 | 2 |
| wawan93/gojev | library | 1.21 | 0 | 3 | 2 | 2 | 2 |
| openjevai/jev | fork of kataras | 1.27 | 1 | n/a | n/a | n/a | 1 |

### kataras/jev

`github.com/kataras/jev`, MIT, v0.1.0, 1 commit, 5 stars, about 4k lines,
36 tests.

- Most complete library. Options for base URL, model, timeout, retry, HTTP
  client, headers and logger. `SystemOne`, generic `SystemOneAs[T]`,
  `ListModels`, question types `Noul`, `Choice`, `Score` and `Raw`.
- Rate limiter (`golang.org/x/time`), typed errors, retries, redacted
  headers in logs, no redirect following. Sends `X-TypeSafe-SDK` and
  `X-TypeSafe-Runtime` headers.
- Reads `TYPESAFE_API_KEY`, `TYPESAFE_BASE_URL`, `TYPESAFE_DEFAULT_MODEL`.
- Weakness: a single commit and single author. No streaming found.

### anilsenay/jev

`github.com/anilsenay/jev`, MIT, v0.1.0, 8 commits, 2 stars, about 3.2k
lines, 42 tests including a parity test against the official SDK shape.

- Generic `jev.Ask[A]` with `Choice[T ~string]` returning your own enum,
  plus `Batch`, `Handle[A]`, `ListModels` via `Models()`, and a `RawQuestion`
  escape hatch. Answers are validated on decode.
- Middleware, cache, logger and provider options. `jevtest` fake package for
  testing. CI runs gofmt, vet and race tests on Go 1.22 and stable.
- `jev.New()` reads `TYPESAFE_API_KEY`; `NewWithKey` takes a key.
- Best testability of the set and the lowest Go version among the modern
  ones.

### Gaurav-Gosain/jev-go

`github.com/Gaurav-Gosain/jev-go`, MIT, v0.1.0, 1 commit, 6 stars, 12
tests.

- Sealed `Question` interface with `YesNo`, `OneOf` and `Levels`
  constructors. Typed answers with `ErrWrongKind` and `ErrNoAnswer`.
  Configurable retry with jitter. Generic `Batch[T]`.
- `New()` returns `ErrNoAPIKey` when `TYPESAFE_API_KEY` is unset.
- Weaknesses: one commit, `WithTimeout` mutates the shared HTTP client so
  option order matters, batching looks over-built for v0.1.0.

### mattn/go-jev

`github.com/mattn/go-jev`, MIT, v0.0.3, 8 commits, 40 stars (highest), 5
tests.

- Small library plus a `jev-cli`. Retries only 429 and 529, no jitter. Also
  supports tensai-compatible servers through `WithURL`.
- Loose types: `Question.Type` is a raw string and most inputs are `any`. A
  missing key sends no auth header and fails at 401.
- The library does not read env vars; only the CLI reads `TYPESAFE_API_KEY`.

### stefafafan/jev

`github.com/stefafafan/jev`, MIT, v0.1.2, 17 commits, 6 stars, 35 tests.

- CLI only, code under `internal/`, so it is not importable.
- Providers: TypeSafe, Cloudflare and Vercel AI Gateway, selected with
  `JEV_PROVIDER`. CI plus Renovate. Useful as a reference for multi-provider
  design.

### taigrr/gojev

`github.com/taigrr/gojev`, 0BSD, v0.1.0, 2 commits, 1 star, 9 tests.

- A harness for agent gating, not a general SDK. Routes to TypeSafe, Vercel
  AI Gateway (`typesafe-ai/jev`) or a local open-weight Kev model run
  in-process through llama.cpp (weights checked against pinned SHA-256).
- Helpers: `Classify[T]`, `AskBool`, `Rate`, `Fallback(cloud, localKev)`,
  `Above(thresh, margin)`. Protocol code lives in personal forks
  (`taigrr/fantasy`, `taigrr/catwalk`), not this repo.
- 95 `go.mod` requires including indirect. Heavy for a client.
- The name collides with this project (`gojev`).

### wawan93/gojev

`github.com/wawan93/gojev`, MIT, v0.1.0, 2 commits, 0 stars, 5 tests, no
CI.

- Fluent builder: `client.NewSystemOne(state).Choice(...).Noul(...).Do(ctx)`.
  Typed error structs per status and a `RetryPolicy`. Mirrors the shape of
  the official Python and TS SDKs.
- Questions are `map[string]any`, so answers are stringly typed. Thin
  README. Also collides with this project's name.

### openjevai/jev (avoid)

Two-day-old fork of `kataras/jev` with no releases. Adds a `WithProvider`
option that can send your state and questions to a third-party gateway,
`api.openjev.sh`. It auto-selects that gateway when only `OPENJEV_API_KEY`
is set. The operator, logging and retention are unverified. Prefer
`kataras/jev` directly.

## Other Go names seen, not reviewed

These appeared in registry searches only:
`Stumble/jev-go`, `devbackend/jevgo`, `drpaneas/jev`, `mhmdkzr/jev`,
`leonardjke/go-jev`, `kazz187/jev-sdk-go`, `kyledickey/jev-go`,
`draganm/go-jev`, `havlan/jev-go`, `ajayk/jev-go-sdk`,
`mheers/typesafeai-systemone-jev-go`, `HomayoonAlimohammadi/jev-sdk-go`,
`danninx/go-jev`, `withzombies/jev-go`, `shanehull/go-jev`. Not Go clients:
`BorisLeMeec/jev` (Claude Code plugin), `arnobroekhof/jev` (unrelated JSON
compare tool).

## Once you have an API token

Tested so far with `kataras/jev` only; see `docs/probe-2026-09-28.md` and
`docs/probe-round2-2026-09-28.md`. Boxes
ticked below are verified for that client. The other three still need the
same run (`jev_recommendations_test.go` is written against `kataras/jev`).

Run these against each shortlisted client (`kataras/jev`, `anilsenay/jev`,
`Gaurav-Gosain/jev-go`, `mattn/go-jev`) with the same inputs. Keep the token
in `.env`, which is gitignored, never in code.

- [x] `GET /v1/models` works and matches the client's model handling.
- [x] One `noul`, one `choice` and one `score` question round-trip and the
  answers decode correctly, including `probabilities` and `confidence`.
- [x] `score` returns a fractional value between levels and the client keeps
  it as a float.
- [x] `state` as string, object and array all serialize correctly (prose
  string, object and ordered list all used in round 2).
- [x] Bad key: which status comes back (401 or 403) and whether the client
  maps it to a useful error. Invalid key gives 401; the earlier 403 was a
  request with no auth header at all.
- [ ] Invalid request (no questions, more than 10 score levels): 422 body is
  surfaced. Covered offline by the `jevclient` tests; not observed live.
- [ ] Rate limit or 529 handling: does retry honour `retry-after`. A burst of
  120 requests at concurrency 40 produced no 429s (the server queues, about
  21 requests per second), so the retry path was not exercised live. The
  `jevclient` tests cover it against a synthetic server only.
- [x] Request ID (`x-typesafe-request-id`) is exposed on errors.
- [ ] Context cancellation and timeout stop the call promptly. Covered offline
  by the `jevclient` tests; not observed live.
- [x] Measure latency against the documented 70-500 ms.
- [ ] Confirm the token terms in the Master Customer Agreement allow a
  published third-party client.

## Decision (2026-09-29)

`kataras/jev` is adopted, pinned at v0.1.0, and is the only client jev-decide
uses. It closes the client spike (#11) and reshapes #1: the module does not
write a client, it configures this one through `jevclient.NewClient` and
classifies failures through `jevclient.Classify` (see
[jev-errors.md](jev-errors.md)).

Why: it scored highest on the survey (tied with `anilsenay/jev` on paper) and
is the only client run live, and it already rejects every malformed 2xx
response (missing answer, wrong type, value outside 0 to 1), refuses
redirects and exposes the request ID.

What the adapter adds, because the dependency's defaults do not fit:

- The base URL must be `https`, or `http` to a loopback IP address. The
  dependency accepts `http` for any host.
- Only 429 and 529 are retried, at most twice. The dependency's default also
  retries 408, every 5xx, timeouts and connection failures, with no time
  budget.
- `error_type` from the `{"detail": {...}}` body is kept. The dependency
  discards it.
- The API key is redacted from logs and classified errors. The dependency
  redacts request headers only, so an echoed key would reach debug logs.

Risks to keep in view:

- One commit, 5 stars, v0.1.0, maintainer score 3 in the survey. Pin the exact
  version and read the diff on every bump. The adapter is two functions, so
  replacing it stays a one-package change.
- Whether TypeSafe bills a request that returns 429 or 529 is unconfirmed. The
  retry policy assumes it does not. The retry list is one constant in
  `jevclient`.
- `anilsenay/jev`, `Gaurav-Gosain/jev-go` and `mattn/go-jev` were never run
  against the API, so "best client" rests on `kataras/jev` being the tested
  one, not on a measured comparison.
- The token terms in the Master Customer Agreement are still unread.
