# choice-decision Specification

## Purpose

Defines how a caller asks Jev a choice question over a typed set of options and
receives a result that cannot be acted on without handling low confidence, so
that a shaky answer can never authorize an action.

## Requirements

### Requirement: The option set is validated before any API call

The system SHALL accept the options of a choice question as a typed set of
names with descriptions. It SHALL reject a set with fewer than two options, a set
with more than 255 options, or a set with an empty option name. It SHALL report
the rejection as an error before any request is sent, so an invalid set costs no
API spend.

#### Scenario: Valid option set

- **WHEN** an option set has two named options
- **THEN** it is accepted and keeps each option's description

#### Scenario: Too few options

- **WHEN** an option set has zero or one option
- **THEN** it is rejected with an error and no request is sent

#### Scenario: Too many options

- **WHEN** an option set has 256 options
- **THEN** it is rejected with an error that names the options and no request is
  sent

#### Scenario: Largest allowed set

- **WHEN** an option set has exactly 255 options
- **THEN** it is accepted

#### Scenario: Empty option name

- **WHEN** an option set contains an option whose name is empty
- **THEN** it is rejected with an error and no request is sent

#### Scenario: Duplicate names cannot be expressed

- **WHEN** a caller builds an option set from a set of names
- **THEN** each name appears once, so a duplicate name never reaches a request

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

### Requirement: The question is validated before any API call

The system SHALL validate a choice question before sending any request, so an
invalid question costs no API spend. It SHALL reject a question whose state is
absent or empty, and a question whose estimated size clearly exceeds the 32k
limit for state plus the longest question. The size estimate SHALL err toward
accepting: a question near the limit is sent and left for the API to judge.
Every rejection SHALL be an error that names the offending field and gives the
reason, and callers SHALL be able to recognise it as a validation failure and
read the field name programmatically.

#### Scenario: Absent state

- **WHEN** a question has no state
- **THEN** it is rejected with an error naming the state field and no request is
  sent

#### Scenario: Empty state

- **WHEN** a question's state is an empty string, an empty object or an empty
  list
- **THEN** it is rejected with an error naming the state field and no request is
  sent

#### Scenario: State that is a typed nil

- **WHEN** a question's state holds a nil pointer, map or slice
- **THEN** it is rejected with an error naming the state field and no request is
  sent

#### Scenario: State over the size limit

- **WHEN** the state, the instructions and the option descriptions together are
  clearly larger than the 32k limit
- **THEN** the question is rejected with an error naming the state field and
  giving the estimated size and the limit, and no request is sent

#### Scenario: State that cannot be encoded

- **WHEN** a question's state cannot be encoded as JSON
- **THEN** it is rejected with an error naming the state field and no request is
  sent

#### Scenario: State near the limit

- **WHEN** the estimated size is under the limit
- **THEN** the question is sent

#### Scenario: Invalid question makes no HTTP call

- **WHEN** any invalid question is given to the choice operation
- **THEN** it returns the validation error and the server has received zero
  requests

#### Scenario: Valid question is unchanged

- **WHEN** a question with non-empty state, valid options and any instructions,
  including none, is given to the choice operation
- **THEN** it is sent and answered as before
