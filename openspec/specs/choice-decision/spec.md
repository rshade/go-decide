# choice-decision Specification

## Purpose

Defines how a caller asks Jev a choice question over a typed set of options and
receives a result that cannot be acted on without handling low confidence, so
that a shaky answer can never authorize an action.

## Requirements

### Requirement: The option set is validated before any API call

The system SHALL accept the options of a choice question as a typed set of
names with descriptions. It SHALL reject a set with fewer than two options or
with an empty option name. It SHALL report the rejection as an error before any
request is sent, so an invalid set costs no API spend.

#### Scenario: Valid option set

- **WHEN** an option set has two named options
- **THEN** it is accepted and keeps each option's description

#### Scenario: Too few options

- **WHEN** an option set has zero or one option
- **THEN** it is rejected with an error and no request is sent

#### Scenario: Empty option name

- **WHEN** an option set contains an option whose name is empty
- **THEN** it is rejected with an error and no request is sent

### Requirement: A choice answer becomes exactly one of three results

The system SHALL turn a usable choice answer into exactly one result by
comparing the answer's confidence with the thresholds. Confidence at or above
the confident level SHALL give a decided result. Confidence below the confident
level and at or above the floor SHALL give an uncertain result, meant for human
review. Confidence below the floor SHALL give an escalate result, meant for the
full debate. Every result SHALL carry the leading option, the confidence and the
probability of every option.

#### Scenario: Confident answer

- **WHEN** the answer picks `ship` with confidence 0.95 and the thresholds are
  0.5 and 0.9
- **THEN** the result is decided, its choice is `ship`, and it carries 0.95

#### Scenario: Exactly at the confident level

- **WHEN** the confidence equals the confident level
- **THEN** the result is decided

#### Scenario: Between the levels

- **WHEN** the confidence is 0.7 and the thresholds are 0.5 and 0.9
- **THEN** the result is uncertain and carries the leading option and its
  probabilities

#### Scenario: Exactly at the floor

- **WHEN** the confidence equals the floor
- **THEN** the result is uncertain

#### Scenario: Below the floor

- **WHEN** the confidence is 0.3 and the floor is 0.5
- **THEN** the result is escalate and carries the leading option and its
  probabilities

#### Scenario: Probabilities cover every option

- **WHEN** a result is produced for a set of three options
- **THEN** it carries a probability for each of the three options

### Requirement: Nothing is decided below the confident level

Only a decided result SHALL expose an option that a caller may act on. An
uncertain or escalate result SHALL expose its leading option only under a name
that says it is not a decision. No configuration SHALL make an answer below the
confident level decided. A result that was never produced by a choice, such as
a zero value, SHALL never be decided.

#### Scenario: Low confidence is never decided

- **WHEN** the confidence is below the confident level for any valid thresholds
- **THEN** the result is not decided

#### Scenario: Leading option of a non-decision

- **WHEN** a caller reads the option of an uncertain or escalate result
- **THEN** it is available only as the leading option, not as a choice

#### Scenario: Zero result

- **WHEN** a result value was never produced by a choice
- **THEN** it is treated as escalate and never as decided

### Requirement: Callers must handle all three results

The system SHALL keep the set of results closed: no code outside the package can
create a decided, uncertain or escalate result. It SHALL provide a way to
consume a result that requires one handler for each of the three cases, so
that a missing case is a compile-time error.

#### Scenario: All handlers supplied

- **WHEN** a caller consumes a result with a handler for each case
- **THEN** exactly the handler for that result's case runs

#### Scenario: A case cannot be left out

- **WHEN** a caller consumes a result and omits a handler
- **THEN** the program does not compile

### Requirement: Failures are errors, not results

A failed call SHALL be returned as an error and never as a result. The error
SHALL be the structured error produced for that failure. An answer that is
missing, of the wrong type, or reports a choice outside the option set SHALL
be an error and never a result.

#### Scenario: API failure

- **WHEN** the server answers 401
- **THEN** the call returns the unauthorized error and no result

#### Scenario: Unusable answer

- **WHEN** a 200 response omits the answer to the question
- **THEN** the call returns the invalid-response error and no result

#### Scenario: Missing client

- **WHEN** the call is made without a client
- **THEN** it returns the missing-client error and no result

#### Scenario: Caller cancellation

- **WHEN** the caller's context is canceled during the call
- **THEN** the call returns the cancellation and no result
