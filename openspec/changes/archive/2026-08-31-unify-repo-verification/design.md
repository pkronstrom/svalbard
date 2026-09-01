## Context

Verification commands are duplicated between spec-flow and manual practice, while `make test` covers one of five modules.

## Goals / Non-Goals

**Goals:** one root readiness command, complete module coverage, catalog parity, and consistent spec-flow use.

**Non-Goals:** changing build/install paths, runtime CLI parsing, release publishing, or adding a monorepo framework.

## Decisions

- `make verify` checks catalog synchronization and tests all five modules.
- `make test` aliases `make verify`.
- Spec-flow invokes `make verify`.
- Existing CI is unchanged because the repository has no test workflow to consolidate.

## Risks / Trade-offs

- Sequential module tests are slower but deterministic and match the established full matrix.
