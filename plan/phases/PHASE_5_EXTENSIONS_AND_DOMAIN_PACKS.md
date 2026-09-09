# Phase 5: Extension Platform, Domain Packages, and Core Release Candidate

## Outcome

At the end of this phase, the complete core API is frozen as a release candidate. Third parties can create, test, sign, distribute, install, execute, upgrade, disable, and remove extensions without modifying core source. Independent healthcare, finance, and education packages prove that the frozen core is genuinely domain-agnostic.

## Entry conditions

- Phase 4 complete end-to-end workflow passes from a clean installation.
- Core public schemas, APIs, receipts, permits, and SDK behavior are ready for compatibility review.
- No domain package or vocabulary exists in the core dependency graph.

## Mandatory TDD workflow

The extension contract is built from conformance tests before any production extension runner is written. Every extension type starts with a fake implementation, timeout case, malformed-result case, crash case, permission-denial case, and compatibility case.

Each domain policy begins as executable scenarios reviewed by a domain expert. A policy is not accepted because its Rego compiles; it must pass positive, negative, boundary, adversarial, and missing-evidence cases. Legal or regulatory mapping text must never substitute for executable tests.

## Workstream 1: Core completeness and API freeze

### Tests first

- Create `test/release/core_workflow_test.go` covering installation, identity registration, destination registration, policy activation, allow, deny, transform, approval, downstream execution, receipt verification, replay, simulation, and evidence export.
- Create compatibility snapshots for all public schemas, HTTP routes, SDK methods, CLI commands, reason codes, and OpenTelemetry attributes.
- Add a test that builds and runs the core after deleting the entire extension and domain-package trees.

### Implementation

- Audit every product surface against the main roadmap.
- Close missing failure paths and remove undocumented public behavior.
- Mark public v1 release-candidate packages and schemas.
- Document compatibility guarantees, deprecation periods, and migration rules.
- Require an architecture decision and compatibility test for every post-freeze breaking change.

## Workstream 2: Extension contracts

### Tests first

Define language-neutral conformance fixtures for:

- Classifier
- Transformer
- Context resolver
- External-fact resolver
- Enforcement adapter
- Evidence exporter
- Policy pack
- Console panel

For each type, test capability negotiation, initialization, health, execution, cancellation, deadline, structured errors, shutdown, and unsupported contract versions.

### Implementation

Define `ExtensionManifest` with:

```yaml
identity: registry.example/namespace/name
version: 1.0.0
type: classifier
core_compatibility: ">=1.0.0-rc.1 <2.0.0"
contract_version: v1
permissions:
  network: []
  filesystem: []
  secrets: []
resources:
  memory_mb: 128
  timeout_ms: 100
capabilities: []
configuration_schema: config.schema.json
artifacts: []
publisher: example
```

Manifest permissions are maximum capabilities, not automatic grants. Installation and runtime configuration must grant the effective subset explicitly.

## Workstream 3: Isolated execution

### Tests first

- Extension crash, hang, malformed output, excess output, memory exhaustion, and repeated failure
- Attempted undeclared network and filesystem access
- Core cancellation propagation
- Circuit-breaker open, half-open, and recovery
- Upgrade while requests are active
- Authoritative extension unavailable
- Non-authoritative extension unavailable

### Implementation

Support:

1. Compiled Go implementations for explicitly trusted distributions.
2. Isolated local processes using authenticated localhost RPC and per-launch credentials.
3. WASI components for portable restricted execution.

Add bounded message sizes, deadlines, concurrency limits, memory limits, health checks, crash supervision, circuit breakers, and structured failure policy. An unavailable authoritative extension must cause denial; an optional evidence exporter may degrade without changing authorization.

## Workstream 4: Extension lifecycle and supply chain

### Tests first

- Deterministic package build
- Signature verification and unknown signer
- Digest mismatch
- Dependency resolution and cycle detection
- Core incompatibility
- Revoked and deprecated versions
- Failed upgrade rollback
- Offline installation from a pinned artifact
- Removal blocked while an active policy depends on the extension

### Implementation

Add:

```bash
normgate extension init
normgate extension test
normgate extension build
normgate extension sign
normgate extension verify
normgate extension install
normgate extension list
normgate extension disable
normgate extension upgrade
normgate extension remove
```

Use OCI artifacts with immutable digests. Store lockfiles, verified signer identity, effective permissions, install time, and dependency graph. Support emergency revocation and atomic rollback.

## Workstream 5: Extension SDK and conformance kit

### Tests first

