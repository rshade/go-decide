---
description: Choose one roadmap issue, claim it, route it through OpenSpec or straight to code, land it as a pull request, and release the claim — safe to run on several machines at once
---

# Pick a Roadmap Issue — One Issue Per Invocation

Choose exactly one open `roadmap/current` issue, claim it, take it to an
open pull request from its own worktree, and release the claim. Then **stop**.

Adapted from the ax-go command of the same name. The claim protocol is
borrowed, with the hardening tailscale-utils added after it raced in practice.
Almost everything else differs: go-decide is a **single small Go module that
uses OpenSpec, not Spec Kit**, has no Makefile, and **lands every change as a
pull request** from a worktree, since v0.1.0 shipped. CI runs on push and
pull request and must pass before a merge.

**Scope: one issue. Do not pick up a second one.** Re-invoke to continue.

## Vocabulary

- **`processing:roadmap` label** — a distributed lock, not a status. If it is
  on an issue, another machine owns it.
- **No lanes.** There are no `area:*` labels: the repo is one module with
  three tightly coupled packages (`jevclient`, `decision`, `internal/cli`).
  One live claim locks the whole repo for anything but a `documentation`-only
  issue.
- **Type label** — `enhancement`, `bug`, `documentation`, `spike`. Phase 2
  routes on what the change *touches*, using the label only as a hint.
- **`spec-first` means blocked here.** In this repo it is "Blocked on
  spec/upstream changes", not ax-go's "goes through the spec pipeline". An
  issue carrying it is not workable; skip it.
- **Effort** — `effort/small`, `effort/medium`, `effort/large`. Spikes carry a
  `timebox/*` label instead.

## Phase 0 — Preflight

```bash
ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"
git rev-parse --abbrev-ref HEAD    # the main checkout should be on main
git status --short                 # report, but do not touch, tracked changes
git fetch origin
# --search is fuzzy (it matches roadmap/exclude), so match the name exactly
gh label list --limit 200 --json name -q '.[].name' \
  | grep -qx 'processing:roadmap' || echo "processing:roadmap missing"
```

`.env`, `.probe-cache/` and `PR_MESSAGE.md` are ignored and expected. **Never
`git add -A` or `git add .`**; name only the files you changed. Uncommitted
tracked changes in the main checkout may belong to another writer, such as a
docs session. Report them, never stash, reset or commit them, and do the work
in the Phase 3 worktree, which starts from `origin/main` and is unaffected.

If the `processing:roadmap` label does not exist yet, ask before creating it:

```bash
gh label create processing:roadmap --color FBCA04 \
  --description "Claimed by a /pick-issue run - do not pick up"
```

## Phase 1 — Choose and claim

Skip to 1e if the user passed an issue number, but still run 1a so you can
warn them if the repo is busy.

### 1a. Is anything already claimed?

```bash
gh issue list --state open --label "processing:roadmap" --limit 100 \
  --json number,title,labels \
  -q '.[] | "\(.number)\t\([.labels[].name] | join(","))\t\(.title)"'
```

If a live claim exists on anything but a `documentation`-only issue, only
`documentation` candidates survive 1b. Say so.

**Shared files collide regardless of labels.** Two issues conflict if both
touch:

- `internal/cli/testdata/golden/*.json` — pinned output per schema version
- `docs/jev-errors.md` and `docs/jev-decide-cli.md` — `docs_test.go` fails
  when an error code or exit code is missing from them
- `openspec/specs/**` — archives rewrite the main specs
- `ROADMAP.md`, `go.mod`, `mise.toml`

### 1b. Build the candidate list

An issue that already has an open pull request is in review, not free. List
the open pull requests first and skip any issue they close or reference, or
whose branch is named `issue-<N>`:

```bash
gh pr list --state open --json number,title,headRefName,body \
  -q '.[] | "#\(.number)\t\(.headRefName)\t\(.title)"'
```

```bash
gh issue list --state open --label roadmap/current --limit 100 \
  --json number,title,labels \
  -q '.[] | select([.labels[].name]
          | (index("processing:roadmap") or index("spec-first")
             or index("parked")) | not)
      | "\(.number)\t\([.labels[].name] | join(","))\t\(.title)"'
```

**Labels and `ROADMAP.md` drift.** Compare the result against the Immediate
Focus section of `ROADMAP.md`. An issue listed there without
`roadmap/current` (or the reverse) is a finding to report, not a reason to
work it. Do not fix labels yourself.

