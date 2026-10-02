# Spec Delta

## MODIFIED Requirements

### Requirement: Output shapes are discoverable through __schema

The system SHALL expose the command tree, including flags and the output schema
version of each command, through the `__schema` command, and SHALL mark the
output fields that vary between identical calls as non-deterministic.

#### Scenario: Schema lists both commands

- **WHEN** `__schema` is run
- **THEN** it lists `ask` and `score` with their flags and output version

#### Scenario: Schema lists eval

- **WHEN** `__schema` is run
- **THEN** it also lists `eval` with its flags and output version

### Requirement: A dry run validates without asking

The system SHALL honor the shared `--dry-run` flag on `ask`, `score` and `eval`
by validating the input and the thresholds and then stopping, so that no
request is sent, no API key is needed and nothing is billed. For `ask` and
`score` it SHALL print the shared envelope with a dry-run result that carries
the schema version, the kind of question, the option or level names in order
and the thresholds. For `eval` the dry-run result SHALL describe the decision
set as the `eval` command specifies. A dry run SHALL exit 0. Invalid input in
a dry run SHALL fail as it does without the flag.

#### Scenario: Dry run sends nothing

- **WHEN** `ask --dry-run` is given a valid spec
- **THEN** the server has received zero requests, the output states it is a dry
  run and lists the option names in order, and the exit code is 0

#### Scenario: Dry run needs no key

- **WHEN** `score --dry-run` is run with no API key set
- **THEN** it succeeds and lists the level names in order

#### Scenario: Dry run still validates

- **WHEN** `ask --dry-run` is given a spec with a repeated option name
- **THEN** it fails with a validation error naming the options field and exits 2

#### Scenario: eval dry run sends nothing

- **WHEN** `eval --dry-run` is given a valid decision set and truth file with
  no API key set
- **THEN** the server has received zero requests and the exit code is 0

### Requirement: A schema change requires a version bump

The system SHALL pin each output shape, and the `__schema` output, with golden
files. A change to a shape, including a new command in `__schema`, SHALL fail
the tests until the schema version is increased and a golden file for the new
version is added. The golden file of an earlier version SHALL keep passing
unchanged. An output that first appears in a later version SHALL have golden
files from that version on, and SHALL NOT need files for earlier versions.

#### Scenario: Shape unchanged

- **WHEN** the tests run against the current output
- **THEN** each output matches its golden file for the current version

#### Scenario: Shape changed without a bump

- **WHEN** a field is added to an output without a version bump
- **THEN** the golden test fails and names the version

#### Scenario: New command starts at the current version

- **WHEN** `eval` first ships at schema version 2
- **THEN** its outputs have golden files for version 2 and none for version 1,
  and the version 1 files of `ask`, `score` and `__schema` are unchanged

### Requirement: Exit codes tell an outcome from a failure

The system SHALL exit 0 for a decided outcome, 10 for an uncertain outcome and
11 for an escalate outcome, and SHALL print the full result in all three cases.
`eval` SHALL exit 0 when it prints a report, because its output is a
measurement, not an outcome. For failures it SHALL exit with the shared code
for the kind: 2 for invalid input, 3 for network failures, 4 for
authentication failures, and 1 for anything else. It SHALL document every
code.

#### Scenario: Decided exits zero

- **WHEN** the outcome is decided
- **THEN** the exit code is 0

#### Scenario: Uncertain and escalate are distinct

- **WHEN** the outcome is uncertain, then escalate
- **THEN** the exit codes are 10 and 11 and neither is 0

#### Scenario: eval report exits zero

- **WHEN** `eval` prints a report in which most decisions are uncertain
- **THEN** the exit code is 0

#### Scenario: Invalid input

- **WHEN** the spec fails validation
- **THEN** the exit code is 2

#### Scenario: Codes are documented

- **WHEN** the tests run
- **THEN** a test fails if a code the commands can return is missing from the
  documentation