- A generated example for every extension type compiles and passes conformance.
- SDK versions reject incompatible manifests.
- Cross-language fixtures return byte-equivalent results where the contract requires determinism.
- Failure injection behaves identically across execution forms.

### Implementation

- Publish Go, Python, and TypeScript authoring libraries where appropriate.
- Provide an extension test host, fake NormGate service, fixture generator, contract assertions, latency measurement, and failure injection.
- Generate a conformance report suitable for attaching to a registry submission.
- Provide starter repositories and CI workflows without embedding domain rules in templates.

## Workstream 6: Healthcare package

### Tests first

Build synthetic scenarios for:

- Same-subject and cross-subject data access
- Minimum-necessary field selection
- Approved and unapproved destinations
- Missing or expired provider-contract metadata
- Purpose changes
- Prompt injection through retrieved content
- Sensitive data in model input, output, logs, and memory
- Record deletion and derived embedding deletion
- Administrative assistance versus clinical recommendation boundaries
- Missing evidence and mandatory human review

### Implementation

Create independently installable artifacts for:

- `healthcare-us` policy pack
- PHI/PII classifier configuration
- FHIR resource mapper
- Provider-contract external-fact resolver
- Healthcare evidence mapping exporter
- Synthetic conformance corpus
- End-to-end sample application

Keep HIPAA mappings, PHI vocabulary, minimum-necessary interpretations, healthcare roles, and FHIR dependencies entirely inside these artifacts. Reports must state that they provide control evidence and not compliance certification.

## Workstream 7: Finance package

### Tests first

Cover card-data handling, account isolation, transaction amount limits, destination restrictions, duplicate execution, approval separation, retention, advice boundaries, and insufficient evidence.

### Implementation

Create a compact `finance-payments` package containing classifications, purposes, roles, transaction policies, approval obligations, evidence mappings, synthetic fixtures, and a sample application. Use only frozen core contracts.

## Workstream 8: Education package

### Tests first

Cover student-record isolation, learner/teacher/guardian access, classroom or institution boundaries, assessment integrity, age-sensitive approval, retention, memory leakage, and insufficient context.

### Implementation

Create a compact `education-us` package containing classifications, purposes, roles, disclosure policies, approval obligations, evidence mappings, synthetic fixtures, and a sample application. Use only frozen core contracts.

## Workstream 9: Cross-domain agnosticism verification

### Tests first

- Install each package alone and run its complete sample.
- Install packages in different orders and verify identical decisions.
- Compose multiple packages and detect namespace or obligation conflicts deterministically.
- Remove all packages and rerun the complete generic core suite.
- Search core source and schemas for forbidden domain imports and vocabulary.

### Implementation

- Add an automated dependency graph and vocabulary allowlist report.
- Publish a compatibility matrix for each package and extension.
- Require package-owned reason-code namespaces and migration notes.
- Make core changes caused by a package request pass a generality review using at least two independent use cases.

## Test and coverage gates

- Extension loader, verifier, permission, and lifecycle packages: at least 95% Go statement coverage
- Extension runner and RPC code: at least 90%
- Every extension type passes the shared conformance and failure-injection suites.
- Every domain rule has positive, negative, boundary, adversarial, and missing-evidence cases.
- Domain packages report coverage by policy rule, not only source statements.
- All sample applications run as black-box end-to-end tests in CI.
- Scheduled CI installs artifacts from a temporary OCI registry and verifies signatures and rollback.

## Required validation

```bash
make test-core-without-extensions
make test-extension-conformance
make test-extension-isolation
make test-domain-packages
make test-e2e
make test-race
make test-fuzz-smoke
make coverage
make compatibility-report
make verify
```

## Suggested pull-request sequence

1. Core completeness audit and compatibility snapshots
2. Extension manifests and conformance fixtures
3. Process and WASI runners
4. Lifecycle CLI, OCI packaging, signing, and rollback
5. Extension SDKs and test host
6. Healthcare package and sample
7. Finance and education proof packages
8. Cross-domain dependency and compatibility gate
9. Core v1 release candidate

## Completion checklist

- [ ] Generic core passes after all domain and extension files are removed.
- [ ] Core release-candidate API and compatibility policy are published.
- [ ] Each extension type passes crash, timeout, permission, and malformed-output tests.
- [ ] Install, upgrade, revoke, roll back, and remove operations are atomic and audited.
- [ ] All domain behavior remains outside the core dependency graph.
- [ ] Each domain package passes its policy-rule and end-to-end coverage gates.
- [ ] Third-party starter extension passes conformance without core changes.

