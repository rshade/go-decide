# Proposal

## Why

Jev is the only model behind go-decide, so one vendor's outage, price or
behaviour change stops every decision. Cloudflare serves a second System One
model, `clef`, with the same question types and answer objects.
`docs/clef-2026-10-06.md` shows it solves the same 40-decision spike (20 of
20 on dominant decisions) with a weaker but usable confidence gate (AUC 0.93
against Jev's 0.98). Supporting it gives a fallback and a way to compare
models on the same questions.

## What Changes

- Add a `clefclient/` package that builds a client for Cloudflare's clef, with
  the same guarantees as `jevclient`: credentials only from the environment,
  https-or-loopback base URL, no redirects, bounded retries, secret redaction
  and answer validation.
- Add Cloudflare failure classification into `contract.Error` codes with a
  `clef.` prefix, documented in `docs/clef-errors.md`.
- Add a `--backend jev|clef` flag to `ask`, `score` and `eval`. The default
  stays `jev`.
- Report the backend in every output, which changes the output shape.
  **BREAKING** for consumers pinned to `schema_version` 3: the version becomes
  4, with new golden files.
- Give each backend its own default thresholds, so one backend's placeholders
  are not silently applied to the other.
- Amend `CONTEXT.md`: the "no third-party gateways" and "TypeSafe endpoint"
  rules name Cloudflare's first-party clef endpoint as the one permitted
  exception, and add the clef credentials to "no token in the repo".
- Add an opt-in, manually triggered CI workflow that runs the live tests with
  repository secrets.

## Capabilities

### New Capabilities

- `clef-client`: how a clef client is constructed from the environment, and
  its endpoint, redirect, retry and response-validation guarantees.
- `clef-error-mapping`: how Cloudflare failures become typed errors with
  stable `clef.*` codes.
- `backend-selection`: how the CLI chooses a backend, what it does when that
  backend's credentials are missing, and how defaults are kept per backend.

### Modified Capabilities

- `cli-output-contract`: outputs carry the backend, and the schema version
  moves to 4.

## Impact

- New code: `clefclient/`, backend selection in `internal/cli/`, and the
  shared client plumbing moved out of `jevclient/`.
- Changed: `internal/cli` outputs, golden files (new `*.v4.json` set),
  `internal/cli/deps_test.go`, `docs/jev-decide-cli.md`, `README.md`,
  `CONTEXT.md`, `CLAUDE.md`.
- `decision/` gains one option, `WithClassifier`, so a caller picks the error
  classifier; defaults are unchanged. `eval/` is unchanged: question and answer
  objects are the same.
- New environment: `CLOUDFLARE_ACCOUNT_ID`, `CLOUDFLARE_AUTH_TOKEN`, optional
  `CLOUDFLARE_BASE_URL`.
- Non-goals: automatic fallback from one backend to the other, mixing
  backends in one decision, and tuning the thresholds (issue #6).
