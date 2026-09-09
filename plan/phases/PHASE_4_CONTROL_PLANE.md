# Phase 4: Complete Control Plane, State, Simulation, and Operator Experience

## Outcome

At the end of this phase, NormGate is a complete domain-agnostic product. An operator can register tenants and applications, activate policy, enforce operations, inspect lineage, manage approvals, replay decisions, simulate changes, roll out policy safely, and export evidence through supported SDKs and the web console.

## Entry conditions

- Phase 3 end-to-end enforcement path is green.
- Permit and receipt formats have passed security-focused mutation tests.
- SQLite local mode can execute and verify a complete request.

## Mandatory TDD workflow

Each control-plane feature begins with an API or repository acceptance test describing the observable state transition. Persistence implementations must pass a shared conformance suite. UI work begins with component behavior tests and is completed by browser-level tests against the real API.

Database migrations require forward, rollback, and data-preservation tests. Authorization tests must cover every role against every administrative action; testing only permitted roles is not sufficient.

## Workstream 1: Persistence architecture

### Tests first

Define repository conformance suites for:

- Tenants and applications
- Principals and credentials
- Destinations
- Policy revisions and assignments
- External facts
- Sessions and lineage
- Decisions, permits, and receipts
- Approvals
- Retention jobs
- Administrative events

Run the same suite against SQLite and PostgreSQL. Add transaction rollback, unique constraint, pagination, concurrent update, migration, and time precision tests.

### Implementation

- Define repository interfaces in `internal/storage`.
- Use explicit SQL migrations checked into `internal/storage/migrations`.
- Keep queries visible and reviewable; generated query code is acceptable if generated deterministically.
- Add optimistic concurrency versions to mutable control-plane records.
- Partition tenant-owned tables logically from the first migration.
- Ensure receipt writes and nonce consumption use transactions appropriate to each backend.

## Workstream 2: Distributed coordination and caching

### Tests first

- Permit nonce is consumed once across concurrent service instances.
- Rate limits remain correct under concurrency.
- Cache keys include tenant, policy revision, destination revision, and external-fact digest.
- Redis loss produces configured fail-closed behavior.
- Local in-process implementation passes the same behavioral suite.

### Implementation

Use Redis for:

- Nonce replay prevention
- Distributed rate limits
- Short-lived decision cache
- Policy rollout coordination
- Approval notifications
- Distributed locks for singleton maintenance work

Do not cache decisions containing one-time approvals or non-cacheable external facts.

## Workstream 3: Tenant, identity, and administrative authorization

### Tests first

Build a role/action matrix with explicit allow and deny cases for registration, credential rotation, policy authoring, activation, approval, audit viewing, evidence export, and break-glass access.

Add tests for:

- Cross-tenant object references
- Revoked credentials
- Credential rotation overlap and expiry
- Policy author attempting self-approval
- Confused-deputy operations
- Audit access without payload access

### Implementation

- Add tenant and application registration APIs.
- Support scoped service credentials and workload-identity claims.
- Separate policy author, policy approver, runtime operator, evidence reviewer, and system administrator capabilities.
- Record every administrative mutation as a chained administrative receipt.
- Require explicit configuration for break-glass roles and expiry.

## Workstream 4: Data lineage and governed memory

### Tests first

Create graph fixtures for ingress, derivation, transformation, disclosure, memory write, memory read, and deletion. Test cycles, missing parents, tenant crossing, partial deletion, and concurrent updates.

Property tests must verify that derived data cannot silently lose a classification unless a recorded transformation explicitly permits that change.

### Implementation

- Assign stable data-reference IDs at ingress.
- Store lineage edges with source, destination, operation, transformation, decision, and timestamp.
- Propagate classifications by default; transformations produce explicit predecessor/successor edges.
- Store memory metadata: source, subject references, verification state, purposes, tenant, retention deadline, and derivative references.
- Treat memory read, write, update, and delete as separate enforceable operations.
- Implement deletion plans that enumerate all known derivatives before execution and receipts that record each store result.

## Workstream 5: Stateful policy evaluation

### Tests first

Create multi-event scenarios for:

- Cumulative disclosure below and above configured limits
- Purpose changes during a session
- Repeated retries
- Tool frequency
- Resource totals
- Recursion and delegation depth
- Session expiry and restart

### Implementation

- Add bounded session summaries as explicit policy inputs.
- Update state transactionally after enforcement results.
- Make counter windows and reset behavior deterministic.
- Prevent stale cached state from authorizing an operation beyond a limit.
- Allow policies to request approval or denial based on aggregate state.

## Workstream 6: Approval workflows

