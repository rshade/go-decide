# Spec Delta

## ADDED Requirements

### Requirement: eval reads a decision log

`eval --log <file>` SHALL read a decision log written by `record` and
evaluate its labelled decisions: for each id with an `outcome` line, the
first `decision` line's decision and the latest `outcome` line's truth. It
SHALL apply the same validation as `--decisions` and `--truth`, SHALL refuse
`--log` together with either of them, and SHALL fail with exit 2 when no
decision is labelled.

#### Scenario: Labelled decisions only

- **WHEN** `eval --dry-run --log decisions.jsonl` runs on a log with one
  labelled and one unlabelled decision
- **THEN** it exits 0 and counts one decision

#### Scenario: Latest outcome wins

- **WHEN** a decision has two `outcome` lines
- **THEN** eval uses the later line's class and correct option

#### Scenario: Nothing labelled

- **WHEN** the log has no `outcome` line
- **THEN** eval exits 2 naming `--log`, and sends nothing

#### Scenario: Conflicting inputs

- **WHEN** `--log` is given with `--decisions` or `--truth`
- **THEN** eval exits 2 naming the flags, and sends nothing
