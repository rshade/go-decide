# eval-metrics Specification

## Purpose

Defines how labelled Jev choice results are scored, so that the confidence
thresholds can be judged and tuned on evidence: pick accuracy, how well
confidence separates contested from clear decisions, calibration, and the
precision and recall of a fast path at each threshold.

## Requirements

### Requirement: A labelled result says whether the fast path was safe

The system SHALL score labelled results. Each result holds the decision's class
(dominant or contested), the option Jev picked, the pick confidence as a
validated probability, and for a dominant decision the correct option. A result
SHALL count as safe to fast-path only when the decision is dominant and the
pick equals the correct option. A contested decision SHALL never count as safe,
whatever Jev picked.

#### Scenario: Correct pick on a dominant decision

- **WHEN** a dominant decision whose correct option is `a` is picked as `a`
- **THEN** the result counts as safe to fast-path

#### Scenario: Contested decision

- **WHEN** a contested decision is picked with confidence 0.99
- **THEN** the result does not count as safe to fast-path

### Requirement: A malformed labelled result is rejected

Creating a labelled result SHALL fail with a typed error when the confidence is
not a valid probability, when the pick is empty, when the class is neither
dominant nor contested, when a dominant result has no correct option, or when
a contested result has one.

#### Scenario: Dominant result without a correct option

- **WHEN** a dominant result is created with no correct option
- **THEN** creation fails with a typed error and no result is returned

### Requirement: Metrics are computed from labelled results

The system SHALL compute these metrics from a non-empty set of labelled
results:

- **Accuracy**: correct picks divided by dominant decisions.
- **Contested AUC**: the area under the ROC curve for detecting contested
  decisions, scoring each result by 1 minus its pick confidence. A tied pair
  counts as one half.

#### Scenario: Perfect separation

- **WHEN** every contested result has a lower confidence than every dominant
  result
- **THEN** the contested AUC is 1

#### Scenario: Accuracy

- **WHEN** two dominant decisions are picked, one correctly and one wrongly
- **THEN** accuracy is 0.5

### Requirement: The Brier score measures calibration

The system SHALL also compute, from the same non-empty set, the **Brier
score**: the mean squared difference between the pick confidence and 1 for a
result that is safe to fast-path, or 0 for one that is not.

#### Scenario: Brier score of a confident wrong pick

- **WHEN** the only result is a dominant decision picked wrongly with
  confidence 0.9
- **THEN** the Brier score is 0.81 and accuracy is 0

### Requirement: An undefined metric is absent, never zero

The system SHALL report a metric that is undefined for the set as absent, and
SHALL NOT report it as zero. Accuracy is undefined with no dominant results,
and the AUC is undefined unless both classes are present. Computing metrics
over an empty set SHALL fail with a typed error.

#### Scenario: Only contested decisions

- **WHEN** every result is contested
- **THEN** accuracy and the contested AUC are absent, and the Brier score is
  present

#### Scenario: Empty set

- **WHEN** metrics are computed over an empty set
- **THEN** the computation fails with a typed error

### Requirement: Precision and recall are reported per threshold

The system SHALL report, for each threshold from 0.05 to 0.95 in steps of 0.05
and for each threshold the run was given, how many results have a confidence
at or above it. For each threshold it SHALL also report how many of those are
safe to fast-path and how many are not. Thresholds SHALL be listed in
ascending order without duplicates.

#### Scenario: Unsafe results above the confident level are counted

- **WHEN** two results reach 0.95 confidence and one of them is contested
- **THEN** the 0.95 row selects 2, counts 1 as not safe, and has precision 0.5

#### Scenario: A run threshold off the grid

- **WHEN** the run's confident threshold is 0.92
- **THEN** a 0.92 row appears between the 0.9 and 0.95 rows

### Requirement: Precision and recall can be absent

For each threshold the system SHALL also report the precision (safe among those
selected) and the recall (selected among all safe results). Precision SHALL be
absent when nothing is selected, and recall SHALL be absent when no result is
safe.

#### Scenario: Nothing selected

- **WHEN** no result reaches 0.95 confidence
- **THEN** the 0.95 row selects 0 and its precision is absent

#### Scenario: No safe result

- **WHEN** no result is safe to fast-path
- **THEN** recall is absent at every threshold

### Requirement: Outcome counts use the run's thresholds

The system SHALL count how many results would be decided, uncertain and
escalate under the thresholds of the run, using the same rule as a choice
question. It SHALL also count the decided results that are not safe to
fast-path.

#### Scenario: Decided but unsafe

- **WHEN** a contested decision is picked with confidence 0.95 under a
  confident threshold of 0.9
- **THEN** it counts as decided, and as decided but not safe
