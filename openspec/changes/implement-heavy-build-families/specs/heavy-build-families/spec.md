## ADDED Requirements

### Requirement: Concrete heavy-family handlers
Every retained reference, ZIM, crawl, and geodata family SHALL have a concrete Go handler using only typed tools capabilities.

#### Scenario: Valid family recipe
- **WHEN** a retained heavy-family recipe is applied
- **THEN** its handler produces one verified artifact and manifest entry with resumable state

#### Scenario: Invalid family recipe
- **WHEN** required family fields are missing
- **THEN** validation fails before network, container, or filesystem effects

#### Scenario: Browser crawl
- **WHEN** a zimit recipe executes
- **THEN** only the pinned browser image runs and the completed ZIM works from the standalone drive