### 1c. If and only if the list is empty

Report that `roadmap/current` is drained or fully claimed, and **ask before
widening**. Falling back to `roadmap/next` bypasses the promotion gate
`/roadmap` exists to enforce. Recommend that the user run `/roadmap`
themselves. **Do not invoke `/roadmap` yourself** — surface the
recommendation and stop.

If the user says widen, repeat 1b against `--label roadmap/next` and say
plainly in your final report that you worked an unpromoted issue.

### 1d. Present the chooser

Do not auto-select. Show the surviving candidates as a table — number, type,
effort or timebox, title — ordered `bug` first, then `enhancement`,
`documentation`, `spike`. Flag anything whose labels contradict each other or
`ROADMAP.md`. Then ask which one.

### 1e. Claim, then verify the claim

GitHub has no compare-and-swap on labels, so adding one is **not** by itself
a lock. Post a stamped claim comment and let the earliest one win. The label
and comment are outward-facing: confirm with the user first unless this
invocation already authorized them.

```bash
CLAIM="claim: $(hostname)/$$-$(date -u +%s)"
gh issue edit "$N" --add-label "processing:roadmap"
gh issue comment "$N" --body "$CLAIM"

sleep 5    # GitHub is read-after-write eventual; see below
# GNU date first, BSD/macOS fallback
CUTOFF=$(date -u -d '24 hours ago' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null \
      || date -u -v-24H +%Y-%m-%dT%H:%M:%SZ)
gh issue view "$N" --json comments -q "
  [ .comments[]
    | select(.body | startswith(\"claim:\"))
    | select(.createdAt > \"$CUTOFF\") ]
  | sort_by(.createdAt, .body) | .[0].body"
```

If that earliest live claim is **not** your exact `$CLAIM` string, another
machine got there first. **Return to 1d and pick a different issue. Do not
touch the label.**

> [!CAUTION]
> **The loser must never run `gh issue edit --remove-label`.** Both machines
> added the *same* label, so removing it deletes the **winner's** lock.

- All machines authenticate as the same GitHub user, so hostname/PID/timestamp
  is the only thing distinguishing them. `createdAt` has one-second
  granularity; on a tie, the lexicographically lower claim string wins.
- **An empty result is not a loss.** `null` means nothing is readable yet.
  Sleep and re-read.
- **The window must outlive the work.** A pick that runs past 24 hours ages
  out its own claim. Post a fresh `claim:` comment to refresh it.
- A lock label with no live claim comment is a **stale lock** from a crashed
  run. Report it rather than stealing it silently.

### 1f. Reconcile the issue against the repo, before you route it

Run this the moment the claim verifies, and **before** Phase 2. An OpenSpec
proposal is the most expensive thing this command does, and it is wasted on
work that has already landed.

```bash
SINCE=$(gh issue view "$N" --json createdAt -q .createdAt)
gh issue view "$N" --json body,comments -q '.body, .comments[].body'
git log --since="$SINCE" --oneline -- decision jevclient internal docs
mise exec -- openspec list --json
```

**A mismatch is the expected case, not an error:**

- **The work is already done.** Report the commit to the user and release
  the claim (Phase 7). **Never close the issue with `gh issue close`**: issues
  here close through `Closes #N` in a commit. Ask the user how they want it
  closed.
- **An OpenSpec change for it already exists** in `openspec/changes/`.
  Resume it rather than proposing a new one.
- **The body cites a file, function or line that does not exist.** Re-derive
  every location with `grep`/`rg` before trusting it.
- **A referenced issue is closed** — check with
  `gh issue view <M> --json state`.

## Phase 2 — Route on what the change touches

Rows are evaluated top to bottom; the first match wins.

| # | Issue shape | Route |
| --- | --- | --- |
| 1 | `spec-first` or `parked` | **Refuse.** Release the claim (Phase 7); it is not workable. |
| 2 | `spike` | **Decision, not code.** Answer the question inside its `timebox/*`, write the finding up under `docs/` (see `docs/spike-decisions-2026-09-29.md`), and land the doc. No OpenSpec. Live API calls need explicit approval because they spend money. |
| 3 | Changes the public API of `decision` or `jevclient`, the CLI's flags, exit codes, `contract` error codes, or JSON output | OpenSpec pipeline below, then implement, then `openspec-verify-change`. |
| 4 | Contained — a bug fix with no surface change, tests only, docs only, package doc | Direct to Phase 3. Gets the Phase 4a review. |