### Tests first

- Request, approve, reject, cancel, delegate, expire, and consume
- Two reviewers racing on one request
- Changed operation after approval
- Revoked approver
- Required reviewer group not satisfied
- Break-glass operation and mandatory reason

### Implementation

- Persist approval requests against the exact operation digest and policy revision.
- Support reviewer groups, quorum count, comments, expiry, and notification hooks.
- Issue a new one-operation permit only after approval and immediate policy reevaluation.
- Never mutate an old denied or pending receipt into an allowed receipt; append the subsequent decision.

## Workstream 7: Simulation, replay, and rollout

### Tests first

- Replaying an immutable receipt reproduces its original decision.
- Missing authoritative input returns `inconclusive`, not a guessed result.
- Policy comparison groups newly allowed, denied, transformed, and approval-required cases.
- Shadow decisions cannot be consumed as permits.
- Canary assignment is stable for a tenant/application key.
- Emergency revoke overrides scheduled and cached revisions.

### Implementation

Implement:

```bash
normgate replay --receipt RECEIPT --policy REVISION
normgate simulate --events INPUT --current CURRENT --candidate CANDIDATE
normgate rollout start --policy REVISION --mode shadow
normgate rollout promote --percentage 10
normgate rollout revoke --policy REVISION
```

Persist observed, simulated, and enforced decisions as distinct record types. Reports must include missing evidence and affected applications.

## Workstream 8: Evidence export

### Tests first

- Golden JSON and HTML outputs
- Redaction and payload-minimization tests
- Audit-chain verification before export
- Stable references from control to policy, test, decision, approval, and outcome
- Large-range pagination and cancellation

### Implementation

Generate evidence bundles containing:

- Export manifest and digests
- Active configuration and policy revisions
- Policy test results
- Decision and enforcement summaries
- Exceptions and approvals
- Failed enforcement attempts
- Audit-chain verification result
- Assurance limitations and missing evidence

## Workstream 9: SDKs

### Tests first

- Generated model compatibility tests
- Mock-server API tests
- Retry and timeout behavior
- Authentication injection
- Error mapping
- Async cancellation
- Streaming
- No implicit logging of payloads

### Implementation

- Generate low-level Python and TypeScript clients from OpenAPI.
- Add maintained ergonomic wrappers for decision, enforcement, inference, tool, retrieval, response, memory, simulation, and approval APIs.
- Keep retry behavior explicit and safe for non-idempotent operations.
- Publish local prerelease packages from CI for end-to-end testing.

## Workstream 10: Web console and operations

### Tests first

Use Vitest and Testing Library for components, then Playwright against a real service for:

- Login and tenant switching
- Policy diff and activation
- Decision and receipt inspection
- Approval processing
- Simulation comparison
- Lineage navigation
- Evidence export
- Authorization-denied UI behavior

### Implementation

- Build a React/TypeScript console generated from the same API contract.
- Treat the console as an untrusted API client; enforce authorization server-side.
- Add operational health, policy staleness, storage state, exporter state, and failed enforcement views.
- Add backup, restore, retention, migration, and recovery CLI workflows with runbooks.

## Test and coverage gates

- New Go control-plane packages: at least 85% statement coverage
- Security-critical tenant, approval, lineage, and deletion code: at least 90%
- Python and TypeScript maintained SDK code: at least 85%
- React statements/branches/functions/lines: at least 80%, with 100% coverage for authorization guards
- SQLite and PostgreSQL pass the same repository suite.
- Playwright covers every critical operator journey.
- Integration tests use disposable PostgreSQL and Redis instances and run in CI.

## Required validation

```bash
make test-unit
make test-contract
make test-integration
make test-e2e
make test-ui
make test-race
make coverage
make db-migration-test
make verify
```

## Suggested pull-request sequence

1. Repository interfaces, migrations, and backend conformance
2. Redis coordination and cache correctness
3. Tenant administration, identity, and RBAC
4. Lineage and memory governance
5. Stateful policy evaluation
6. Approval workflows
7. Simulation, replay, and rollout
8. Evidence export and SDKs
9. Web console and operational workflows

## Completion checklist

- [ ] A clean deployment completes the entire configure-to-enforce-to-evidence journey.
- [ ] SQLite and PostgreSQL behavior matches.
- [ ] Cross-tenant and administrative authorization matrices pass.
- [ ] Lineage classification propagation and deletion are tested end to end.
- [ ] Simulation never produces an enforceable permit.
- [ ] UI critical paths pass browser tests against the real API.
- [ ] All language and UI coverage gates pass.
- [ ] Full repository verification passes.

