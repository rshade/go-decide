<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Decision log

go-decide keeps a log of each decision as it stood before anyone knew the
answer, the path its run took, and how it turned out weeks later. The skill
writes it by calling the record tools at fixed points. Labelled decisions are
scored with `go-decide eval --log`, which measures whether the fast path holds
on real decisions.

## Recording is opt-in

Real decisions can be sensitive, so go-decide records nothing unless its own
`DECIDE_LOG` environment variable names the log file. For the MCP server,
that is the server's environment:

```sh
export DECIDE_LOG="${XDG_STATE_HOME:-$HOME/.local/state}/decide/decisions.jsonl"
claude mcp add go-decide -e DECIDE_LOG="$DECIDE_LOG" -- go-decide mcp-server
```

- A result with `recording: false` means the variable is unset. Nothing was
  written. Say nothing about recording during the run.
- No tool input names the file. Never try to pass a path.
- A log inside a git working tree fails with a validation error naming
  `--yes`. Tell the user the log could be committed with that repository,
  and pass `yes` only after they confirm.
- A validation error naming `DECIDE_LOG` that says the log is larger than
  its limit means the log is full. Tell the user to point `DECIDE_LOG` at a
  new file; nothing was written.
- Skip recording, and say no record was written, when the user asks not to
  record this decision.

Prefer the MCP tools. Without them, run the same subcommands of the
`go-decide` binary, for example `go-decide record decision --title ...`.

## What to record, and when

| When | Tool | Inputs |
| --- | --- | --- |
| The brief is settled, before the pre-screen or Round 1 | `go-decide-record-decision` | `title` (the one-line decision), `context`, one `constraint` per hard constraint, `instructions` (the question the pre-screen asks), and one `option` per option as `name=description`, the same list the pre-screen sends. |
| The pre-screen decided and the debate is skipped | `go-decide-record-run` | `id`, `path: fast`, `pre-screen`: the `ask` envelope's `data`, whole, as a JSON string. |
| The user stopped after an uncertain result | `go-decide-record-run` | `id`, `path: stopped`, `pre-screen`. |
| The pre-screen call failed and the user stopped | `go-decide-record-run` | `id`, `path: stopped`, `pre-screen-error`: the error code. |
| Phase 3 presented the consensus | `go-decide-record-run` | `id`, `path: debate`, one of `pre-screen`, `pre-screen-error` or `pre-screen-skipped` (why it did not run), `recommendation`, `evidence-mode` when go-decide was available, `go-decide-calls`, and one `citation` per cited result. |

`record decision` returns the `id`; tell the user the id and the `log` path in
one line. A debate the user starts after a `fast` or `stopped` run gets a
second `record run` with `path: debate`. The first one stays.

Each citation is a JSON string with exactly these fields, taken from the
agent's citation block in `references/debate-prompts.md`: `agent`
(`advocate_a`, `advocate_b` or `moderator`), `round` (1 or 2), `backend`,
`outcome`, `result` (the choice or leading option), `confidence`, `asked` (the
instructions sent, verbatim), and `options` (every option or level sent, each
a `name` and its `probability`).

Every field of a citation, and every field `ask` prints in `pre-screen`, is
required. go-decide refuses one that is missing or null rather than recording
a zero the model never reported.

Never put a credential, an API key or an environment value in any input.

## Record format

go-decide writes one JSON object per line and never rewrites a line. Each
line has `v` (format version 1), `kind`, `id`, `recorded_at` (UTC) and the
recording call's `trace_id`.

| Kind | Also holds |
| --- | --- |
| `decision` | `instructions`, and `decision`: `eval`'s decision shape (`id`, `title`, `context`, `constraints`, `options`). Nothing a run produced. |
| `run` | `path`, `pre_screen` (the envelope data, `{"error": code}` or `{"skipped": reason}`), and for a debate `debate`: `recommendation`, `evidence_mode`, `go_decide_calls`, `citations`. |
| `outcome` | `truth`: `eval`'s truth entry (`id`, `class`, `correct_option`, `why`). The latest outcome for an id wins. |

