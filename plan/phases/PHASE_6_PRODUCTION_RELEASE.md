# Phase 6: Production Hardening, Independent Validation, and v1 Release

## Outcome

At the end of this phase, NormGate v1.0 is independently reviewed, reproducibly built, operationally documented, observable, recoverable, and ready for controlled production adoption. Release evidence proves the tested source, binaries, images, policy artifacts, and compatibility results.

## Entry conditions

- Phase 5 core release candidate and extension conformance suites are green.
- Public v1 contracts are frozen except for release-blocking security corrections.
- All known critical and high defects are triaged as release blockers.

## Mandatory TDD workflow

Production hardening remains test-driven. Every discovered security, load, failure-recovery, migration, or pilot defect receives a failing regression test before the fix. Operational runbooks are validated by executable drills rather than editorial review alone.

Coverage percentages cannot waive security or failure-path tests. Release-critical code requires explicit scenario coverage even when repository thresholds already pass.

## Workstream 1: Final threat model and security verification

### Tests first

Translate every implemented threat-model mitigation into one of:

- Unit or property test
- Integration attack test
- Fuzz target
- Deployment policy test
- Manual test with reproducible evidence

Add attack scenarios for policy bypass, malicious artifacts, confused-deputy access, cross-tenant references, permit replay, time-of-check/time-of-use mutation, audit tampering, credential misuse, unsafe redirects, request smuggling, resource exhaustion, and administrative privilege escalation.

### Implementation

- Finalize data-flow and trust-boundary diagrams.
- Document residual risks and unsupported assurance claims.
- Add hardened defaults for listeners, credentials, TLS, payload limits, timeouts, redirects, and outbound destinations.
- Commission independent review of signing, permit verification, policy activation, tenant isolation, receipts, extension isolation, secrets, and administrative authorization.
- Track every finding in the public security report with severity, fix revision, and verification evidence.

## Workstream 2: Continuous fuzzing and property testing

### Tests first

Promote earlier fuzz targets and add corpus seeds from integration failures for:

- Contract and configuration parsing
- Canonical normalization
- Policy bundle parsing
- Obligation composition
- Permit and receipt verification
- HTTP streaming and protocol translation
- Tool schemas and MCP frames
- Extension manifests and RPC messages
- Archive extraction and installation

### Implementation

- Run bounded fuzz smoke tests on pull requests.
- Run multi-hour campaigns on a scheduled workflow.
- Preserve minimized crashing inputs as permanent regression fixtures.
- Track fuzz target execution time, corpus size, and last successful run.

## Workstream 3: Performance, load, and capacity

### Tests first

Define reproducible workloads for decision-only, transformed inference, streaming, tool invocation, stateful session, approval, receipt ingestion, policy reload, and evidence export.

Measure:

- Throughput
- p50, p95, and p99 latency
- Allocations and memory
- Database and cache saturation
- Queue depth
- Error and denial rates
- Recovery time after injected failure

### Implementation

- Publish hardware and configuration with every benchmark.
- Add bounded worker pools, backpressure, connection pools, pagination, and batch receipt writes where measurements justify them.
- Add capacity-planning guidance and alerts for saturation.
- Enforce performance regression budgets in CI for stable microbenchmarks and in scheduled environments for system benchmarks.

## Workstream 4: Failure and disaster recovery

### Tests first

Automate drills for:

- PostgreSQL primary loss and recovery
- Redis loss
- Policy source unavailability
- Telemetry exporter outage
- Signing or verification key rotation
- Partial receipt-store failure
- Interrupted policy activation
- Rolling version upgrade
- Backup corruption
- Point-in-time restore
- Region or cluster evacuation where supported

### Implementation

- Define recovery point and recovery time objectives.
- Add encrypted backups and verified restore jobs.
- Implement zero-downtime schema migrations with preflight and rollback checks.
- Preserve last-known-good policy locally.
- Document which dependency failures deny operations and which degrade non-authoritative features.
- Run disaster drills from clean infrastructure and retain receipts as release evidence.

## Workstream 5: Key, secret, and credential lifecycle

### Tests first

- Signing-key rotation with overlap
- Immediate compromised-key revocation
- mTLS certificate rotation
- Secret-manager outage
- Expired workload credentials
- Backup-key recovery
- Break-glass access expiry

### Implementation

- Add mTLS between enforcement points and the service.
- Integrate environment/file secrets plus documented external secret-manager interfaces.
- Separate permit-signing, artifact-signing, transport, and backup keys.
- Automate rotation without restart where possible.
- Prevent secrets and private key material from appearing in configuration diagnostics, telemetry, receipts, or support bundles.

## Workstream 6: Build and software supply chain

### Tests first

