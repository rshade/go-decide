# decision-spec Specification

## Purpose

Defines the decision-spec input that the `ask` and `score` commands read, and
its validation before any request, so a bad input costs no API spend.

## Requirements

### Requirement: A decision spec is read from a JSON document

The system SHALL read a decision spec from a JSON file or from standard input.
A spec SHALL carry the state, optional instructions, and either an ordered list
of options (each with a name and optional description) for a choice, or an
ordered list of levels (each with a name and optional description) for a score.
The system SHALL reject a document with an unknown field or trailing content,
and SHALL report a malformed document as a validation error that names the
offending field.

#### Scenario: Valid choice spec

- **WHEN** a JSON document has a state and two named options
- **THEN** it is read as a choice spec, keeping the option order

#### Scenario: Valid score spec

- **WHEN** a JSON document has a state and three named levels
- **THEN** it is read as a score spec, keeping the level order

#### Scenario: Unknown field

- **WHEN** a document contains a field the format does not define
- **THEN** it is rejected with an error naming that field

#### Scenario: Malformed JSON

- **WHEN** the document is not valid JSON
- **THEN** it is rejected with a validation error and no request is sent

### Requirement: Flags fill in the spec without silent overrides

The system SHALL accept the state, instructions, options and levels as flags in
place of, or in addition to, a spec document. A field given both in the
document and by a flag SHALL be rejected as a conflict that names the field,
never resolved silently. Options and levels given by repeated flags SHALL keep
the order in which they were given.

#### Scenario: Flags alone

- **WHEN** a state and two options are given only as flags
- **THEN** they form a valid choice spec

#### Scenario: Flag fills a missing field

- **WHEN** the document has options and the state comes from a flag
- **THEN** the two are combined into one spec

#### Scenario: Conflict

- **WHEN** the state is in the document and also given as a flag
- **THEN** the input is rejected with an error naming the state field

### Requirement: The spec is validated before any request

The system SHALL validate a spec before sending anything. It SHALL reject
duplicate option names, duplicate level names, an empty name, and everything
the choice and score validation already rejects, including a missing or empty
state, more than 255 options, more than 10 levels, and a state clearly over the
size limit. Every rejection SHALL name the offending field, and no request SHALL
be sent.

#### Scenario: Duplicate option names

- **WHEN** two options in the list have the same name
- **THEN** the spec is rejected with an error naming the options field and the
  duplicated name, and no request is sent

#### Scenario: Duplicate level names

- **WHEN** two levels in the list have the same name
- **THEN** the spec is rejected with an error naming the levels field and the
  duplicated name, and no request is sent

#### Scenario: Invalid spec makes no HTTP call

- **WHEN** any invalid spec is given to a command
- **THEN** the command fails with a validation error and the server has
  received zero requests
