# Spec Delta

## Purpose

Defines the opt-in decision log that go-decide writes for the `decide` skill:
each decision recorded before its answer is known, the path its run took, and
an outcome labelled later, so real decisions can be scored with
`go-decide eval` without leaking the answer into the evidence.

## ADDED Requirements

### Requirement: Recording is opt-in

go-decide SHALL write and read the decision log only when the `DECIDE_LOG`
environment variable of the go-decide process names a file. When it is unset
or empty, every `record` command SHALL succeed without touching any file and
SHALL report that recording is off. That is a result, not an error.

#### Scenario: Variable unset

- **WHEN** `go-decide record decision` runs with `DECIDE_LOG` unset
- **THEN** it exits 0, its data says nothing was recorded, and no file is
  created or written

#### Scenario: Variable set

- **WHEN** `go-decide record decision` runs with `DECIDE_LOG` naming a file
  and valid input
- **THEN** one line is appended to that file, and the data carries the new
  record's id and the log path

### Requirement: The log path never comes from a caller

The log path SHALL come only from `DECIDE_LOG`. No `record` flag or MCP tool
input SHALL name, override or redirect the file that is read or written.

#### Scenario: Tool input cannot pick the file

- **WHEN** a client calls a `go-decide-record-*` tool
- **THEN** the tool's inputs have no path field, and the file used is the one
  named by the server process's `DECIDE_LOG`

### Requirement: A decision is recorded before any answer exists

`record decision` SHALL take the decision as the pre-screen would ask it: a
title, context, constraints, the instructions, and at least two options as
`name=description`. It SHALL validate them by the rules `eval` applies to a
decision entry before writing, and SHALL write nothing on a failure. Its input
SHALL have no field for a pick, leading option, confidence, probability or
recommendation.

#### Scenario: Valid decision

- **WHEN** `record decision` gets a title, context, instructions and two
  options
- **THEN** the appended line's `decision` object has exactly `eval`'s decision
  fields, and the line holds no run output

#### Scenario: Invalid decision

- **WHEN** `record decision` gets one option, or two options with the same
  name
- **THEN** it exits 2 naming the field, and the log is unchanged

#### Scenario: Not a decision

- **WHEN** the skill's pre-flight validation turns the input away as not a
  decision
- **THEN** the skill does not call `record decision`

### Requirement: go-decide assigns identity to every line

go-decide SHALL set each line's `v` (format version 1), `kind`, `recorded_at`
(UTC, RFC 3339) and `trace_id` (the trace ID ax-go reports for the call that
wrote it). For a `decision`, it SHALL generate an `id` of the UTC date and
four random hex characters that appears on no earlier `decision` line, and
SHALL use it as the `decision.id`. A missing log file SHALL count as a log
with no ids.

#### Scenario: First write

- **WHEN** `record decision` runs and the log file does not exist yet
- **THEN** the file is created and the decision gets a new id without an error

#### Scenario: Unique id

- **WHEN** the log already holds a `decision` line
- **THEN** a new decision never gets that line's id

### Requirement: The run is recorded when it ends

`record run` SHALL append a `run` line for an id that has a `decision` line,
with the path taken: `fast` (the pre-screen decided and the debate was
skipped), `debate`, or `stopped` (the user stopped after an uncertain result
or a failed call). It SHALL carry the pre-screen's envelope data, the error
code of a failed call, or why the pre-screen did not run, and exactly one of
those. A failure SHALL NOT carry an outcome.

#### Scenario: Fast path

- **WHEN** `record run` gets path `fast` and decided envelope data
- **THEN** the line holds that data, and a `fast` path with a non-decided
  outcome is refused with exit 2

#### Scenario: Pre-screen failed

- **WHEN** `record run` gets path `stopped` and an error code
- **THEN** the line holds the error code and no outcome, choice or leading
  option

#### Scenario: Incomplete envelope

- **WHEN** `record run` gets envelope data that lacks, or holds as null, a
  field `ask` prints (such as `confidence` or a threshold)
- **THEN** it exits 2 naming `pre-screen`, the log is unchanged, and no zero
  is recorded in its place

#### Scenario: Unknown id

- **WHEN** `record run` names an id with no `decision` line
- **THEN** it exits 2 naming the id, and the log is unchanged

#### Scenario: Debate after a decided pre-screen

- **WHEN** a `fast` run is recorded and the user then demands the debate
- **THEN** a second `run` line with path `debate` is appended, and the first
  is left as it was

### Requirement: A debate run records its evidence

A `run` line with path `debate` SHALL carry the moderator's recommendation,
the model evidence mode (`blind` or `informed`) when go-decide was used in the
debate, the number of go-decide calls the debate agents made, and one
citation per go-decide result an agent cited. Each citation SHALL hold the
agent, round, backend, outcome, choice or leading option, confidence, the
instructions sent, and every option or level sent with its probability.
Debate fields on another path SHALL be refused.

#### Scenario: Cited result

- **WHEN** `record run` gets path `debate` and a citation from Advocate B in
  Round 2
- **THEN** the line's citation names `advocate_b`, round 2, and the
  instructions and options it sent

#### Scenario: Malformed citation