- Two clean builds from the same revision produce matching artifacts where platform tooling permits reproducibility.
- Binary and image signatures verify offline.
- SBOM corresponds to packaged artifacts.
- Container runs as non-root with read-only root filesystem.
- Startup rejects altered embedded assets.

### Implementation

- Produce multi-architecture binaries and minimal container images.
- Generate SPDX or CycloneDX SBOMs.
- Attach source revision, build parameters, provenance attestations, checksums, and signatures.
- Pin toolchains and CI actions by immutable versions.
- Scan source, dependencies, binaries, images, and infrastructure templates.
- Publish a supported-platform and dependency-update policy.

## Workstream 7: Deployment engineering

### Tests first

- Docker Compose smoke installation
- Helm template and schema validation
- Kubernetes readiness, liveness, disruption, scaling, and network-policy tests
- Upgrade and rollback between supported versions
- Restricted egress and direct-downstream-access denial
- Persistent-volume and backup recovery

### Implementation

- Ship local single-binary mode.
- Ship Docker Compose with explicit development-safe defaults.
- Ship Kubernetes manifests and a hardened Helm chart with pod security, network policies, resource limits, disruption budgets, autoscaling hooks, and external secret support.
- Provide reference topologies for single-node, highly available, and separated control/data plane deployments.

## Workstream 8: Observability and operational support

### Tests first

- Alert rules fire on synthetic failures.
- Dashboards remain usable without sensitive payload fields.
- Trace-to-decision-to-receipt correlation works across services.
- Support bundle redaction tests.
- Clock-skew and missing-telemetry scenarios remain diagnosable.

### Implementation

- Publish dashboards for latency, decisions, denials, failures, policy age, extension health, cache, database, approval backlog, and audit integrity.
- Add alert rules with runbook links.
- Implement redacted diagnostics and support-bundle generation.
- Document metric cardinality limits and retention recommendations.

## Workstream 9: Documentation and usability validation

### Tests first

- Run every quickstart from a clean environment in CI.
- Compile and execute every code sample.
- Check internal links and generated API references.
- Conduct task-based usability tests with new contributors and operators.

### Implementation

Publish:

- Five-minute local quickstart
- Architecture and trust boundaries
- API, CLI, and SDK references
- Policy and extension author guides
- Deployment and operations guide
- Backup, restore, rotation, upgrade, and rollback procedures
- Security and privacy guide
- Incident-response guide
- Assurance-boundary statement
- Troubleshooting decision tree

## Workstream 10: Pilots and release governance

### Tests first

- Convert each pilot acceptance criterion into an automated journey.
- Capture every pilot defect as a failing regression test before correction.
- Re-run the entire release candidate suite after each pilot change.

### Implementation

- Run controlled pilots with at least two independently implemented applications.
- Record integration effort, latency, policy authoring effort, false denials, missed denials, operator workload, and recovery experience.
- Resolve release-blocking usability and correctness failures.
- Establish maintainer roles, contribution requirements, code of conduct, security disclosure process, release cadence, artifact signing, and extension trust levels.
- Generate and sign the release evidence manifest before tagging v1.0.

## Test and coverage gates

- Repository Go statement coverage remains at least 85%.
- Security-critical packages remain at least 95%.
- Maintained Python and TypeScript SDK code remains at least 85%.
- Web console remains at least 80% across statements, branches, functions, and lines.
- All critical workflows have black-box tests independent of unit coverage.
- No critical or high security finding may be waived for v1.0.
- No flaky test may be retried into passing; it must be fixed, quarantined with a release blocker, or removed with justification.
- Required CI matrix passes on every supported operating system and architecture.

## Required validation

```bash
make verify
make test-e2e
make test-security
make test-extension-isolation
make test-load-smoke
make test-chaos
make test-deploy
make test-docs
make coverage
make release-reproducibility
make release-evidence
```

## Suggested pull-request sequence

1. Threat-model test mapping and security hardening
2. Continuous fuzzing and property-test expansion
3. Performance harness and capacity limits
4. Failure recovery, backups, and migrations
5. Key, secret, and credential lifecycle
6. Reproducible builds, SBOMs, and signed artifacts
7. Deployment assets and network enforcement
8. Dashboards, alerts, and support bundles
9. Documentation validation
10. Pilot regressions and v1.0 release evidence

## Completion checklist

- [ ] Independent security review has no unresolved critical or high finding.
- [ ] Load and chaos targets pass at published capacity.
- [ ] Backup, restore, failover, key rotation, upgrade, and rollback drills pass.
- [ ] Images run non-root with restricted filesystem and network policies.
- [ ] Release artifacts are reproducible, signed, and accompanied by verified SBOMs and provenance.
- [ ] Every documentation example executes in CI.
- [ ] Pilot failures are represented by permanent regression tests.
- [ ] A clean isolated build produces the exact v1.0 release evidence manifest.