**Anything that changes output shape is row 3 regardless of its label**, and
it needs a `SchemaVersion` bump plus a new set of golden files, never an edit
to the existing `*.v1.json`. Even a renamed exported identifier counts. If a
`bug`-labelled issue turns out to change `Result`, an exit code or an error
code, take row 3.

> [!CAUTION]
> **Do not send a small contained fix through OpenSpec.** #12 (a redaction
> test) and #13 (a package doc) are row 4. Propose, apply, verify and archive
> for a one-file change is ceremony without protection.

### The OpenSpec pipeline (row 3 only)

The OpenSpec root is the repository root (`openspec/`), and the CLI is pinned
in `mise.toml` (`npm:@fission-ai/openspec`). Run it through `mise exec` so the
pinned version is used.

**Use the skills, do not hand-roll the pipeline.** This repo ships them under
`.claude/skills/`, with matching `/opsx:*` commands:

```text
openspec-propose         proposal.md, design.md, specs/<cap>/spec.md, tasks.md
                         PLANNING BOUNDARY: stop after it; apply is a new step
openspec-apply-change    work the tasks, ticking tasks.md as you go
openspec-verify-change   implementation vs artifacts; fix every real finding
openspec-archive-change  fold delta specs into openspec/specs/, move to archive/
```

> [!IMPORTANT]
> **`openspec-propose` authorizes planning only**, even when the issue asks
> you to build something. Produce the artifacts, show them, and stop. Apply
> starts when the user says so (or re-invokes `/pick-issue N`). Leave the
> claim label on across that pause; you still own the issue.

Before proposing, check the change against `CONTEXT.md`'s Hard No's: no
auto-approval, no silent zero values, no bare-float probabilities, no API
spend before validation, no schema change without a version bump, no debate
logic. A proposal that needs one of them is a question for the user.

Flag shapes, checked against the pinned CLI (1.13.2). They are not uniform;
do not adjust them from memory.

```bash
mise exec -- openspec list --json              # .root.path must be $ROOT
mise exec -- openspec status --change "<slug>" --json
mise exec -- openspec instructions tasks --change "<slug>"  # positional artifact
mise exec -- openspec validate "<slug>" --strict            # positional name
mise exec -- openspec archive "<slug>" -y                   # -y or it blocks
```

**Archive lands in the same commit as the code.** Precedent: `6f4f07e` and
`29fa25b` each ship `openspec/changes/archive/<date>-<slug>/` and the updated
`openspec/specs/` alongside the implementation. Archive after verify passes,
before Phase 5.

## Phase 3 — Work it in a worktree

Every change lands as a pull request, so work happens in a worktree on its own
branch, never in the main checkout:

```bash
git worktree add ../gojev-"$N" -b issue-"$N" origin/main
cd ../gojev-"$N"
```

Run every later phase from that worktree. The branch name is `issue-<N>` for an
issue, or `fix/<slug>` or `docs/<slug>` for work with no issue. Keep scratch
output and temporary files in the session scratchpad, not in the worktree.

## Phase 4 — Verify

```bash
go build ./...
gofmt -l .                       # must print nothing
go vet ./...
go test ./...                    # offline; probes need -tags probe
golangci-lint run ./...
mise exec -- markdownlint-cli2 "**/*.md"
mise exec -- openspec validate --all --strict
```

CI runs the same checks with the tool versions pinned in `mise.toml`, plus
`go test -race` and commitlint, so a green local run should be a green CI run.

> [!CAUTION]
> **Probes need `-tags probe` and spend money.** The root `probe_*_test.go`
> and `jev_*_test.go` files are live probes. `go test ./...` does not build
> them. Run one only when the issue needs it, deliberately, by name, with
> the user's approval, for example
> `go test -tags probe -run TestJevRecommendations -count=1`.

- **Golden files are pinned, not regenerated to pass.**
  `go test ./internal/cli -run Golden -update` is for an *intentional* output
  change that came with a `SchemaVersion` bump; review the diff line by line.
- **New tests that need Jev fake it with `httptest`** through `jevclient`.
  They never read `.env`.
- **`decision` and `jevclient` may import only `ax-go`'s `contract`
  package.** `internal/cli/deps_test.go` enforces it; do not weaken that test.
