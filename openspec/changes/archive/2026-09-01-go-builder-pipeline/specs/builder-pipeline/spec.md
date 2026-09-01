## ADDED Requirements

### Requirement: Typed build recipe data
The catalog SHALL preserve typed build assets, tables, tool requirements, network use, and transfer/staging estimates from YAML.

#### Scenario: Parse structured build fields
- **GIVEN** a recipe with asset or table arrays and resource declarations
- **WHEN** the catalog loads the recipe
- **THEN** every declared value is available to its family handler without lossy string conversion

### Requirement: Linear procedure execution
Build steps SHALL compile into typed procedures and execute in YAML order.

#### Scenario: Existing linear recipe
- **GIVEN** a recipe using the existing ordered `steps` list
- **WHEN** it is built
- **THEN** procedures execute in the same order and produce one verified artifact

#### Scenario: Resume interrupted build
- **GIVEN** completed procedures with matching input fingerprints in vault staging
- **WHEN** apply retries the recipe
- **THEN** valid completed procedures are skipped and execution resumes at the first incomplete or stale procedure

### Requirement: Recipe dependency ordering
Apply SHALL realize recipe dependencies before their dependents.

#### Scenario: Build portable Python tools
- **GIVEN** selected Python-package recipes
- **WHEN** apply runs
- **THEN** uv is available before the Python environment is built, and that environment is available before packages are installed

### Requirement: Isolated build tools
Non-Go provisioning dependencies SHALL run only through Svalbard-controlled pinned tools images.

#### Scenario: Execute base tool
- **WHEN** a procedure requires uv, zimwriterfs, GDAL, tippecanoe, ffmpeg, or another base capability
- **THEN** Go invokes the pinned base image with only vault/staging mounts and a typed command mapping

#### Scenario: Execute browser tool
- **WHEN** a procedure requires zimit or browser crawling
- **THEN** Go invokes the pinned browser image layered on the same base

#### Scenario: Consume completed drive
- **WHEN** the finished drive is used on a supported host
- **THEN** its launcher and selected runtime tools work without Docker or build images

### Requirement: Build resource disclosure
Plan and apply SHALL disclose build network and storage estimates before execution.

#### Scenario: Plan network build
- **GIVEN** a build recipe declaring network use and estimated transfer/work sizes
- **WHEN** the plan is rendered
- **THEN** it identifies network requirement, approximate download GB, staging GB, and tools-image requirement

### Requirement: Shared build events
CLI and TUI SHALL consume the same typed build-event contract.

#### Scenario: Build progress
- **WHEN** procedures execute
- **THEN** queued, started, progress, log, completed, failed, and skipped states are representable as JSON Lines events

### Requirement: Honest build family dispatch
The foundation SHALL execute implemented families and reject other declarations before checkout-relative script or filesystem effects.

#### Scenario: Unsupported family
- **WHEN** a recipe declares a family not implemented in the foundation
- **THEN** validation fails with the recipe and family identified

#### Scenario: Foundation family
- **WHEN** an app-bundle, explicit pipeline, or python-venv recipe is selected
- **THEN** its Go handler executes through the shared procedure/tool boundaries
