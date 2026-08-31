# mechanical-simplification Specification

## Purpose
TBD - created by archiving change simplify-mechanical-duplication. Update Purpose after archive.
## Requirements
### Requirement: Production palette matching
The command palette SHALL discover entries by case-insensitive ID, label, or alias matching without verb or freeform transport.

#### Scenario: Matching an alias
- **GIVEN** a palette entry with an alias
- **WHEN** the query matches that alias regardless of case
- **THEN** the entry is selectable with no extra freeform argument

### Requirement: Bounded navigation
Navigation SHALL move one item within list bounds and safely handle empty or out-of-range selection.

#### Scenario: Lower boundary
- **GIVEN** the last navigation item is selected
- **WHEN** navigation moves down
- **THEN** selection remains on the last item

### Requirement: Image conversion
Image conversion SHALL return RGBA pixels with source bounds preserved.

#### Scenario: Convert non-zero bounds
- **GIVEN** a non-RGBA image whose bounds do not start at zero
- **WHEN** it is converted
- **THEN** the result has identical bounds and pixel colors

#### Scenario: Existing RGBA image
- **GIVEN** an RGBA image
- **WHEN** it is converted
- **THEN** the same image value is returned

### Requirement: Builder dispatch precedence
Builder resolution SHALL prefer explicit pipeline steps, then `python-venv`, then app-bundle conversion, then no native builder.

#### Scenario: Explicit steps for Python family
- **GIVEN** a `python-venv` recipe with explicit pipeline steps
- **WHEN** its builder is resolved
- **THEN** explicit steps win over the family-specific builder