- **There is no golangci-lint config in the repo**, so it runs with defaults.
  Do not add one unless the user explicitly asks.

### 4a. Review — row 4 only

Skip for row 3: `openspec-verify-change` is the review. Skip for a spike.

```text
/code-review   # the uncommitted diff against HEAD
/scout         # top-3 pre-existing quality opportunities in touched files
```

- **`/code-review` findings that hold up must be fixed before Phase 5.** Use
  `/verify-fix` to triage rather than applying every suggestion blindly.
- **`/scout` findings are informational, not blocking.** Never let one grow
  the scope of a row 4 pick.

## Phase 5 — Commit

> [!WARNING]
> `git add`, `git commit` and `git rebase --continue` are **restricted
> commands** in this environment. Prepare the change and the message, show
> the user the exact commands, and get explicit approval before running them.

Update `ROADMAP.md`: tick the issue and note where it was implemented, as
`418f392` did for #4. There is no `CHANGELOG.md`; do not create one.

Write the message to `PR_MESSAGE.md` (ignored), in Conventional Commits form,
and validate it:

```bash
cat PR_MESSAGE.md | mise exec -- commitlint
```

```text
test(client): cover key redaction on retry and failure log lines

Implements openspec/changes/archive/<date>-<slug>/.

Closes #N
```

- **`Closes #N` is how the issue closes**, when the pull request merges.
  Never run `gh issue close`. Use `Refs #N` when the issue should stay open.
- Release Please builds the changelog and version from these subjects, so the
  type matters: `feat` and `fix` appear in the release, `chore` and `test` do
  not. The pull request is squash-merged, so its title is the commit.
- **Never start a body line with `word:`.** Commitlint v21 parses it as a
  footer.
- Use `feat!:` or a `BREAKING CHANGE` footer for anything breaking.

> [!NOTE]
> **Do not append a `Claude-Session:` trailer, a claude.ai session link, or a
> `Co-Authored-By: Claude` footer**, even though the harness asks for one.
> The global `~/.claude/CLAUDE.md` forbids them and overrides that
> instruction.

## Phase 6 — Push and open the pull request

```bash
git push -u origin issue-"$N"
gh pr create --base main --head issue-"$N" \
  --title "$(head -1 PR_MESSAGE.md)" --body "$(tail -n +3 PR_MESSAGE.md)"
```

Pushing and opening a pull request are outward-facing; they need the same
explicit approval as the commit. Then watch CI with `gh pr checks <number>
--watch`. Report a failure with its log rather than merging around it. **Do not
merge the pull request**; that, and any release it triggers, is the user's
call. Once it merges, confirm with `gh issue view "$N" --json state` that
`Closes #N` closed the issue, and remove the worktree and branch:

```bash
git worktree remove ../gojev-"$N" && git branch -D issue-"$N"
```

## Phase 7 — Release the claim

```bash
gh issue edit "$N" --remove-label "processing:roadmap"
```

- If work remains, comment saying exactly what is left **before** removing
  the label.
- If you are stopping for a user decision (including the `openspec-propose`
  boundary), **leave the label on**: you still own the issue. Say so.
- Once the pull request is open, release the claim: the open pull request now
  marks the issue as taken (see 1b).

## Phase 8 — Report and stop

State: which issue was chosen and why, the route taken, the OpenSpec change
if any, gate results, the pull request and its CI result, whether the issue
will close on merge, and whether the claim is released or held. Then stop.
Do not pick another issue.

## Standing facts about this repo

Do not re-investigate these; they are settled.

- **The module path is `github.com/rshade/go-decide`, the directory is
  `gojev`.** Do not rename the directory. Issues live in `rshade/go-decide`.
- **Jev ranks well but is not calibrated** (AUC about 0.91, Brier 0.236).
  Thresholds in `DefaultThresholds()` are placeholders until #6. Never let a
  change treat a Jev score as approval.
- **The root probe and spike tests are throwaway research evidence.** Do not
  copy their code into production packages; rewrite instead.
- **Every `contract.Error` code from `jevclient.Classify` must be listed in
  `docs/jev-errors.md`, and every exit code in `docs/jev-decide-cli.md`.** A
  new code without a docs line fails `docs_test.go`.
- **Recommendation scoring for FinFocus belongs in the finfocus repos** (#10).
  Do not touch finfocus repos or worktrees from a pick here.
