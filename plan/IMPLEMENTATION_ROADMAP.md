# NormGate End-to-End Implementation Roadmap

## Product definition

NormGate is a self-hostable, domain-agnostic policy enforcement and evidence runtime for AI applications and agents. It intercepts model requests, retrieval, tool calls, responses, and memory operations; normalizes them into one event contract; makes deterministic policy decisions; enforces the resulting obligations; and records verifiable receipts.

The core contains no industry vocabulary, regulatory assumptions, provider-specific business logic, or hard-coded application workflows. Core development is completed and exercised end to end before domain extensions begin in Phase 5.

## Product surfaces included

- `normgate` CLI for configuration, policy development, testing, simulation, audit verification, and operations
- Headless decision and enforcement service
- OpenAI-compatible inference gateway
- Generic tool and MCP enforcement gateway
- Retrieval and memory enforcement APIs
- Policy lifecycle, distribution, versioning, rollout, and rollback
- Identity, tenancy, approval, and authorization context
- Data classification, transformation, propagation, and lineage
- Signed capability permits tied to exact operations
- Tamper-evident decision and enforcement receipts
- SQLite local mode and PostgreSQL production mode
- Web console for policies, decisions, approvals, simulations, and evidence
- Python and TypeScript client SDKs
- OpenTelemetry traces, metrics, and structured events
- Extension SDK, signed package format, and package registry introduced in Phase 5
- Container, Docker Compose, Kubernetes, and Helm deployment assets

## Detailed phase plans

### Implementation progress

