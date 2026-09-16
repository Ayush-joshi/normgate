# Phase 2: Policy Engine and Policy Lifecycle

## Outcome

At the end of this phase, NormGate can compile, test, explain, distribute, atomically activate, and roll back deterministic policy bundles. A CLI user can evaluate canonical events locally and reproduce decisions from immutable policy revisions.

## Entry conditions

- Phase 1 completion checklist is green.
- Canonical schemas and normalization vectors are frozen at v1 pre-release.
- Threat-model findings affecting policy loading are resolved or tracked as release blockers.

## Mandatory TDD workflow

Implement each rule or lifecycle behavior from a failing test case. Policy behavior starts as a YAML scenario in `policies/baseline/tests`; engine behavior starts as a Go table test. Every fixed defect receives both the narrow regression test and, when externally visible, a CLI or API-level test.

No pull request may change an expected policy decision by updating a golden file alone. The pull request must document the reason, show the semantic policy diff, and add a test proving the intended boundary.

## Workstream 1: Policy engine abstraction

### Tests first

Define conformance tests against a fake engine before adding OPA:

- Compile succeeds and returns a content-addressed revision.
- Compile errors include source location without leaking input data.
- Evaluate accepts only canonical `EventEnvelope` values.
- Explain returns matched rule IDs and obligation provenance.
- Close is idempotent.
- Evaluation respects context cancellation and deadlines.

### Implementation

Define in `internal/policy/engine.go`:

```go
type Engine interface {
    Compile(ctx context.Context, bundle Bundle) (Revision, error)
    Evaluate(ctx context.Context, revision Revision, event contracts.EventEnvelope) (contracts.Decision, error)
    Explain(ctx context.Context, revision Revision, event contracts.EventEnvelope) (Explanation, error)
    Health(ctx context.Context) Health
    Close() error
}
```

Keep Rego types inside `internal/policy/opa`. No other package may import the OPA SDK.

## Workstream 2: OPA-backed evaluation

### Tests first

- Engine conformance suite executed against the embedded OPA implementation
- Cancellation and timeout tests
- Concurrent evaluation tests under the race detector
- Malformed and oversized bundle tests
- Tests proving evaluation has no filesystem or network capability
- Benchmark fixtures for cold compile and cached evaluation

### Implementation

- Compile bundles into prepared queries.
- Cache immutable revisions using content digests.
- Place hard limits on bundle bytes, module count, rule depth, and evaluation time.
- Expose only approved deterministic built-ins.
- Reject policies using time, network, random, or other non-replayable inputs; such values must enter as `ExternalFact` records.
- Return structured engine errors mapped to stable public reason codes.

## Workstream 3: Policy composition and conflict resolution

### Tests first

Create pairwise and multi-layer decision tables covering:

- Allow plus deny
- Allow plus approval
- Multiple compatible transformations
- Conflicting field transformations
- Missing decision at one layer
- Unknown obligation type
- Namespace collision
- Duplicate policy priority
- Missing external fact

Property tests must verify that adding a deny cannot weaken a final deny and that input ordering does not change composed output.

### Implementation

Implement explicit layers:

```text
baseline -> organization -> tenant -> application -> local override
```

Each layer produces a partial decision with rule provenance. Compose using these rules:

1. Any applicable deny produces deny.
2. Any applicable approval requirement prevents direct allow.
3. Compatible transformations merge in deterministic order.
4. Conflicting obligations produce deny.
5. No applicable terminal decision produces deny.

Store the full explanation separately from the minimal runtime decision so callers can request diagnostic detail without expanding the enforcement payload.

## Workstream 4: Bundle build and validation

### Tests first

- Manifest schema fixtures
- Digest changes when any authoritative file changes
- Digest does not depend on tar entry order or file modification time
- Dependency cycle and incompatible version tests
- Namespace ownership collision tests
- Signature field validation without implementing remote trust yet

### Implementation

The bundle builder must:

- Normalize file paths and reject traversal.
- Include manifest, Rego modules, static data, reason-code catalog, and tests.
- Generate a deterministic archive.
- Record source and build digests.
- Validate core-version compatibility and dependency constraints.
- Produce an inspection report before activation.

Implement CLI commands:

```bash
normgate policy init
normgate policy lint PATH
normgate policy build PATH --out bundle.tar.gz
normgate policy test PATH
normgate policy inspect bundle.tar.gz
normgate policy diff OLD NEW
normgate policy explain --bundle BUNDLE --event EVENT
```

## Workstream 5: Activation, distribution, and recovery

### Tests first

Use an in-process HTTP server and temporary filesystem to test:

- Successful initial activation
- Failed candidate health check
- Interrupted download
- Checksum mismatch
- ETag no-change response
- Stale-policy deadline
- Atomic reader switch during concurrent evaluations
- Restart from last-known-good bundle
- Rollback to an exact earlier revision

### Implementation

- Implement filesystem, HTTPS, and OCI-compatible source interfaces.
- Download to a staging area, verify, compile, run bundle tests, and activate with an atomic pointer swap.
- Persist the active and last-known-good revisions.
- Never overwrite an active revision in place.
- Expose activation status, last error, source, revision, and refresh time through health APIs.
- Add exponential backoff with jitter and configured maximum staleness.

## Workstream 6: Baseline policy

### Tests first

Create scenarios for:

- Authenticated and unauthenticated principals
- Tenant match and mismatch
- Known and unknown operation kinds
- Registered and unregistered destinations
- Present and missing purpose
- Complete and incomplete enforcement context

### Implementation

Create a strictly generic baseline bundle that denies missing identity, cross-tenant operations, unknown operations, undeclared purposes, and unknown destinations. Include reason codes, explanations, and remediation text intended for developers rather than end users.

## Test and coverage gates

- `internal/policy` and `internal/policy/opa`: at least 90% Go statement coverage
- Every baseline rule has allow, deny, and boundary scenarios.
- Engine conformance suite runs against fake and OPA implementations.
- Race tests cover simultaneous evaluation and activation.
- Fuzz targets cover bundle parsing, manifest parsing, composition, and explanation serialization.
- Benchmarks publish cold compile time, cached evaluation p50/p95/p99, and allocation counts.
- Mutation testing is run on conflict resolution and permit-independent decision composition before phase completion.

## Required validation

```bash
make test-unit PKG=./internal/policy/...
make test-contract
make test-race
make test-fuzz-smoke
make coverage
go test -bench=. -benchmem ./internal/policy/...
normgate policy test policies/baseline
make verify
```

## Suggested pull-request sequence

1. Engine interface, fake engine, and conformance suite
2. Embedded OPA implementation and deterministic built-in restrictions
3. Composition and conflict resolution
4. Deterministic bundle builder and CLI
5. Activation, refresh, rollback, and recovery
6. Baseline policy and performance gate

## Completion checklist

- [x] All policy changes are covered by scenario tests.
- [x] Cached evaluation meets the phase latency target.
- [x] Policy activation is atomic under concurrent load.
- [x] Invalid updates cannot replace the active revision.
- [x] Restart and rollback reproduce prior decisions exactly.
- [x] Explanations identify every contributing rule and obligation.
- [x] Full repository verification remains green.


Completed and locally verified on 2026-09-17. See the [Phase 2 validation record](../../docs/validation/phase-2.md) for coverage, benchmarks, gate results and remaining phase boundaries.
