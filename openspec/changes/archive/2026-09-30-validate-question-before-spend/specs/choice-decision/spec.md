# Spec Delta

## MODIFIED Requirements

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

## ADDED Requirements

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
