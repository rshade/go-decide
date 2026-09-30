# probability Specification

## Purpose

Defines a probability value that can only hold a number from 0 to 1, and the
two confidence thresholds built from it, so that no caller handles an
unchecked float as if it were a probability.

## Requirements

### Requirement: A probability holds a number from 0 to 1

The system SHALL provide a probability value that is created only from a number
from 0 to 1 inclusive. Creation SHALL fail with a typed error for NaN, positive
or negative infinity, and any number below 0 or above 1. The zero value of the
type SHALL NOT be a valid probability, so a value that was never set cannot be
mistaken for a probability of 0.

#### Scenario: Boundary values are accepted

- **WHEN** a probability is created from 0 and from 1
- **THEN** both succeed and report exactly those numbers

#### Scenario: Out-of-range numbers are rejected

- **WHEN** a probability is created from -0.01 or from 1.01
- **THEN** creation fails with the invalid-probability error and no value

#### Scenario: Non-finite numbers are rejected

- **WHEN** a probability is created from NaN, +Inf or -Inf
- **THEN** creation fails with the invalid-probability error and no value

#### Scenario: An unset probability is not valid

- **WHEN** a probability value was never created through the constructor
- **THEN** it reports that it is not valid, and no comparison against it is
  ever true

### Requirement: Thresholds are validated and configurable

The system SHALL provide thresholds made of a floor and a confident level, both
valid probabilities, with the floor strictly below the confident level.
Creation SHALL fail with a typed error otherwise. The system SHALL provide
default thresholds of a floor of 0.5 and a confident level of 0.9, documented
as placeholders that issue #6 replaces. Callers SHALL be able to supply their
own valid thresholds.

#### Scenario: Valid thresholds

- **WHEN** thresholds are created with a floor of 0.4 and a confident level of
  0.8
- **THEN** creation succeeds and reports both levels

#### Scenario: Floor not below the confident level

- **WHEN** thresholds are created with equal floor and confident levels, or a
  floor above the confident level
- **THEN** creation fails with the invalid-thresholds error

#### Scenario: Unset level

- **WHEN** thresholds are created with an unset probability for either level
- **THEN** creation fails with the invalid-thresholds error

#### Scenario: Defaults

- **WHEN** no thresholds are supplied
- **THEN** the defaults are a floor of 0.5 and a confident level of 0.9
