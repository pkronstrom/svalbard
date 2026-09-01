# repository-verification Specification

## Purpose
TBD - created by archiving change unify-repo-verification. Update Purpose after archive.
## Requirements
### Requirement: Canonical repository verification
The repository SHALL provide one root command that verifies generated catalog parity and every maintained Go module.

#### Scenario: Verify repository
- **WHEN** `make verify` runs
- **THEN** catalog parity is checked and tests run for `tui`, `host-tui`, `host-cli`, `drive-runtime`, and `build-tools`

#### Scenario: Verification failure
- **WHEN** catalog data is stale or any module test fails
- **THEN** the canonical command exits non-zero

#### Scenario: Spec-flow verification
- **WHEN** spec-flow verifies a change
- **THEN** it invokes `make verify` rather than duplicating module commands

