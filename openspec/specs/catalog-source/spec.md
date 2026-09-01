# catalog-source Specification

## Purpose
TBD - created by archiving change canonicalize-catalog-source. Update Purpose after archive.
## Requirements
### Requirement: Single editable catalog source
Recipes and presets SHALL be edited in the root canonical catalog and the tracked embedded distribution snapshot SHALL be synchronized from it.

#### Scenario: Synchronize catalog
- **WHEN** the catalog sync command runs
- **THEN** managed embedded files reflect canonical files and ignored local data remains untouched

#### Scenario: Detect drift
- **GIVEN** a managed canonical and embedded catalog file differs
- **WHEN** repository verification runs
- **THEN** verification reports the drift and exits non-zero

#### Scenario: Clean checkout
- **WHEN** a clean checkout builds or tests `host-cli` without running generation first
- **THEN** the tracked embedded snapshot remains available and valid

