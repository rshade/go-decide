# Design

## Context

See `proposal.md` for the motivation. The measurement already exists as
throwaway code. `jev_decide_spike_test.go` builds the question, and
`probe_harness_test.go` caches responses under `.probe-cache/`, keyed on a
hash of the request. `CONTEXT.md` forbids copying that code, so this change
rewrites it.

Constraints that shape the approach:

- `decision` and `jevclient` may import only `ax-go`'s `contract` package,
  which `internal/cli/deps_test.go` enforces.
- There is one `SchemaVersion` constant for every command. The golden tests
  require a file for every case at every version from 1 to the current one.
- `jevclient.NewClient` builds the client through `kataras/jev`. That library
  supplies its own `http.Client`, whose `CheckRedirect` returns
  `http.ErrUseLastResponse`, unless `jev.WithHTTPClient` replaces it. Retries
  run inside `jev.Client`, above the HTTP client.
- `cli.Env.NewClient` is `func() (*jev.Client, error)`, and the CLI tests
  inject an `httptest` server through it.

## Goals / Non-Goals

**Goals:**

- A metrics library with no I/O, so every number in the report is
  unit-tested on hand-built inputs.
- One validation pass over both files before the first request.
- A cache that sits below everything else in the client, so it cannot change
  what a call returns, only whether the call reaches the network.

**Non-Goals:**

- Running requests concurrently. 40 sequential calls take seconds, and running
  them in order keeps the fail-fast and cache behavior easy to reason about.
- Expiring or invalidating cache entries. Deleting the directory is the reset.
- Gating CI on metric values, such as a minimum AUC.

## Decisions

### A new `eval` package holds the metrics

`eval` exposes a labelled-result constructor and a `Compute` function that
returns a report. A metric that can be undefined (accuracy, AUC, precision,
recall) has an optional type, so absence cannot be confused with zero. The
package imports `decision` for `Probability` and `Thresholds`. It applies the
same rule as `decision`'s unexported `classify` through the exported
`Probability.AtLeast` and the threshold accessors: decided at or above the
confident level, uncertain at or above the floor, escalate below it. Tests pin
the boundaries. Exporting `classify` would add `decision` API that no spec
asks for. It is added to the
`deps_test.go` list.

- *Alternative: compute inside `internal/cli`.* Rejected because #6 needs the
  same numbers from Go, and because pure functions in their own package are
  the easiest thing in the repo to test.
- *Alternative: put it in `decision`.* Rejected because `decision` is the
  per-call domain, while this is analysis over many calls.

### File loading stays in the CLI

The two file formats belong to the command, not the library.
`internal/cli/evalinput.go` parses both files. It checks ids, classes and
correct options, then builds each question through `decision.Spec{State,
Instructions, Options}.Question()`, so a decision passes or fails by exactly
the rules `ask` applies. The state is a map of `title`, `context`,
`constraints` and `options`. Each option is a `json.RawMessage`, so fields such
as `pros` and `cost_usd_per_month` pass through unchanged. A failure is the
same validation envelope `ask` prints (exit 2, with a `field` in its context)
named `decisions[<id>].<field>` or `truth[<id>].<field>`. `decision.FieldError`
cannot be built outside its package, so the loader builds the envelope
directly, and a `FieldError` from `Spec.Question()` gets the decision id
prefixed onto its field. Both files are decoded with unknown fields disallowed
at the entry level, following the archived ask/score decision that an input
format rejects what it does not define so that it can grow. Only the option
objects are free-form, because they are content sent to Jev, not input the
command interprets.

### The cache is an `http.RoundTripper` behind a `jevclient` option

