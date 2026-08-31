## ADDED Requirements

### Requirement: Shared search backend
Terminal and menu search SHALL use the same Session-owned database, engine selection, fallback, result-opening, and child-process lifecycle.

#### Scenario: Initial terminal query
- **GIVEN** terminal search starts with a non-empty initial query
- **WHEN** the adapter begins
- **THEN** it searches through the Session before prompting for another query

#### Scenario: Mode command
- **GIVEN** an active terminal Session
- **WHEN** the user enters `/fts`, `/keyword`, `/sem`, `/semantic`, `/hybrid`, or `/full`
- **THEN** subsequent searches use the corresponding existing mode

#### Scenario: Hybrid fallback
- **GIVEN** hybrid or semantic search cannot use the embedding backend
- **WHEN** Session falls back to keyword search
- **THEN** results are returned and the terminal displays the fallback status

#### Scenario: Open numbered result
- **GIVEN** displayed search results
- **WHEN** the user selects a valid result number
- **THEN** Session opens that result through its configured opener

#### Scenario: Clean exit
- **GIVEN** terminal search owns a Session
- **WHEN** the user quits or the terminal adapter returns an error
- **THEN** the database and Session-owned child processes are closed exactly once

#### Scenario: Stable entry points
- **WHEN** search is opened from the drive menu or the native search action
- **THEN** the existing routing and action identifiers remain valid