- **WHEN** a citation has an unknown field, no `asked` instructions, or no
  `result`, `confidence` or option `probability`
- **THEN** `record run` exits 2 naming the citation, and the log is unchanged

### Requirement: The log is append-only and a write failure is an error

Each `record` call SHALL append exactly one line with a single append-mode
write, and SHALL NOT rewrite, reorder or delete any earlier line. A failure to
open, write or close the file SHALL be returned as an error, and the call
SHALL NOT report the record as written.

#### Scenario: Relabelled outcome

- **WHEN** an id that already has an `outcome` line is labelled again
- **THEN** a new `outcome` line is appended, the earlier one is still in the
  log, and the latest line wins

#### Scenario: Unwritable log

- **WHEN** `DECIDE_LOG` names a file that cannot be opened for writing
- **THEN** `record` fails with an error, and its data never says the record
  was written

### Requirement: A dry run writes nothing

Under `--dry-run`, `record decision`, `record run` and `record outcome` SHALL
validate their input against the log as a write would, and SHALL NOT create
or write any file or directory. The result SHALL say it was a dry run and
SHALL carry no id.

#### Scenario: Dry run of a decision

- **WHEN** `record decision --dry-run` runs with `DECIDE_LOG` naming a file
  whose directory does not exist
- **THEN** it exits 0 with `dry_run` true and no id, and creates neither the
  directory nor the file

#### Scenario: Dry run still validates

- **WHEN** `record run --dry-run` names an id with no `decision` line
- **THEN** it exits 2 naming the id, and the log is unchanged

### Requirement: Pending decisions are listed without their runs

`record pending` SHALL list the `decision` lines older than a threshold (30
days by default) that have no `outcome` line, with their id, time,
instructions and decision. Its output SHALL NOT contain any field of a `run`
line.

#### Scenario: Threshold

- **WHEN** the log holds a 40-day-old and a 10-day-old decision, neither
  labelled
- **THEN** `record pending` lists only the 40-day-old one

#### Scenario: Run hidden

- **WHEN** a pending decision was decided on the fast path
- **THEN** the listing holds no pick, confidence, probability, recommendation
  or path for it

#### Scenario: Recording off

- **WHEN** `record pending` runs with `DECIDE_LOG` unset
- **THEN** it exits 0, says recording is off, and reads no file

### Requirement: An outcome is a valid eval truth entry

`record outcome` SHALL take an id, a class (`dominant` or `contested`), a
correct option and why, and SHALL validate them against that id's
`decision` line: a `dominant` outcome needs one of its option names, and a
`contested` one has none. The appended `truth` object SHALL be an `eval`
truth entry.

#### Scenario: Dominant outcome

- **WHEN** `record outcome` gets class `dominant` and one of the decision's
  option names
- **THEN** the line's `truth` has that name as `correct_option`

#### Scenario: Contested outcome

- **WHEN** `record outcome` gets class `contested` and no correct option
- **THEN** the line's `truth` has `correct_option` null

#### Scenario: Option not in the decision

- **WHEN** a `dominant` outcome names an option the decision does not have
- **THEN** it exits 2 naming `correct-option`, and the log is unchanged

### Requirement: The log stays private

go-decide SHALL create the log file with mode `0600` and any directory it
creates with mode `0700`. Before writing, it SHALL find the nearest existing
directory on the log's path and, when that directory is inside a git working
tree, SHALL write only with `--yes`. A line SHALL hold only the given fields
and the ones go-decide assigns, never a credential or an environment value.

#### Scenario: Owner-only file

- **WHEN** `record` creates the log file and its directory
- **THEN** the file's mode is `0600` and the directory's is `0700`

#### Scenario: Path inside a repository whose directory does not exist yet

- **WHEN** `DECIDE_LOG` names `<repo>/logs/decisions.jsonl` and `logs/` does
  not exist
- **THEN** `record` without `--yes` fails with exit 2 naming `--yes`, and
  creates neither the directory nor the file

#### Scenario: No secrets

- **WHEN** `record` runs with a backend key in the environment
- **THEN** no line of the log contains the key or any environment value

### Requirement: The skill records at fixed points

When the go-decide tools are available, the `decide` skill SHALL call
`record decision` once the brief is settled and before the pre-screen or
Round 1, and `record run` each time a run ends. It SHALL say nothing about
recording when `record` reports recording off, SHALL tell the user the id and
path when a record is written, and SHALL skip recording when the user asks it
to.

#### Scenario: Recorded before the pre-screen

- **WHEN** the skill runs on a decision with recording on
- **THEN** the `record decision` call is made before the first
  `go-decide-ask` call

#### Scenario: Logging declined

- **WHEN** the user asks not to record this decision
- **THEN** the skill makes no `record` call for it and says so

### Requirement: Labelling never shows the model's answer first

When the user asks to review or label the log, the skill SHALL use
`record pending`, show each decision only as listed, and SHALL NOT show any
`run` line for it before `record outcome` is called. The label SHALL be the
user's; the skill SHALL NOT suggest or invent one, and SHALL record nothing
for a decision the user skips.

#### Scenario: No label invented

- **WHEN** the user skips a decision or does not know how it turned out
- **THEN** no `record outcome` call is made for it
