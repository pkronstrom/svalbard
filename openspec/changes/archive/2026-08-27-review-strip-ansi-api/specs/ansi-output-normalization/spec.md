## ADDED Requirements

### Requirement: ANSI stripping compatibility
The reusable TUI module SHALL preserve exported ANSI-stripping behavior unless downstream review establishes that removal is compatible with the project's API policy.

#### Scenario: External use is possible
- **GIVEN** the module is reusable or published and downstream use cannot be ruled out
- **WHEN** `StripAnsi` is reviewed for removal
- **THEN** the exported function remains available

#### Scenario: Removal is proven safe
- **GIVEN** the module is private or an API break is explicitly acceptable and no downstream caller requires the export
- **WHEN** `StripAnsi` has no production purpose
- **THEN** the unused production export may be removed or made private while test assertions retain needed normalization
