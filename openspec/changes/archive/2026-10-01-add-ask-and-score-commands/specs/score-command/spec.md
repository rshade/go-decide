# Spec Delta

## Purpose

Defines the `score` command, which asks Jev to rate content against an ordered
rubric and prints the typed outcome.

## ADDED Requirements

### Requirement: score rates content against a rubric and prints the outcome

The system SHALL provide a `score` command that reads a score spec, validates
it, asks Jev and prints the outcome as JSON on standard output. The outcome
SHALL be exactly one of decided, uncertain or escalate, carrying the fractional
score, the nearest level, the confidence and the probability of every level.
Only a decided outcome SHALL present its level as a rating; the other two SHALL
present it as the nearest level only.

#### Scenario: Decided rating

- **WHEN** Jev answers with confidence at or above the confident level
- **THEN** the output states the outcome is decided and gives the level and the
  fractional score

#### Scenario: Uncertain rating

- **WHEN** Jev answers between the floor and the confident level
- **THEN** the output states the outcome is uncertain and does not present the
  level as a rating

#### Scenario: Escalate rating

- **WHEN** Jev answers below the floor
- **THEN** the output states the outcome is escalate

#### Scenario: Wrong kind of spec

- **WHEN** the spec carries options instead of levels
- **THEN** the command fails with a validation error naming the levels field
  and sends no request

### Requirement: score accepts the same thresholds as ask

The `score` command SHALL accept the same confident level and floor flags as
`ask`, with the same validation and defaults.

#### Scenario: Invalid thresholds

- **WHEN** the floor is above the confident level
- **THEN** the command fails with a validation error and sends no request
