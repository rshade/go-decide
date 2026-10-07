# Spec Delta

## Purpose

Defines how `ask`, `score` and `eval` choose between the Jev and clef
backends, so a run is always explicit about which model answered and never
falls back to the other one silently.

## ADDED Requirements

### Requirement: The backend is chosen with a flag and defaults to Jev

`ask`, `score` and `eval` SHALL accept `--backend` with the value `jev` or
`clef`. When the flag is absent the backend SHALL be `jev`. Any other value
SHALL be a validation failure before any request is made.

#### Scenario: Default

- **WHEN** `ask` runs without `--backend`
- **THEN** the request goes to the Jev endpoint

#### Scenario: Clef selected

- **WHEN** `ask --backend clef` runs
- **THEN** the request goes to the clef endpoint and the output names `clef`

#### Scenario: Unknown backend

- **WHEN** `--backend gpt` is given
- **THEN** the command fails with a validation error that lists the allowed
  values, and no request is sent

### Requirement: A backend never falls back to another

The system SHALL use only the selected backend's credentials. When they are
missing it SHALL fail with that backend's configuration error and SHALL NOT
try the other backend. A dry run SHALL validate the question and the backend
name without credentials or a request.

#### Scenario: Clef credentials missing

- **WHEN** `ask --backend clef` runs with `TYPESAFE_API_KEY` set and the
  Cloudflare variables unset
- **THEN** the command fails with the clef configuration error, and no request
  is sent to either backend

#### Scenario: Dry run needs no credentials

- **WHEN** `ask --backend clef --dry-run` runs with no credentials
- **THEN** the question is validated, the output names `clef`, and no request
  is sent

### Requirement: Thresholds default per backend

Each backend SHALL have its own default confident and floor thresholds, and the
output SHALL report the thresholds that were applied. Explicit threshold flags
SHALL override the backend default. Until the thresholds are tuned (issue #6),
the clef defaults SHALL equal Jev's and SHALL be documented as placeholders.

#### Scenario: Override

- **WHEN** `--confident 0.95` is given with `--backend clef`
- **THEN** the output reports 0.95 as the confident threshold

#### Scenario: Backend default reported

- **WHEN** no threshold flag is given
- **THEN** the output reports the selected backend's default thresholds
