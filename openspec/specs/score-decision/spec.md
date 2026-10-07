# score-decision Specification

## Purpose

Defines how a caller asks Jev to rate content against an ordered rubric and
receives a result that cannot be acted on without handling low confidence, so a
shaky rating can never authorize an action.

## Requirements

### Requirement: The rubric is a validated ordered set of levels

The system SHALL accept the levels of a score question as an ordered list of
names with optional descriptions. It SHALL reject a rubric with fewer than two
levels, more than 10 levels, an empty level name, or a duplicate level name. It
SHALL report the rejection as an error that names the levels field before any
request is sent.

#### Scenario: Valid rubric

- **WHEN** a rubric has three named levels
- **THEN** it is accepted and keeps their order and descriptions

#### Scenario: Too few levels

- **WHEN** a rubric has zero or one level
- **THEN** it is rejected and no request is sent

#### Scenario: Too many levels

- **WHEN** a rubric has 11 levels
- **THEN** it is rejected with an error naming the levels field

#### Scenario: Ten levels

- **WHEN** a rubric has exactly 10 levels
- **THEN** it is accepted

#### Scenario: Duplicate level name

- **WHEN** two levels share a name
- **THEN** it is rejected with an error naming the levels field and the name

### Requirement: A score answer becomes exactly one of three results

The system SHALL turn a usable score answer into exactly one result by
comparing the answer's confidence with the same thresholds used for choices.
Confidence at or above the confident level SHALL give a decided result, at or
above the floor and below the confident level an uncertain result, and below
the floor an escalate result.

#### Scenario: Confident rating

- **WHEN** the answer has score 1.9 with confidence 0.95 over levels `low`,
  `medium`, `high` and the thresholds are 0.5 and 0.9
- **THEN** the result is decided, its level is `high`, and it carries 1.9 and
  0.95

#### Scenario: Between the thresholds

- **WHEN** the confidence is 0.7 and the thresholds are 0.5 and 0.9
- **THEN** the result is uncertain and carries the score, the nearest level and
  the probabilities

#### Scenario: Below the floor

- **WHEN** the confidence is 0.3 and the floor is 0.5
- **THEN** the result is escalate

### Requirement: A score result carries the whole answer

Every result SHALL carry the fractional score, the nearest level, the
confidence and the probability of every level. Only a decided result SHALL
expose its level as one a caller may act on.

#### Scenario: Score between levels

- **WHEN** the score is 0.5 over levels indexed from 0
- **THEN** the nearest level is the lower one, and the fractional score is
  still reported unchanged

#### Scenario: Only decided exposes a level

- **WHEN** the result is uncertain or escalate
- **THEN** it carries the score and probabilities but does not expose its
  level as one to act on

### Requirement: An unusable score answer is an error

The system SHALL treat a score answer that is missing, not finite, outside the
rubric, or missing a probability for a level as an invalid-response error and
SHALL NOT produce a result from it.

#### Scenario: Score outside the rubric

- **WHEN** the answer's score is below 0 or above the highest level index
- **THEN** the call returns an invalid-response error and no result

#### Scenario: Missing probability

- **WHEN** the answer has no probability for one level
- **THEN** the call returns an invalid-response error and no result
