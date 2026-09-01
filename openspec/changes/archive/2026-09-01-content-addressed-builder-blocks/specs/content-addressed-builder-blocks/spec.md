## ADDED Requirements

### Requirement: Content-addressed block output
A cacheable build procedure SHALL key its output by normalized procedure parameters and input content.

#### Scenario: Cache miss
- **WHEN** no valid cache entry matches a procedure and its input directory digests
- **THEN** the procedure runs, its output is verified, and an atomic cache entry with manifest is published

#### Scenario: Cache hit
- **WHEN** procedure parameters and input digests match a valid cache entry
- **THEN** cached output is restored and the procedure emits a skipped event without executing effects

#### Scenario: Input changes
- **WHEN** any declared input file content changes
- **THEN** the cache key changes and the procedure runs again

### Requirement: Inspectable directory edge
Cached block state SHALL be ordinary directories plus a small JSON manifest.

#### Scenario: Inspect failed build
- **WHEN** a block fails
- **THEN** its input, work, partial output, and logs remain inspectable under recipe staging

### Requirement: Standalone final artifact
Final drive output SHALL NOT depend on cache symlinks.

#### Scenario: Complete build
- **WHEN** cached output is promoted to its final drive destination
- **THEN** removing `.staging/cache` does not break the final artifact
