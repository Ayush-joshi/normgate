---
name: normgate-docs
description: >-
  Guide for authoring, maintaining, and validating documentation in NormGate.
  Covers Architecture Decision Records (ADRs), Phase Validation Records,
  schema-documentation synchronization, trust boundary specifications,
  and evidence-backed technical writing for AI agents.
---

# NormGate Technical Documentation & Architecture Records

## Overview

In NormGate, documentation is treated with the same engineering rigor as code. Documentation is audited in CI, cross-referenced against requirement IDs (`NG-F00X`), and must never make unverifiable claims.

This skill guides any AI agent or engineer in authoring:
1. **Architecture Decision Records (ADRs)** in `docs/architecture/decisions/`
2. **Phase Validation Records** in `docs/validation/`
3. **Core Architecture & Policy Specs** in `docs/architecture/` and `docs/policy.md`
4. **API and Contract Documentation Sync** across OpenAPI and SDK models

---

## When to Use

- When making significant architectural decisions or adding runtime protocols.
- When completing a phase milestone or publishing benchmark results.
- When adding or modifying schemas in `schemas/v1/` and needing to update documentation.
- When documenting trust boundaries, credential handling, or security controls.

---

## 1. Authoring Architecture Decision Records (ADRs)

ADRs record key architectural and cryptographic decisions. They live in `docs/architecture/decisions/`.

### File Naming Convention
Use the next sequential 4-digit number:
`docs/architecture/decisions/NNNN-<short-description>.md`
(e.g., `0006-response-filtering-buffer.md`)

### ADR Template
Copy and adapt this template:

```markdown
# ADR 000X: Short Descriptive Title

Status: accepted design; implementation target in Phase X.

## Context
Brief description of the design problem, untrusted inputs, and architectural constraints. Explain why alternative approaches were rejected.

## Decision
Concise, prescriptive statement of the architectural choice. Explicitly state:
- What component is trusted.
- What input is untrusted.
- How fail-closed behavior is achieved.
- What data is stored vs. minimized/digested.

## Consequences & Invariants
- What this decision guarantees.
- What trade-offs or latency impacts exist.
- What downstream operations or consumers must assume.

## Evidence & Requirements
Requirement IDs: NG-F00X.
Relevant fixtures: testkit/fixtures/...
Relevant tests: test/...
```

---

## 2. Authoring Phase Validation Records

Whenever a milestone or phase is completed, a formal validation record is created in `docs/validation/phase-<N>.md` (e.g. `docs/validation/phase-2.md`).

### Required Structure

A phase validation record **MUST** contain these 5 sections:

1. **Header & Status**:
   ```markdown
   # Phase N validation record
   Date: YYYY-MM-DD. Status: **Phase N implementation complete; full local verification and built-binary CLI smoke passed**. Phase N+1 is next.
   ```
2. **Delivered Capabilities**:
   - Bulleted list of every interface, package, engine capability, and command added.
3. **Environment and Checks Table**:
   - Tool versions: macOS/Linux arch, Go version, Python version, Node.js version, `govulncheck` version.
   - Comprehensive checklist table:
     | Check | Result |
     | --- | --- |
     | Generated artifacts, formatting, vet, TypeScript, architecture | Pass |
     | Unit, CLI, conformance and contract tests | Pass |
     | Cross-language canonical vectors (Go, Python, TypeScript) | 39/39 pass |
     | Policy statement coverage | XX.XX% (required $\ge 90\%$) |
     | Repository statement coverage | XX.XX% (required $\ge 85\%$) |
     | Fuzz smoke (all targets $\ge 30$ seconds) | Pass |
     | Mutation tests (behavioral mutants killed) | X/X pass |
     | License review | XXX pinned licenses verified |
     | Vulnerability scans (`govulncheck`, `npm audit`) | Zero vulnerabilities |
4. **Performance Benchmark Table**:
   - Run `go test -run '^$' -bench=. -benchmem ./internal/...`
   - Record exact numbers:
     | Benchmark | Mean | p50 | p95 | p99 | Bytes/op | Allocs/op |
5. **Scope and Remaining Phase Boundaries**:
   - Explicitly document what was **NOT** implemented in this phase to prevent false assumptions.

---

## 3. Schema & API Documentation Synchronization

Whenever schemas in `schemas/v1/*.schema.json` are modified:
1. Run `make generate` to update `api/openapi/openapi.gen.json`.
2. Update the corresponding markdown documentation in `README.md` and `docs/policy.md`.
3. If new reason codes or obligations were introduced, ensure `policies/baseline/reasons.json` and `docs/policy.md` list developer descriptions and remediation advice.

---

## DO THIS / NEVER DO THIS (Engineering Guardrails)

| Never Do This (Anti-Patterns) | Do This Instead (NormGate Standard) |
| :--- | :--- |
| ❌ Never make unsubstantiated compliance claims (e.g. "Complies with HIPAA / SOC2 / GDPR"). | ✅ State: *"Core is domain-neutral. No production or compliance assurance is available."* |
| ❌ Never claim an SDK is a client when it is only a transport type. | ✅ State: *"Generated SDK types are transport models, not HTTP clients or runtime validators."* |
| ❌ Never introduce industry-specific terms (e.g. `patient`, `banking`) into core docs. | ✅ Use domain-neutral vocabulary: `principal`, `tenant`, `destination`, `operation`. |
| ❌ Never document mock results as production benchmarks. | ✅ State: *"Values are local measurements from `go test -bench`, not hosted CI or production SLO claims."* |
| ❌ Never leave broken markdown links or outdated phase status. | ✅ Use relative file links (e.g. `[Phase 2 guide](../../plan/phases/PHASE_2_POLICY_ENGINE.md)`). |

---

## Mandatory Pre-Completion Documentation Checklist

Before concluding any documentation task, verify each item:
- [ ] All file paths mentioned exist and are linked correctly.
- [ ] Any requirement references use standard IDs (e.g. `NG-F001`, `NG-F004`).
- [ ] No domain-specific vocabulary (`hipaa`, `patient`, `banking`) was introduced into core docs or schemas.
- [ ] Trust boundaries are explicitly stated (distinguishing trusted vs. untrusted components).
- [ ] Status of features (e.g. implemented vs. deferred to future phase) is accurately described.
