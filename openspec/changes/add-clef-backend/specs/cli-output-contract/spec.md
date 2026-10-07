# Spec Delta

## MODIFIED Requirements

### Requirement: Output is a versioned JSON envelope

The system SHALL print a command's result as one JSON document on standard
output using the shared success envelope, and SHALL print nothing else there.
The data SHALL carry a schema version for its shape and the name of the
backend that was selected, including on a dry run. Errors SHALL be printed as
the shared error envelope on standard error and SHALL leave standard output
empty of results. An uncertain or escalate outcome is not an error and SHALL
NOT produce an error envelope. Diagnostics and logs SHALL go to standard error.
The JSON envelope SHALL be printed in both the machine and human output modes.

#### Scenario: Result on standard output only

- **WHEN** `ask` produces an outcome
- **THEN** standard output holds exactly one JSON envelope with the schema
  version and the backend, and standard error holds no result

#### Scenario: Failure

- **WHEN** a command fails
- **THEN** the error envelope is printed on standard error, nothing is printed
  on standard output, and the exit code reflects the failure kind

#### Scenario: Uncertain outcome is not an error

- **WHEN** `ask` produces an uncertain outcome
- **THEN** the result is on standard output, standard error has no error
  envelope, and the exit code is 10

#### Scenario: Backend added at version 4

- **WHEN** the schema version moves from 3 to 4 to add the backend
- **THEN** golden files for version 4 exist for every output, and the
  version 3 files still pass unchanged
