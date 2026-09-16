# Initial threat model

Status: Phases 1–2 foundation and policy controls are implemented. No enforcement
service, permit issuer, audit database or extension host exists yet. The policy
engine requires authenticated canonical inputs from the future enforcement service.

## Assets and actors

Assets include operation authorization, identity and tenancy, policy and fact
revisions, signing keys, credentials, sensitive payloads, receipt integrity and
runtime availability. Actors include application callers, agent/model output,
operators, policy authors, external fact sources, downstream providers, extension
publishers, dependency publishers and attackers controlling any untrusted input.

Entry points are planned HTTP APIs, SDKs, gateway protocols, policy/package sources,
operator configuration, secret files, administrative interfaces and build tooling.
The trust boundaries are listed in `../architecture/trust-boundaries.md`.

## Abuse cases, controls and executable evidence

| Abuse case | Mitigation and status | Evidence | Residual risk |
| --- | --- | --- | --- |
| Duplicate keys or Unicode ambiguity changes signed meaning | Strict parsing rejects duplicate/NFC-colliding keys and invalid Unicode; implemented. | `TestNGF004GoldenVectors`, `FuzzNGF004Canonicalize`, Python/TS shared vectors | Runtime Unicode versions still need the Phase 6 compatibility campaign. |
| Large/deep inputs exhaust parser resources | 1 MiB and depth-64 limits; bounded secret files and CLI reads; implemented. | normalization/config tests and fuzz targets | Request rate limits and HTTP body limits arrive in Phase 3. |
| Unknown fields, versions or operation names bypass policy assumptions | Strict offline schemas and compatibility fixtures; implemented. | `TestNGF002Fixtures`, `TestNGF002SafeDecode` | Semantic tenant/identity consistency requires authenticated runtime context. |
| Remote schema fetch becomes SSRF or external dependency | Embedded bundle and rejecting loader; implemented. | `TestNGF002InvalidSchemaBundleFailsClosed` | Policy sources pin checksums, require HTTPS and reject redirects; authoritative fact clients remain Phase 3. |
| Configuration enables fail-open or injects YAML aliases | Required fail-closed field, strict schemas, alias/tag rejection; implemented. | `TestNGF005Invalid`, `FuzzNGF005Load` | A host administrator can still replace the binary or trusted config. |
| Diagnostics disclose credentials | Explicit credential accessor, redacted standard formatting and JSON, safe public errors; implemented. | `TestNGF005LoadAndRedact`, `TestNGF006ErrorEnvelope`, `TestNGF005CLI` | Reflection/debuggers and intentional APIKey access can expose secrets; memory encryption is not provided. |
| Core acquires a domain dependency | Import and vocabulary checks; implemented. | `TestNGF001ImportBoundary`, `TestNGF007Vocabulary` | Vocabulary detection is a guardrail, not semantic proof of neutrality. |
| Generated clients silently drift | Byte comparison against deterministic generation, including untracked files; implemented. | `make check-generated` | Reviewer must assess intentional schema changes; CI branch protection must be configured when hosted. |
| Policy tampering/conflicting rules | Deterministic composition, content-addressed bundles, source checksum pins, atomic activation and rollback; implemented. | Policy/OPA tests, fuzzing, mutation tests; `docs/validation/phase-2.md` | Pins require trusted configuration; signing identity is metadata and remote signature trust remains Phase 5. |
| Policy accesses host resources or depends on clock/randomness | Explicit deterministic OPA capabilities, offline data and declared facts; implemented. | `TestForbiddenCapabilities`, `TestActiveEvaluationCancellation` | In-process evaluation shares process resources; compile checks occur between bounded synchronous stages. |
| Failed refresh silently leaves policy stale | Maximum-staleness readiness/evaluation gate, durable recovery, bounded retries; implemented. | Manager/source/OCI tests, activation race tests | One writer per state directory; retain and monitor immutable artifacts. |
| Permit replay or changed arguments | Planned one-operation Ed25519 permits, nonce tracking and final-point checks. | Phase 3 pending | Contracts alone provide no authorization. |
| Confused deputy or cross-tenant access | Planned authenticated principals and tenant-scoped stores. | Phases 3–4 pending | Caller-supplied IDs remain untrusted data. |
| Audit tampering or payload over-retention | Planned append-only chains, minimized records and retention controls. | Phases 3–4 pending | Digest-only references do not guarantee replay availability. |
| Malicious extension or dependency | Current checks pin dependency checksums/licenses and scan known vulnerabilities. Extension isolation/signatures are planned. | `make license-check`, `make vuln`; Phase 5 pending | Unknown vulnerabilities and compromised maintainers remain possible. |

## Deferred independent validation

Phase 6 requires independent review, adversarial cross-tenant tests, permit fuzzing,
load/chaos testing, key rotation and recovery rehearsals. This document makes no
compliance or production-readiness claim. New bypasses must become regression tests
before a mitigation is considered implemented.
