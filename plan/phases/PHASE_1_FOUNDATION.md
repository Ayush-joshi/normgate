# Phase 1: Contracts, Repository Foundation, and Security Model

## Outcome

At the end of this phase, contributors can build the repository, validate configuration, generate identical contract types in Go, Python, and TypeScript, and run a mandatory test suite. No runtime policy decision is implemented yet. The output is a stable foundation that later phases can extend without changing the meaning of existing contracts.

## Entry conditions

- Empty or documentation-only repository
- Go 1.25+, Python 3.11+, Node.js 22+, and Docker available in CI
- Architecture decisions in the main roadmap accepted

## Mandatory TDD workflow

Every behavior change follows this sequence:

1. Add or update a requirement ID in `docs/requirements/core.md`.
2. Write a failing unit, contract, or architecture test that references the requirement ID.
3. Run the narrowest test and confirm it fails for the expected reason.
4. Implement the smallest production change that makes the test pass.
5. Refactor while the narrow test remains green.
6. Run the affected package tests, then the full phase gate.
7. Add a regression fixture for every bug found during review.

Tests must assert behavior and public contracts, not private function call order. Generated files are tested through generation-drift checks rather than hand-edited.

## Workstream 1: Bootstrap the repository

### Tests first

- Add `test/architecture/layout_test.go` that asserts all required top-level directories exist.
- Add `test/architecture/dependencies_test.go` that parses Go imports and rejects forbidden dependency directions.
- Add a CI smoke job that initially fails because `make verify` does not exist.

### Implementation

Create:

```text
cmd/normgate/
internal/api/
internal/config/
internal/contracts/
internal/enforcement/
internal/identity/
internal/normalization/
internal/policy/
internal/receipt/
internal/storage/
internal/telemetry/
api/openapi/
schemas/
policies/baseline/
sdk/python/
sdk/typescript/
web/
deployments/
test/architecture/
test/contract/
testkit/fixtures/
docs/architecture/decisions/
docs/requirements/
```

Add:

- `go.mod` and `go.work`
- `.editorconfig`
- `.golangci.yml`
- `Makefile`
- `.github/workflows/verify.yml`
- Dependabot or Renovate configuration
- `CODEOWNERS`
- `SECURITY.md`
- Apache-2.0 `LICENSE`

The initial `Makefile` must expose stable targets:

```text
make generate
make lint
make test-unit
make test-contract
make test-race
make test-fuzz-smoke
make coverage
make verify
```

`make verify` runs generation checks, lint, unit tests, contract tests, race tests, coverage enforcement, and vulnerability scanning.

## Workstream 2: Define canonical schemas

### Tests first

Create invalid and valid fixtures before implementing generated models:

```text
testkit/fixtures/contracts/valid/
testkit/fixtures/contracts/invalid/
testkit/fixtures/contracts/compatibility/
```

Cover:

- Missing schema version
- Unsupported major version
- Unknown operation kind
- Empty principal or tenant identity
- Duplicate classification labels
- Invalid destination properties
- Invalid decision/obligation combinations
- Permit without expiration or operation digest
- Receipt without policy revision
- Unknown fields where strict parsing is required

### Implementation

Create schemas under `schemas/v1/` for:

- `event-envelope.schema.json`
- `principal.schema.json`
- `operation.schema.json`
- `resource.schema.json`
- `destination.schema.json`
- `external-fact.schema.json`
- `decision.schema.json`
- `obligation.schema.json`
- `permit.schema.json`
- `receipt.schema.json`
- `policy-manifest.schema.json`
- `test-case.schema.json`

Rules:

- IDs use opaque strings with explicit maximum lengths.
- Classification and purpose values use hierarchical strings but have no predefined domain values.
- Times use UTC RFC 3339 with required offsets.
- Byte counts and counters use bounded integers.
- Extension metadata lives only in a namespaced `extensions` object.
- Sensitive values are referenced by stable data-reference IDs instead of copied into decision metadata.

Generate Go types into `internal/contracts/v1`, Python models into `sdk/python/src/normgate/types`, and TypeScript types into `sdk/typescript/src/types`.

## Workstream 3: Canonical normalization and hashing

### Tests first

Create cross-language golden vectors containing:

- Reordered JSON keys
- Equivalent Unicode representations
- Empty, absent, and null values
- Boundary integers
- Escaped strings
- Reordered classification input
- Timestamps representing the same instant with different offsets

The Go, Python, and TypeScript implementations must generate the same normalized bytes and SHA-256 digest for every vector.

### Implementation

Implement normalization in `internal/normalization`:

- Sort object keys recursively.
- Normalize strings to Unicode NFC.
- Convert timestamps to UTC.
- Sort set-like fields and preserve sequence-like fields.
- Reject floating-point fields in signed objects.
- Remove explicitly non-authoritative transport metadata before hashing.
- Return typed errors instead of partially normalized output.

Document the algorithm in `docs/architecture/canonicalization.md`. Golden vectors, not prose, are the compatibility authority.

## Workstream 4: Configuration and error contracts

### Tests first

- Table tests for precedence between file configuration, environment references, and command flags.
- Tests for missing secret references, unknown keys, invalid URLs, invalid timeouts, and unsafe fail-open settings.
- Snapshot tests for public error envelopes and stable reason codes.

### Implementation

Implement strict configuration loading in `internal/config`:

- YAML and JSON input
- Environment-variable references using explicit `${ENV:NAME}` syntax
- File-secret references using `${FILE:/mounted/path}` syntax
- Redacted diagnostic output
- Separate validation from loading
- No implicit network access during validation

Define error packages with stable codes for validation, authentication, policy, enforcement, storage, upstream, and internal failures. HTTP status mapping belongs in `internal/api`, not business packages.

## Workstream 5: Threat model and architecture records

Create:

- `docs/security/threat-model.md`
- `docs/architecture/trust-boundaries.md`
- `docs/architecture/decisions/0001-canonical-contracts.md`
- `docs/architecture/decisions/0002-fail-closed.md`
- `docs/architecture/decisions/0003-exact-operation-permits.md`
- `docs/architecture/decisions/0004-minimized-receipts.md`
- `docs/architecture/decisions/0005-versioning.md`

The threat model must identify assets, actors, entry points, trust boundaries, abuse cases, mitigations, residual risks, and the test that verifies each implemented mitigation.

## Test and coverage gates

- `internal/contracts`, `internal/normalization`, and `internal/config`: at least 90% Go statement coverage
- Repository Go statement coverage: at least 85%
- Every schema has valid, invalid, and forward-compatibility fixtures.
- Architecture tests run on every pull request.
- Fuzz smoke tests run for at least 30 seconds per target in CI; scheduled CI runs longer campaigns.
- No skipped test is permitted without an issue URL and expiry date.

## Required validation

```bash
make generate
git diff --exit-code
make lint
make test-unit
make test-contract
make test-race
make test-fuzz-smoke
make coverage
make verify
```

## Suggested pull-request sequence

1. Toolchain, repository layout, and CI
2. JSON Schemas and fixture corpus
3. Generated Go/Python/TypeScript models
4. Canonical normalization and cross-language vectors
5. Configuration and public error contracts
6. Threat model, architecture decisions, and phase gate

## Completion checklist

- [ ] Clean checkout passes `make verify`.
- [ ] Generated artifacts cannot drift silently.
- [ ] Cross-language hashes match every golden vector.
- [ ] Unsupported contract versions fail explicitly.
- [ ] Core import and vocabulary boundaries pass.
- [ ] Coverage gates pass without excluding production packages.
- [ ] Threat-model mitigations reference executable tests where applicable.