<!-- markdownlint-disable MD013 -->
```jsonl
{"v":1,"kind":"decision","id":"2026-10-12-a1b2","recorded_at":"2026-10-12T14:03:00Z","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736","instructions":"Which cache should the service use?","decision":{"id":"2026-10-12-a1b2","title":"Cache backend for the pricing service","context":"Reads dominate, and the service already runs next to a managed Postgres.","constraints":["No new service to operate","Ship this quarter"],"options":[{"name":"postgres","description":"Cache in an unlogged table in the existing Postgres"},{"name":"sqlite","description":"Cache in a local SQLite file on each instance"}]}}
{"v":1,"kind":"run","id":"2026-10-12-a1b2","recorded_at":"2026-10-12T14:40:00Z","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736","path":"debate","pre_screen":{"schema_version":7,"backend":"jev","outcome":"escalate","leading":"postgres","confidence":0.41,"probabilities":{"postgres":0.52,"sqlite":0.48},"thresholds":{"floor":0.5,"confident":0.9}},"debate":{"recommendation":"Use Postgres, and revisit if cache reads exceed its connection budget.","evidence_mode":"blind","go_decide_calls":1,"citations":[{"agent":"advocate_b","round":2,"backend":"jev","outcome":"decided","result":"sqlite","confidence":0.95,"asked":"Which cache keeps latency lowest?","options":[{"name":"sqlite","probability":0.95},{"name":"postgres","probability":0.05}]}]}}
{"v":1,"kind":"decision","id":"2026-10-13-c3d4","recorded_at":"2026-10-13T09:15:00Z","trace_id":"00000000000000000000000000000000","instructions":"Which runner setup should CI use?","decision":{"id":"2026-10-13-c3d4","title":"Split the test suite or buy bigger runners","context":"CI takes 25 minutes, and most of it is one integration package.","constraints":["Budget under 200 USD a month"],"options":[{"name":"split","description":"Split the suite across parallel jobs"},{"name":"bigger_runners","description":"Move to larger hosted runners"}]}}
{"v":1,"kind":"run","id":"2026-10-13-c3d4","recorded_at":"2026-10-13T09:16:00Z","trace_id":"00000000000000000000000000000000","path":"stopped","pre_screen":{"error":"jev.rate_limited"}}
{"v":1,"kind":"decision","id":"2026-11-02-e5f6","recorded_at":"2026-11-02T16:20:00Z","trace_id":"00000000000000000000000000000000","instructions":"Which docs site should the project use?","decision":{"id":"2026-11-02-e5f6","title":"Keep MkDocs or switch the docs to Starlight","context":"The docs are 40 pages of Markdown with a few diagrams.","constraints":[],"options":[{"name":"mkdocs","description":"Keep the MkDocs site"},{"name":"starlight","description":"Move to Starlight"}]}}
{"v":1,"kind":"outcome","id":"2026-10-12-a1b2","recorded_at":"2026-11-20T10:00:00Z","trace_id":"00000000000000000000000000000000","truth":{"id":"2026-10-12-a1b2","class":"dominant","correct_option":"postgres","why":"Six weeks in, the unlogged table held up and nobody had to run anything new."}}
{"v":1,"kind":"outcome","id":"2026-10-13-c3d4","recorded_at":"2026-11-20T10:05:00Z","trace_id":"00000000000000000000000000000000","truth":{"id":"2026-10-13-c3d4","class":"contested","correct_option":null,"why":"We split the suite and it helped, but bigger runners would have too."}}
```
<!-- markdownlint-enable MD013 -->

## Label past decisions

Run this when the user asks to review or label the log.

1. Call `go-decide-record-pending`, with `older-than-days` when the user
   names a threshold (30 by default). With `recording: false`, say recording
   is off and how to turn it on (above), and stop.
2. For each listed decision, show its title, context, constraints and
   options, and ask how it turned out. The listing holds no run, and you must
   not look one up or recall it from this conversation before the outcome is
   recorded. Seeing the pick, the confidence or the recommendation first
   anchors the label on the model's answer.
3. Ask for the class: `dominant` with the option that proved right, or
   `contested` when reasonable people would still disagree. Offer
   `contested` when hindsight does not settle it. Ask why, in the user's
   words. Never suggest or invent a label.
4. Call `go-decide-record-outcome` with `id`, `class`, `correct-option` for a
   `dominant` decision only, and `why`. Record nothing for a decision the user
   skips or cannot judge yet.

## Scoring

```sh
go-decide eval --dry-run --log "$DECIDE_LOG"   # validates, free
go-decide eval --log "$DECIDE_LOG"             # one paid request per decision
```

`eval --log` uses only labelled decisions, the latest outcome for each. It is
not served over MCP, because it spends.
