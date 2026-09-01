## ADDED Requirements

### Requirement: Packaged binary resolution
Packaged binary resolution SHALL detect the current host platform internally while preserving existing path precedence and archive extraction.

#### Scenario: Resolve packaged binary
- **WHEN** a caller resolves a named drive binary
- **THEN** the resolver searches the current platform's tool-specific directory, platform directory, generic tool directory, generic bin directory, and PATH in the existing order

#### Scenario: Resolve packaged archive
- **WHEN** a supported archive contains the requested executable
- **THEN** extraction and executable discovery behave as before the contract simplification