Phase 1 is implemented and locally verified as of 2026-09-15. Its full local gate
passes, including 39 cross-language canonicalization vectors, 99 contract fixtures,
race and fuzz tests, dependency checks, and 94.55% overall Go statement coverage.
See the [Phase 1 validation record](../docs/validation/phase-1.md) for evidence and
scope. Hosted CI status is tracked separately in
[GitHub Actions](https://github.com/Ayush-joshi/normgate/actions/workflows/verify.yml).

Phase 2 implements deterministic OPA evaluation, policy composition, bundle and CLI
tooling, atomic lifecycle management, distribution sources and the generic baseline.
See the [Phase 2 validation record](../docs/validation/phase-2.md). Phase 3 is next;
Phases 3–6 have not started. Protected downstream operations are not executed yet.

### Execution order

Execute phases in order. A phase begins only after the previous phase's required validation and completion checklist pass. Each detailed plan defines its own test-first sequence, file-level workstreams, coverage gates, validation commands, and suggested pull-request boundaries.

1. [Phase 1: Foundation](phases/PHASE_1_FOUNDATION.md)
2. [Phase 2: Policy engine](phases/PHASE_2_POLICY_ENGINE.md)
3. [Phase 3: Enforcement runtime](phases/PHASE_3_ENFORCEMENT_RUNTIME.md)
4. [Phase 4: Control plane](phases/PHASE_4_CONTROL_PLANE.md)
5. [Phase 5: Extensions and domain packages](phases/PHASE_5_EXTENSIONS_AND_DOMAIN_PACKS.md)
6. [Phase 6: Production release](phases/PHASE_6_PRODUCTION_RELEASE.md)

## Fixed technical choices

- Runtime and CLI: Go 1.26+ (OPA v1.20.1 requirement; CI uses Go 1.26.8)
- Policy evaluation: embedded Open Policy Agent/Rego behind an internal `PolicyEngine` interface
- Public API: HTTP/JSON described by OpenAPI 3.1
- Contract source of truth: JSON Schema Draft 2020-12
- Local persistence: SQLite
- Production persistence: PostgreSQL
- Cache and distributed coordination: Redis, optional in single-node mode
- Telemetry: OpenTelemetry Protocol
- Web console: React and TypeScript
- Client SDKs: Python 3.11+ and TypeScript 5+
- Artifact distribution: OCI registries with signed immutable versions
- Deployment: single binary, container image, Docker Compose, and Helm
- Default enforcement behavior: deny when an authoritative decision cannot be obtained

## Core invariants

1. Models and classifiers may produce facts; only deterministic policy grants authority.
2. Every protected operation is checked at the final enforcement point immediately before execution.
3. Authorization is bound to the exact identity, arguments, destination, data labels, policy revision, and expiration.
4. Unknown operations, invalid policies, missing context, and unavailable enforcement dependencies fail closed.
5. Sensitive payloads are not stored in receipts or telemetry unless an explicit retention policy permits them.
6. Every decision is replayable from versioned inputs, policy, configuration, and external facts.
7. All public contracts are versioned independently from implementations.
8. No domain-specific package may be imported by the core runtime.

## Canonical core model

- `Principal`: user, service, workload, or agent identity; tenant; roles; authentication claims.
- `Operation`: model invocation, retrieval, tool call, response emission, or memory read/write/update/delete.
- `Resource`: data or capability being accessed, identified without domain interpretation.
- `Classification`: opaque hierarchical labels assigned to data and resources.
- `Purpose`: declared reason for an operation.
- `Destination`: recipient, service, store, model, region, and declared handling properties.
- `Context`: session, workflow, trace, application, environment, and delegation identifiers.
- `ExternalFact`: versioned input resolved from an authoritative system at decision time.
- `Decision`: `allow`, `deny`, `transform`, or `require_approval`.
- `Obligation`: transformations and conditions an enforcement point must satisfy.
- `Permit`: short-lived signed authorization for one normalized operation.
- `Receipt`: decision inputs, versions, result, enforcement outcome, and replay metadata.

## Phase 1 - Contracts, repository foundation, and security model

Detailed execution plan: [PHASE_1_FOUNDATION.md](phases/PHASE_1_FOUNDATION.md)

### Objective

Establish the complete domain-neutral vocabulary, compatibility rules, repository boundaries, and security invariants on which every later component depends.

### Implementation

1. Create the monorepo:

   ```text
   cmd/normgate/
   internal/api/
   internal/contracts/
   internal/identity/
   internal/config/
   internal/normalization/
   internal/policy/
   internal/enforcement/
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
   testkit/
   docs/
   ```

2. Initialize Go workspaces, linting, formatting, unit-test, race-test, vulnerability-scan, license-check, and generated-code verification jobs.
3. Define JSON Schemas for `EventEnvelope`, `Principal`, `Operation`, `Resource`, `Destination`, `ExternalFact`, `Decision`, `Obligation`, `Permit`, `Receipt`, `PolicyManifest`, and `TestCase`.
4. Generate Go types, OpenAPI components, Python models, and TypeScript types from the schemas. CI must fail when generated artifacts are stale.
5. Specify canonical JSON normalization: stable key ordering, timestamp format, number representation, Unicode normalization, absent-versus-null handling, and SHA-256 digest construction.
6. Define stable reason-code namespaces, error envelopes, request correlation, pagination, idempotency keys, and API version negotiation.
7. Define trust boundaries and a threat model for callers, the service, policy sources, external-fact sources, persistence, downstream systems, and administrators.
8. Implement configuration loading from files, environment references, and secret references with strict schema validation and unknown-field rejection.
9. Create architecture tests that reject imports from extension directories into `internal/` and reject domain vocabulary in core schemas.
10. Publish architecture decision records for determinism, fail-closed behavior, permit binding, audit minimization, and schema compatibility.

### Deliverables

- Compiling monorepo with mandatory CI checks
- Versioned canonical contracts and generated language types
- OpenAPI skeleton and configuration schema
- Initial threat model and architecture decisions
- Enforced dependency and vocabulary boundaries

### Exit criteria

- Canonical serialization produces identical digests across Go, Python, and TypeScript fixtures.
- All public objects reject unsupported major schema versions.
- No domain term or dependency exists in the core contracts or runtime.
- CI verifies formatting, generation, unit tests, race tests, dependency boundaries, and vulnerabilities.

## Phase 2 - Policy engine and policy lifecycle

Detailed execution plan: [PHASE_2_POLICY_ENGINE.md](phases/PHASE_2_POLICY_ENGINE.md)

### Objective

Deliver deterministic policy evaluation and the full lifecycle required to author, validate, distribute, activate, and roll back policy safely.

### Implementation

1. Implement the internal `PolicyEngine` interface with `Compile`, `Evaluate`, `Explain`, `Health`, `Revision`, and `Close` operations.
2. Implement an embedded OPA engine while keeping all service and enforcement code dependent only on `PolicyEngine`.
3. Define policy inputs solely in terms of the canonical core model. Policy output must conform to the `Decision` and `Obligation` schemas.
4. Implement policy composition across baseline, organization, tenant, application, and local overlays.
5. Define deterministic conflict handling: deny overrides every decision; approval overrides allow; compatible transformations merge; incompatible obligations deny with a conflict reason.
6. Implement policy manifests containing identity, version, minimum core version, owned namespaces, dependencies, source digest, build digest, signing identity, and creation time.
7. Implement compile-time checks for unreachable rules, unknown reason codes, invalid obligations, missing defaults, namespace collisions, cyclic dependencies, and undeclared external facts.
8. Implement policy unit tests and table-driven scenario tests with exact decision, obligation, reason-code, and explanation assertions.
9. Add `normgate policy init`, `lint`, `build`, `test`, `diff`, `inspect`, and `explain` commands.
10. Implement atomic policy activation, health validation, last-known-good persistence, hot reload, rollback, and startup recovery.
11. Implement filesystem, HTTPS, and OCI policy sources with checksum pinning, ETag support, bounded refresh, backoff, and stale-policy limits.
12. Create a generic baseline policy covering authenticated principals, tenant consistency, declared purposes, known operations, destination registration, and default denial.

### Deliverables

- Embedded deterministic policy engine
- Policy authoring, build, test, diff, and explanation tooling
- Atomic activation and rollback
- Versioned policy-source and distribution support
- Generic baseline policy

### Exit criteria

- Identical normalized inputs and policy revisions always produce identical decisions.
- Invalid or conflicting policy cannot become active.
- Failed updates retain the last-known-good revision and emit an operational alert.
- Cached evaluations complete below 5 ms p95 on the published fixture set.
- Policy rollback restores the exact prior decision behavior in replay tests.

## Phase 3 - Enforcement data plane, gateways, and receipts

Detailed execution plan: [PHASE_3_ENFORCEMENT_RUNTIME.md](phases/PHASE_3_ENFORCEMENT_RUNTIME.md)

### Objective

Build the complete runtime path from intercepted operation through decision, transformation, downstream execution, response validation, and receipt creation.

### Implementation

1. Implement the service endpoints:

   ```text
   POST /v1/decisions
   POST /v1/permits/verify
   POST /v1/enforcements
   POST /v1/responses/evaluate
   GET  /v1/policies/status
   GET  /v1/health/live
   GET  /v1/health/ready
   ```

2. Add service authentication using workload identities or scoped API credentials. Resolve tenant and principal from authenticated context rather than trusting caller-supplied headers.
3. Issue Ed25519-signed permits bound to normalized operation digest, principal, tenant, destination, classifications, purpose, policy revision, external-fact digest, nonce, issued time, and expiration.
4. Implement final-point permit verification. Reject replay, expiration, argument changes, target changes, identity changes, destination changes, policy invalidation, and missing obligations.
5. Implement generic obligations: remove JSON paths, retain only allowed paths, tokenize values, mask values, cap records or bytes, restrict destination, require confirmation, require approval, prohibit retention, and attach response checks.
6. Implement deterministic transformation ordering and verify the transformed payload against the permit before forwarding it.
7. Build an OpenAI-compatible inference gateway supporting request/response streaming, cancellation, retries, model aliases, upstream timeouts, and provider health without placing provider rules in core policy.
8. Build a generic tool gateway and MCP proxy that normalize discovery and invocation, validate schemas, enforce permits, constrain arguments, and record downstream outcomes.
9. Implement canonical APIs for retrieval and memory operations so callers can enforce access even when they do not use a gateway.
10. Store append-only decision and enforcement receipts in SQLite with hash chaining, idempotent writes, integrity verification, and payload-digest defaults.
11. Emit OpenTelemetry spans, metrics, and structured events with decision ID, policy revision, operation kind, latency, decision, enforcement result, and reason codes.
12. Create fake inference, tool, retrieval, and memory services for end-to-end tests covering allow, deny, transform, approval, cancellation, streaming, retries, partial failure, and fail-closed behavior.

### Deliverables

- Runnable `normgate serve` service
- Inference, tool, MCP, retrieval, and memory enforcement paths
- Signed one-operation permits
- Generic transformation runtime
- Tamper-evident local receipts and telemetry

### Exit criteria

- Protected downstream operations cannot execute without a valid permit.
- Any post-decision mutation invalidates authorization.
- Streaming output can be stopped before a denied segment reaches the caller.
- Service restarts preserve audit integrity, idempotency, and active policy state.
- Gateway overhead excluding external classification and upstream inference is below 15 ms p95.

## Phase 4 - Complete control plane, state, simulation, and operator experience

Detailed execution plan: [PHASE_4_CONTROL_PLANE.md](phases/PHASE_4_CONTROL_PLANE.md)

### Objective

Complete the domain-agnostic product end to end: persistence, tenancy, lineage, approvals, policy operations, evidence, SDKs, and an operator console.

### Implementation

1. Add PostgreSQL repositories and migrations for tenants, applications, principals, destinations, policy revisions, external facts, sessions, decisions, permits, approvals, receipts, lineage edges, retention jobs, and administrative events.
2. Keep SQLite behaviorally compatible through a shared repository conformance suite.
3. Add Redis-backed permit replay prevention, rate limits, distributed locks, short-lived decision caching, and approval notifications; provide in-process equivalents for local mode.
4. Implement tenant administration, RBAC, service credentials, credential rotation, destination registration, policy assignment, and separation of policy-author and policy-approver roles.
5. Implement data lineage by assigning stable references at ingress and propagating classifications, source, subject references, transformations, destinations, and derived-data relationships through each operation.
6. Implement memory governance for read, write, update, and delete. Track source, verification status, allowed purposes, tenant, retention deadline, and downstream derivatives.
7. Implement deletion propagation across configured primary records, derived records, memories, embeddings, caches, queued exports, and audit references permitted to be removed. Produce a completion receipt per store.
8. Implement stateful policies for cumulative disclosure, cross-purpose reuse, session budgets, record totals, tool frequency, retry count, recursion, and delegation depth.
9. Implement approval workflows with queues, reviewer groups, comments, expiry, delegation, rejection, break-glass access, and mandatory reason capture. Reevaluate every approved operation immediately before execution.
10. Implement `normgate simulate` and `normgate replay` against fixtures or minimized historical receipts. Report decision changes, new obligations, missing context, and affected applications.
11. Add shadow, canary, scheduled activation, emergency revoke, and percentage-based rollout modes. Keep observed, simulated, and enforced decisions explicitly distinct.
12. Build evidence export in JSON and HTML containing control identifiers, policy hashes, tests, decision summaries, exceptions, approvals, enforcement outcomes, and audit-chain verification.
13. Generate Python and TypeScript clients from OpenAPI and add ergonomic middleware for inference, tool, retrieval, response, and memory operations.
14. Build the web console with tenant selection, policy revisions and diffs, live decisions, receipt inspection, approvals, simulation comparison, lineage view, audit verification, and operational health.
15. Add backup/restore, retention scheduling, database migration rollback, configuration diagnostics, readiness checks, and operator runbooks.

### Deliverables

- Multi-tenant PostgreSQL control plane with Redis coordination
- Complete data lineage and memory governance
- Approval, simulation, replay, rollout, and evidence systems
- Python and TypeScript SDKs
- Operational web console and runbooks

### Exit criteria

- A fresh installation can register an application, load policy, enforce operations, request approval, inspect lineage, simulate an update, and export evidence.
- Cross-tenant access fails throughout service, storage, cache, console, and telemetry tests.
- Deletion propagation reports every configured store and fails visibly on incomplete removal.
- Candidate policy can be tested against recorded traffic without invoking an external model.
- Backup/restore recreates policy, state, and verifiable receipt history.

## Phase 5 - Extension platform, domain packages, and core release candidate

Detailed execution plan: [PHASE_5_EXTENSIONS_AND_DOMAIN_PACKS.md](phases/PHASE_5_EXTENSIONS_AND_DOMAIN_PACKS.md)

### Objective

Freeze the complete core API, introduce extensions only after the core works end to end, and prove agnosticism with independently installable domain packages.

### Implementation

1. Run an end-to-end core completeness audit covering every product surface, public contract, command, API, enforcement path, state transition, failure path, and operational workflow. Close all core gaps before freezing the v1 release-candidate contracts.
2. Define extension types for classifiers, transformers, context resolvers, external-fact resolvers, enforcement adapters, evidence exporters, policy packs, and console panels.
3. Define a signed extension manifest with identity, type, permissions, configuration schema, compatibility range, network/filesystem requirements, exported capabilities, dependencies, digest, and publisher identity.
4. Support three execution forms: compiled Go extensions for trusted distributions, isolated local processes over authenticated localhost RPC, and WASI components for portable restricted execution.
5. Enforce extension timeouts, memory and concurrency limits, explicit capabilities, health checks, circuit breakers, version pinning, and fail-closed behavior for authoritative extensions.
6. Add `normgate extension init`, `test`, `build`, `sign`, `verify`, `install`, `list`, `disable`, `upgrade`, and `remove` commands.
7. Implement OCI publication and installation with signature verification, dependency locking, vulnerability metadata, revocation, deprecation, and rollback.
8. Publish extension SDKs and conformance harnesses. Every extension type must have contract fixtures, failure injection, compatibility tests, and a minimal reference implementation.
9. Build the first complete domain package for US healthcare. Keep all HIPAA vocabulary, PHI classification, minimum-necessary rules, provider-contract metadata, FHIR mapping, synthetic scenarios, and control evidence inside that package and its extensions.
10. Build smaller proof packages for finance and education to verify that the same frozen core contracts support different classifications, purposes, roles, actions, evidence mappings, and enforcement obligations.
11. Add optional classifier extensions using Microsoft Presidio and custom detector endpoints. Classifiers emit labels and confidence; they never return authorization decisions.
12. Create one end-to-end sample application per domain package using the same binary, APIs, SDKs, permit format, receipt format, simulation tooling, and console.
13. Add CI that removes every domain package and proves the core still builds and passes its full test suite, then installs each package independently and runs its conformance suite.
14. Publish the NormGate core v1 release candidate together with extension compatibility guarantees and a migration policy.

### Deliverables

- Feature-complete NormGate core release candidate
- Stable extension contracts and isolated extension runtime
- Signed OCI extension distribution
- Extension development and conformance SDKs
- Independently installable healthcare, finance, and education proofs

### Exit criteria

- No domain package imports, patches, forks, or recompiles the core.
- Removing all extensions leaves a complete usable generic policy runtime.
- A third party can create, test, sign, install, execute, upgrade, and remove an extension without editing core source.
- Each reference application uses the same frozen core release candidate.
- Extension failure, compromise simulation, or incompatibility cannot silently bypass enforcement.

## Phase 6 - Production hardening, independent validation, and v1 release

Detailed execution plan: [PHASE_6_PRODUCTION_RELEASE.md](phases/PHASE_6_PRODUCTION_RELEASE.md)

### Objective

Validate security and reliability, ship reproducible deployment artifacts, and establish sustainable project operations for the complete platform.

### Implementation

1. Finalize the threat model for policy bypass, malicious extensions, confused-deputy access, cross-tenant leakage, permit replay, time-of-check/time-of-use changes, audit tampering, classifier failure, supply-chain compromise, and denial of service.
2. Add property tests and continuous fuzzing for schema parsing, normalization, obligation merging, permit verification, policy conflicts, streaming, protocol translation, extension RPC, and artifact installation.
3. Commission independent review of cryptography usage, permit verification, policy activation, tenant isolation, audit chaining, extension isolation, secrets handling, and administrative authorization. Publish findings and remediation status.
4. Perform load, soak, and chaos tests covering policy reloads, database failover, Redis loss, telemetry outage, extension timeout, upstream timeout, concurrent approval, rolling deployment, and last-known-good recovery.
5. Establish and verify release SLOs for decision availability, enforcement latency, receipt durability, recovery objectives, and zero cross-tenant access in the conformance suite.
6. Complete mTLS, encryption-key rotation, signing-key rotation, secret-manager integrations, encrypted backups, point-in-time recovery, administrative session controls, and audited break-glass procedures.
7. Produce reproducible multi-architecture binaries, minimal non-root images, SBOMs, provenance attestations, signed checksums, vulnerability scans, Docker Compose assets, Kubernetes manifests, and a hardened Helm chart.
8. Add zero-downtime schema and policy migrations, compatibility testing across supported versions, rollback rehearsals, disaster recovery exercises, and upgrade documentation.
9. Publish operator, integrator, policy-author, extension-author, security, privacy, retention, migration, incident-response, and assurance-boundary documentation.
10. Establish Apache-2.0 licensing, contribution requirements, code of conduct, security disclosure process, maintainer roles, extension trust levels, release cadence, and artifact-signing procedures.
11. Run controlled pilots against at least two independently implemented applications. Convert every product failure into a permanent core or extension conformance test.
12. Tag v1.0 only after all release gates pass from a clean checkout in an isolated build environment.

### Deliverables

- NormGate v1.0 binaries, images, SDKs, console, and signed artifacts
- Public threat model and independent security report
- Production deployment and recovery assets
- Published conformance, compatibility, and performance reports
- Maintained documentation and community governance

### Exit criteria

- No unresolved critical or high security finding remains at release.
- Clean installations complete the full configure-to-enforce-to-evidence workflow.
- Key rotation, backup/restore, database failover, policy rollback, extension rollback, and disaster recovery tests pass.
- Published performance and failure-mode tests reproduce in CI.
- All release artifacts are reproducible, signed, and independently verifiable.
