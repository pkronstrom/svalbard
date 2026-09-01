# runtime-surface-hygiene Specification

## Purpose
TBD - created by archiving change prune-dead-runtime-apis. Update Purpose after archive.
## Requirements
### Requirement: Production-owned package surfaces
Runtime packages SHALL expose only APIs required by production behavior or a documented external contract.

#### Scenario: Remove superseded APIs
- **GIVEN** an exported method has no production caller and a narrower production replacement
- **WHEN** the cleanup is applied
- **THEN** tests use the production replacement and the obsolete method is removed

#### Scenario: Preserve behavior
- **WHEN** dead wrappers, test introspection, and allocating helpers are removed
- **THEN** search indexing, MCP tools, menu movement, dashboard rendering, and source inspection retain their observable behavior

