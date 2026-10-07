# Spec Delta

## MODIFIED Requirements

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
