# ask-command Specification

## Purpose

Defines the `ask` command, which puts a choice question to Jev and prints the
typed outcome, so scripts and agents can tell whether the answer can be acted
on.

## Requirements

### Requirement: ask puts a choice question and prints the outcome

The system SHALL provide an `ask` command that reads a choice spec, validates
it, asks Jev and prints the outcome as JSON on standard output. The outcome
SHALL be exactly one of decided, uncertain or escalate, carrying the leading
option, the confidence and the probability of every option. Only a decided
outcome SHALL present its option as a choice; the other two SHALL present it as
the leading option only.

#### Scenario: Decided outcome

- **WHEN** Jev answers with confidence at or above the confident level
- **THEN** the output states the outcome is decided and names the chosen option

#### Scenario: Uncertain outcome

- **WHEN** Jev answers between the floor and the confident level
- **THEN** the output states the outcome is uncertain and names the leading
  option without calling it a choice

#### Scenario: Escalate outcome

- **WHEN** Jev answers below the floor
- **THEN** the output states the outcome is escalate

#### Scenario: Wrong kind of spec

- **WHEN** the spec carries levels instead of options
- **THEN** the command fails with a validation error naming the options field
  and sends no request

### Requirement: Thresholds can be set for one run

The `ask` command SHALL accept a confident level and a floor for the run and
SHALL reject a pair that is not valid, before any request. Without them it
SHALL use the default thresholds, and the output SHALL state the thresholds used.

#### Scenario: Custom thresholds

- **WHEN** the floor is 0.4 and the confident level is 0.8
- **THEN** an answer at 0.85 is decided and the output states 0.4 and 0.8

#### Scenario: Invalid thresholds

- **WHEN** the floor is above the confident level
- **THEN** the command fails with a validation error and sends no request
