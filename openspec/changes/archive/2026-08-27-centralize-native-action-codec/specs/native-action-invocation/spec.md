## ADDED Requirements

### Requirement: Native invocation round trip
Every built-in action SHALL encode from a stable action ID and named arguments to hidden-command argv and decode back to the same invocation.

#### Scenario: Required and optional arguments
- **GIVEN** a built-in action with required and optional arguments
- **WHEN** it is encoded and decoded
- **THEN** required values, supplied optional values, and established defaults are preserved

#### Scenario: Optional argument omitted
- **GIVEN** a built-in action invocation omitting an optional argument
- **WHEN** it is decoded
- **THEN** the existing default value is available by its named argument

### Requirement: Protocol validation
The codec SHALL reject unknown protocol values and malformed positional arguments.

#### Scenario: Unknown stable action ID
- **WHEN** encoding is requested for an unknown action ID
- **THEN** the codec returns an error

#### Scenario: Unknown hidden subcommand
- **WHEN** decoding is requested for an unknown hidden subcommand
- **THEN** the codec reports that no native invocation matched

#### Scenario: Missing required argument
- **WHEN** decoding a known subcommand without a required positional value
- **THEN** the codec returns a validation error

#### Scenario: Extra positional argument
- **WHEN** decoding a known subcommand with unsupported extra values
- **THEN** the codec returns a validation error

### Requirement: Execution separation
Command `main` SHALL execute decoded stable actions without knowing their positional wire format.

#### Scenario: Dispatch built-in action
- **GIVEN** a decoded native invocation
- **WHEN** `main` dispatches it
- **THEN** the implementation receives named decoded arguments
