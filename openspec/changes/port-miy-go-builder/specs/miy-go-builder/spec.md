## ADDED Requirements

### Requirement: Resumable MIY build
The MIY recipe SHALL build through Go orchestration while preserving verified crawl state across interruptions and host changes.

#### Scenario: Resume interrupted crawl
- **GIVEN** verified and failed project states in vault staging
- **WHEN** apply resumes
- **THEN** verified work is reused and incomplete work is retried according to current policy

#### Scenario: Browser fallback
- **WHEN** a project cannot be captured through Go HTTP processing and qualifies for browser fallback
- **THEN** only the typed browser capability executes and its result returns to Go state management

#### Scenario: Complete archive
- **WHEN** the build completes
- **THEN** the ZIM passes verification and preserves agreed project, metadata, asset, and link parity with the existing builder
