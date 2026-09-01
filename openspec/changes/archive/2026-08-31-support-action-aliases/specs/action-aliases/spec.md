## ADDED Requirements

### Requirement: Generated action aliases
A recipe menu MAY define aliases and generated drive configuration SHALL preserve them.

#### Scenario: Generate custom alias
- **GIVEN** a realized recipe whose menu defines aliases
- **WHEN** toolkit generation writes `actions.json`
- **THEN** the corresponding item contains those aliases unchanged

#### Scenario: Invoke generated alias
- **GIVEN** a generated item alias
- **WHEN** the drive runtime receives that alias as a command
- **THEN** it resolves and executes the item's configured action

#### Scenario: Discover generated alias
- **GIVEN** a generated item alias
- **WHEN** the command palette query matches it regardless of case
- **THEN** the corresponding item is shown
