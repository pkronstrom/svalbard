## ADDED Requirements

### Requirement: Reusable content archive pipeline
A content collection SHALL build through reusable source, verification, project, collection, and package blocks rather than a recipe-specific orchestration script.

#### Scenario: Build collection
- **WHEN** a configured source document is built
- **THEN** links are extracted, verified, transformed per project, collected, and packaged into one verified archive

### Requirement: Cached project fan-out
The project map block SHALL cache each project and sub-stage independently by content and rule digest.

#### Scenario: Change render template
- **GIVEN** completed cached fetch, metadata, and asset outputs
- **WHEN** only the render template changes
- **THEN** fetch and asset blocks are reused while render, collect, and package rerun

### Requirement: Declarative site rules
Common site extraction SHALL be expressible through typed domain/fetch/metadata/asset/HTML rules.

#### Scenario: Generic site
- **WHEN** a site is fully described by supported rules
- **THEN** no site-specific Go code is required

### Requirement: Explicit exceptional enrichment
Behavior requiring APIs, authentication, or domain semantics SHALL use a named typed Go enricher.

#### Scenario: Exceptional site
- **WHEN** Printables, Thingiverse, GitHub, or Instructables enrichment runs
- **THEN** the enricher can update project metadata/artifacts but cannot control scheduling or arbitrary effects

### Requirement: MIY parity gate
The MIY recipe SHALL remain on the existing builder until bounded old/new outcome parity and ZIM verification pass.

#### Scenario: MIY cutover
- **WHEN** bounded old/new manifests, metadata, links, assets, rendered pages, and ZIM verification match
- **THEN** the MIY recipe may switch to the generic pipeline
