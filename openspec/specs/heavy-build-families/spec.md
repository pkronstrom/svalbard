# heavy-build-families Specification

## Purpose
TBD - created by archiving change implement-heavy-build-families. Update Purpose after archive.
## Requirements
### Requirement: Concrete heavy-family handlers
Fimea reference-static, compact-ZIM, zimit routing, vector-static, and vector-service recipes SHALL use concrete Go-controlled handlers and typed tool capabilities.

#### Scenario: Valid family recipe
- **WHEN** a family in this slice is applied
- **THEN** its handler produces one verified artifact and manifest entry or routes its typed procedures to the correct pinned tools target

#### Scenario: Invalid family recipe
- **WHEN** required family fields are missing
- **THEN** validation fails before network, container, or filesystem effects

#### Scenario: Browser crawl routing
- **WHEN** a zimit tool procedure executes
- **THEN** the recipe cannot select an arbitrary image and Go routes the tool to the pinned browser target

