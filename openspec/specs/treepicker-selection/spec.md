# treepicker-selection Specification

## Purpose
TBD - created by archiving change encapsulate-treepicker-selection. Update Purpose after archive.
## Requirements
### Requirement: Protected selection state
TreePicker SHALL prevent external mutation of its effective, explicit, and automatic selection sets.

#### Scenario: Read explicit selection
- **GIVEN** an existing explicit selection
- **WHEN** a caller obtains and mutates the returned user-selection map
- **THEN** the picker state remains unchanged

### Requirement: Replace explicit selection
Replacing user selection SHALL remove prior effective selections and stale automatic dependencies before applying the replacement.

#### Scenario: Cycle Browse preset
- **GIVEN** a picker with selections from one preset
- **WHEN** Browse applies another preset
- **THEN** only the new preset's explicit IDs are selected and totals and dirty state reflect them

### Requirement: Reconcile automatic dependencies
Setting automatic dependencies SHALL add current dependencies, remove stale automatic-only dependencies, and preserve every explicitly selected ID.

#### Scenario: Dependency ceases to be automatic
- **GIVEN** an ID selected both explicitly and automatically
- **WHEN** it is removed from the automatic dependency set
- **THEN** the ID remains effectively selected

#### Scenario: Stale automatic-only dependency
- **GIVEN** an ID selected only as an automatic dependency
- **WHEN** it is absent from the next dependency set
- **THEN** the ID is no longer effectively selected

### Requirement: Consistent toggling
Cursor toggling SHALL update explicit and effective selection consistently while respecting automatic dependencies.

#### Scenario: Toggle explicit item
- **WHEN** a selectable row is toggled
- **THEN** explicit selection, effective selection, totals, and dirty state agree