`jevclient.WithResponseCache(dir string)` makes `NewClient` pass
`jev.WithHTTPClient` a client that does two things. Its transport serves from
and fills the cache in front of `http.DefaultTransport`. Its `CheckRedirect`
returns `http.ErrUseLastResponse`, which keeps the redirect refusal. The key is
the SHA-256 of the method, scheme, host, path and request body. Headers are
left out, so the bearer token never reaches the key or the entry. An entry
stores the status, the `Content-Type` and the body. Only a 200 whose `answers`
has a key for every name in the request's `questions` is written. Any other
response breaks the API contract, and replaying it would make every rerun fail
the same way. Checking each answer's content stays with the client. The
transport reads the body and then restores it with a fresh reader (and
`GetBody`), so the real request is unchanged. Files are written to a temporary
name and then renamed, so a crash leaves a missing entry rather than a
truncated one.

- *Alternative: cache in `decision.Choose`, keyed on the question.* Rejected
  because the request body is the true identity of a call. A key on the
  question would miss changes in how `kataras/jev` serializes it, and would
  serve a stale answer after an upgrade.
- *Alternative: cache in `internal/cli` only.* Rejected because a transport
  belongs with the other transport guarantees in `jevclient`, where its tests
  can sit next to the redirect and retry tests.

Because the cache sits below `jev`'s retry loop, a 429 or 529 is never stored,
and the retried success is.

### `Env.NewClient` takes options

`Env.NewClient` becomes `func(...jevclient.Option) (*jev.Client, error)`, so
`eval` can pass the cache option and the tests can still inject a server.
`ask` and `score` call it with no options, so their behavior is unchanged.

### Fail fast, sequentially

Decisions are asked in file order. The first failure returns through
`jevclient.Classify` (or `failure` for an unusable answer), wrapped with the
decision id, and no report is printed. Every response before it has already
been cached by the transport, which is what makes a rerun cheap.

### Schema version 2 and golden cases with a starting version

`SchemaVersion` becomes 2. Every existing case gets a `.v2.json` golden, and
the `.v1.json` files are left untouched. Each golden case gains a `since`
version, which is 1 for the existing cases and 2 for `eval.report` and
`eval.dryrun`. The "earlier versions are kept" check then loops from `since`
rather than from 1. This is the only change to the golden harness.

### The report shape

`EvalOutput` holds:

- `schema_version`
- `metrics`: `accuracy`, `contested_auc` and `brier`, each a number or
  `null`
- `outcomes`: `decided`, `uncertain`, `escalate` and `decided_unsafe`
- `thresholds`
- `by_threshold`: one row per threshold, with `threshold`, `selected`,
  `safe`, `unsafe`, `precision` and `recall`
- `decisions`: one row per decision, with `id`, `class`, `pick`,
  `confidence`, `correct_option`, `safe` and `outcome`

The per-decision field is named `pick`, not `choice`, so it never reads as a
decided `choice` from `ask`. Fields derived from confidence are tagged
`ax:"nondeterministic"`. `EvalDryRunOutput` holds `schema_version`,
`dry_run`, `decisions`, `dominant`, `contested` and `thresholds`.

## Risks / Trade-offs

- [The model behind the API changes while the cached answers stay the same]
  → The cache is opt-in and keyed on the request only. The docs say to clear
  the directory to re-measure after a model change.
- [Replacing `jev`'s HTTP client drops a default it sets] → The only default
  is `CheckRedirect`, which the cache client sets as well. A test proves a
  redirect is still refused with the cache on.
- [The bundled set is synthetic, so good numbers mislead] → The docs and
  `ROADMAP.md` say the bundled set is a smoke test, and that #6 needs real
  past decisions.
- [Moving every command to `schema_version` 2 surprises a consumer pinned to
  1] → There are no external consumers before v0.1.0. The bump is the
  documented signal.
- [A run with many decisions spends more than expected] → Dry run reports the
  count first, and every call is still one small choice question.

## Migration Plan

No migration is needed. v1 golden files stay frozen. Rollback means reverting
the commit, which also restores `SchemaVersion` 1.

## Open Questions

- Should the per-threshold grid be configurable (`--grid 0.8:0.99:0.01`)?
  The fixed 0.05 grid plus the run's thresholds answers #5, and #6 can add a
  flag without a shape change, since the rows are already a list.
