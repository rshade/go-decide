# Spec Delta

## Purpose

Defines the `eval` command, which runs a labelled decision set through Jev and
reports how well its confidence separates clear decisions from contested ones,
so that the thresholds can be checked and tuned on evidence.

## ADDED Requirements

### Requirement: eval runs a labelled decision set and prints a report

The system SHALL provide an `eval` command that reads a decision set
(`--decisions`) and a truth file (`--truth`), both JSON files in the format of
`testdata/decisions.json` and `testdata/decisions_truth.json`. For each
decision it SHALL ask Jev one choice question. The question's state is the
decision's title, context, constraints and options. Its options are the
decision's option names, with descriptions. Its instructions default to
"Given the stated constraints, which option should the team choose?" and can
be replaced with `--instructions`. The command SHALL accept `--floor` and
`--confident` as `ask` does. It SHALL print one JSON envelope with the
metrics, the per-threshold rows, the outcome counts, the thresholds used and
one row per decision. Each row holds the decision's id, class, pick, pick
confidence, correct option, whether it was safe to fast-path, and its outcome.

#### Scenario: Report for the bundled set

- **WHEN** `eval --decisions testdata/decisions.json --truth
  testdata/decisions_truth.json` runs against a server that answers every
  question
- **THEN** standard output holds one envelope with 40 decision rows, the
  accuracy, contested AUC and Brier score, and the per-threshold rows, and the
  exit code is 0

#### Scenario: Custom instructions

- **WHEN** `eval` is run with `--instructions "Which option fits best?"`
- **THEN** every request carries those instructions

### Requirement: Both files are validated before any request

The system SHALL validate both files completely before sending any request.
Validation SHALL fail when:

- a file is missing, empty or malformed
- a decision or truth id is empty or repeated
- the two files do not hold the same set of ids
- a class is neither `dominant` nor `contested`
- a dominant entry's correct option is missing or is not one of its options
- a contested entry has a correct option
- a decision does not form a valid choice question, by the rules `ask`
  applies

- a decision or truth entry has a field the format does not define. A
  decision defines `id`, `title`, `context`, `constraints` and `options`. A
  truth entry defines `id`, `class`, `correct_option` and `why`.
- an option has no `name`

The failure SHALL name the id and field at fault and exit 2. An option object
is content: fields beyond `name` and `description` SHALL be kept in the
question's state, not rejected.

#### Scenario: Truth names an option that does not exist

- **WHEN** the truth file gives `dec-001` the correct option `Mainframe`,
  which is not one of its options
- **THEN** the command fails naming `dec-001` and the correct option, the
  server has received zero requests, and the exit code is 2

#### Scenario: Ids do not match

- **WHEN** the decision set has `dec-041` and the truth file does not
- **THEN** the command fails naming `dec-041` and sends nothing

#### Scenario: Unknown top-level field

- **WHEN** a decision carries a field `priority`
- **THEN** the command fails naming that decision and `priority`, and sends
  nothing

#### Scenario: Extra option fields are content

- **WHEN** an option carries `pros`, `cons` and `cost_usd_per_month`
- **THEN** validation passes and those fields are sent in the state

### Requirement: A failed call fails the run

The system SHALL treat a failed call or an unusable answer for any decision as
a failure of the whole run. It SHALL stop sending further requests, print the
error envelope with the shared failure code, and print no partial report.
Responses received before the failure SHALL stay in the cache when caching is
on, so that a rerun does not pay for them again.

#### Scenario: One answer is missing

- **WHEN** the server answers the first decision and returns no choice answer
  for the second
- **THEN** no report is printed, the error names the second decision, and the
  exit code is a failure code

#### Scenario: Rerun after a failure

- **WHEN** a run with `--cache-dir` fails on the third decision and is run
  again against a server that now answers everything
- **THEN** the second run sends no request for the first two decisions

### Requirement: Responses can be cached for free reruns

The system SHALL accept `--cache-dir`. When it is given, the command SHALL use
the client's response cache in that directory, so that rerunning the same
decisions with the same instructions sends no request. Without the flag,
nothing SHALL be written to disk.

#### Scenario: Second run is free

- **WHEN** `eval` runs twice with the same files and `--cache-dir`
- **THEN** the server receives one request per decision in total, and both
  runs print the same metrics

#### Scenario: No flag, no files

- **WHEN** `eval` runs without `--cache-dir`
- **THEN** no file is created

### Requirement: A dry run checks the set without asking

The system SHALL honor `--dry-run` on `eval` by validating both files and the
thresholds and then stopping, with no request sent and no API key needed. It
SHALL print a dry-run result that carries the schema version, the number of
decisions, the number of dominant and contested decisions, and the thresholds.

#### Scenario: Dry run of the bundled set

- **WHEN** `eval --dry-run` is run on the bundled files with no API key set
- **THEN** it reports 40 decisions, 20 dominant and 20 contested, sends
  nothing, and exits 0

### Requirement: eval measures and never approves

The system SHALL exit 0 whenever it prints a report, whatever the metric
values. The report SHALL NOT be presented as an approval of the thresholds or
of any decision. A per-decision row SHALL expose the pick of a decision only
as its pick, together with its outcome, so that a pick below the confident
level does not read as decided.

#### Scenario: Poor metrics still exit zero

- **WHEN** the contested AUC is 0.5
- **THEN** the report is printed and the exit code is 0
