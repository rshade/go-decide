# cli-output-contract Specification

## Purpose

Defines the machine contract of the `go-decide` commands: versioned JSON
output, discovery through `__schema`, pinning by golden tests, and exit codes
that tell an outcome apart from a failure.

## Requirements

### Requirement: Output is a versioned JSON envelope

The system SHALL print a command's result as one JSON document on standard
output using the shared success envelope, and SHALL print nothing else there.
The data SHALL carry a schema version for its shape and the name of the
backend that was selected, including on a dry run.

#### Scenario: Result on standard output only

- **WHEN** `ask` produces an outcome
- **THEN** standard output holds exactly one JSON envelope with the schema
  version and the backend, and standard error holds no result

#### Scenario: Backend added at version 4

- **WHEN** the schema version moves from 3 to 4 to add the backend
- **THEN** golden files for version 4 exist for every output, and the
  version 3 files still pass unchanged

### Requirement: Errors and diagnostics stay off the result stream

Errors SHALL be printed as the shared error envelope on standard error and
SHALL leave standard output empty of results. An uncertain or escalate outcome
is not an error and SHALL NOT produce an error envelope. Diagnostics and logs
SHALL go to standard error. The JSON envelope SHALL be printed in both the
machine and human output modes.

#### Scenario: Failure

- **WHEN** a command fails
- **THEN** the error envelope is printed on standard error, nothing is printed
  on standard output, and the exit code reflects the failure kind

#### Scenario: Uncertain outcome is not an error

- **WHEN** `ask` produces an uncertain outcome
- **THEN** the result is on standard output, standard error has no error
  envelope, and the exit code is 10

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
request is sent, no API key is needed and nothing is billed. A dry run SHALL
exit 0. Invalid input in a dry run SHALL fail as it does without the flag.

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

### Requirement: A dry run prints what it validated

For `ask` and `score` a dry run SHALL print the shared envelope with a dry-run
result that carries the schema version, the kind of question, the option or
level names in order and the thresholds. For `eval` the dry-run result SHALL
describe the decision set as the `eval` command specifies.

#### Scenario: Dry run result content

- **WHEN** `ask --dry-run` is given a valid spec
- **THEN** the envelope carries the schema version, the kind of question, the option
  names in order and the thresholds

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
