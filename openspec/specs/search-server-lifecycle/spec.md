# search-server-lifecycle Specification

## Purpose
TBD - created by archiving change consolidate-search-launchers. Update Purpose after archive.
## Requirements
### Requirement: Ready-only launch result
Concrete embedding and Kiwix launchers SHALL return a process and port only after the service is reachable.

#### Scenario: Successful startup
- **GIVEN** a child process that binds its configured listener
- **WHEN** readiness succeeds before timeout
- **THEN** the launcher returns the live process and bound port

#### Scenario: Startup timeout
- **GIVEN** a child process that never becomes ready
- **WHEN** startup reaches its timeout
- **THEN** the child is killed, waited for, and an error is returned

#### Scenario: Child exits during startup
- **GIVEN** a child process that exits before readiness
- **WHEN** startup observes the exit
- **THEN** the launcher returns the child-process error without leaking the process

### Requirement: Explicit caller ownership
Shared launch protocol SHALL NOT merge Session and MCP process lifetimes.

#### Scenario: Session closes
- **WHEN** a search Session closes
- **THEN** only processes owned by that Session terminate

#### Scenario: Repeated MCP search
- **GIVEN** MCP has started a cached search service
- **WHEN** another MCP request needs the same service
- **THEN** it reuses the cached process

#### Scenario: MCP closes
- **WHEN** MCP closes
- **THEN** its cached processes terminate and are waited for

### Requirement: Search fallback
Embedding launch failure SHALL preserve existing keyword fallback behavior.

#### Scenario: Embedding unavailable
- **WHEN** semantic or hybrid search cannot start the embedding service
- **THEN** search reports fallback status and returns keyword results

