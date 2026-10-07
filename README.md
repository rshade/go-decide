# go-decide

Ask a System One model whether a decision is clear enough to act on. Jev, from
TypeSafe AI, is the default model, and Cloudflare's clef is an opt-in second
backend.

`go-decide` puts a choice (or an ordered rubric) to the model and prints a JSON
outcome. The exit code tells your script what to do next: act, ask a person, or
escalate. It gates and labels decisions. It never approves them.

The project is in early development and has no tagged release yet. See
[CONTEXT.md](CONTEXT.md) for boundaries and [ROADMAP.md](ROADMAP.md) for plans.

## Install

```sh
go install github.com/rshade/go-decide/cmd/go-decide@latest
```

Until v0.1.0 is tagged, this builds the latest commit and `go-decide --version`
prints a pseudo-version. Release archives for linux, darwin and windows on
amd64 and arm64, with a `checksums.txt`, will appear on the GitHub releases
page once a release is published. There is no Docker image, Homebrew tap or
deb/rpm package yet.

## Set your API key

Create a key at `console.typesafe.ai/keys` and export it:

```sh
export TYPESAFE_API_KEY="your-key"
```

The key is read only from the environment, never from a flag. Keep it out of
version control. A `.env` file is fine as long as it is ignored.

To use clef instead, pass `--backend clef` and export a Cloudflare API token
with Workers AI access and your 32-character account ID. clef also needs
`--instructions` (or `"instructions"` in a spec): without it, `ask` and `score`
exit 2 before sending anything.

```sh
export CLOUDFLARE_AUTH_TOKEN="your-token"
export CLOUDFLARE_ACCOUNT_ID="your-account-id"
```

A run uses one backend and only that backend's credentials. It never falls
back to the other one.

## Make your first decision

Put a question, the facts, and the options to the model:

```sh
go-decide ask \
  --state "All 412 tests passed and the security scan is clean." \
  --instructions "Is this build ready to release?" \
  --option ship="safe to release" \
  --option hold="needs more work"
```

You get one JSON envelope on standard output. The values below are
illustrative; yours will differ:

```json
{
  "data": {
    "schema_version": 4,
    "backend": "jev",
    "outcome": "decided",
    "choice": "ship",
    "confidence": 0.93,
    "probabilities": {"hold": 0.03, "ship": 0.97},
    "thresholds": {"floor": 0.5, "confident": 0.9}
  },
  "meta": {"trace_id": "...", "span_id": "...", "idempotency_key": "..."}
}
```

The `outcome` is one of three values, chosen by comparing `confidence` with
the thresholds:

| Outcome | Confidence | Exit code | What to do |
| --- | --- | --- | --- |
| `decided` | at or above `0.9` | 0 | The answer is in `choice`. |
| `uncertain` | from `0.5` up to `0.9` | 10 | Ask a person. `leading` names the front-runner. |
| `escalate` | below `0.5` | 11 | Take it to the full `decide` debate. |

`confidence` comes from the model and is reported apart from `probabilities`,
so it need not equal the top probability. Run the example a few times: a
question this clear-cut can still land on `uncertain` (see
[Know the limits](#know-the-limits)).

Only a `decided` outcome has a `choice`. `uncertain` and `escalate` are not
failures: the full result is still printed.

To check a question without spending anything, add `--dry-run`. It validates
your input and stops before any request, so it needs no key:

```sh
go-decide ask --dry-run --state "All 412 tests passed." \
  --instructions "Ship it?" \
  --option ship="safe to release" --option hold="wait"
```

## Use it in a script

Branch on the exit code. This example uses `jq` to read the answer:

```sh
out=$(go-decide ask --spec decision.json)
case $? in
  0)  echo "go ahead with $(jq -r .data.choice <<<"$out")" ;;
  10) echo "ask a person, leaning $(jq -r .data.leading <<<"$out")" ;;
  11) echo "escalate to the full decide debate" ;;
  *)  echo "go-decide failed" >&2; exit 1 ;;
esac
```

`decision.json` is a decision spec, one JSON document:

```json
{
  "state": "The build passed all 412 tests.",
  "instructions": "Is this build ready to release?",
  "options": [
    {"name": "ship", "description": "Release it now."},
    {"name": "hold", "description": "Needs more work."}
  ]
}
```

Pass `--spec -` to read it from standard input. Flags can fill in a spec, but
giving the same field both ways is an error.

A failure prints an error envelope on standard error and leaves standard output
empty. Exit codes 1 to 4 mean failure:

| Code | Meaning |
| --- | --- |
| 1 | Internal failure, a response that broke the contract, or a missing API key. |
| 2 | Invalid input. The error names the field. |
| 3 | Network failure or timeout. |
| 4 | Authentication failure. |

## Other commands

`score` rates something against an ordered rubric, lowest level first, and
prints a `level` instead of a `choice`:

```sh
go-decide score --state "checkout fails" \
  --instructions "How severe is this bug?" \
  --level minor="cosmetic" --level major="cannot buy"
```

`eval` runs a labelled set of decisions through the chosen backend and reports
how well confidence separates clear decisions from contested ones. Use it to
check the thresholds:

```sh
go-decide eval --decisions testdata/decisions.json \
  --truth testdata/decisions_truth.json --cache-dir .eval-cache
```

All three commands accept `--backend` and `--dry-run`.
`go-decide <command> --help` lists every flag.

## Know the limits

- **Decided does not mean approved.** The model ranks options well but its
  confidence is not calibrated. Treat `decided` as "clear enough to skip a
  second look", not as sign-off. In one run of the 40-decision probe at the
  default thresholds, 2 of Jev's 20 `decided` outcomes and 4 of clef's 24
  were not safe to fast-path.
- **The thresholds are placeholders.** The floor of 0.5 and confident level of
  0.9 stay provisional until issue #6 tunes them on real decisions. A clear-cut
  question can land at `uncertain` under these defaults. Set `--floor` and
  `--confident` to tune them, and use `eval` to check your choice.
- **Backends differ.** On the 40-decision probe, clef separated contested
  decisions less sharply than Jev (AUC 0.94 against 0.98 in one run). Both
  backends start from the same 0.5 and 0.9 placeholders, but each is meant to
  be tuned on its own, so do not carry thresholds tuned for one backend over
  to the other.
- **Answers vary.** Identical calls can return slightly different
  `confidence` and `probabilities`.
- **Calls cost money.** Validate with `--dry-run` first. `eval` can cache
  responses with `--cache-dir` so a rerun sends nothing.

## Learn more

- [docs/jev-decide-cli.md](docs/jev-decide-cli.md): the full CLI reference,
  covering backends, the spec format, every output field, exit codes and output
  versioning.
- [docs/jev-errors.md](docs/jev-errors.md) and
  [docs/clef-errors.md](docs/clef-errors.md): the error codes behind exit codes
  1 to 4.
- [docs/clef-2026-10-06.md](docs/clef-2026-10-06.md): how clef compares with
  Jev on the same decisions.
- [CONTEXT.md](CONTEXT.md): what this project will and won't do.
- [ROADMAP.md](ROADMAP.md): planned work, mapped to issues.

Using go-decide as a Go library is not documented here yet.
