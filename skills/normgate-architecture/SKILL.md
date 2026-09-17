---
name: normgate-architecture
description: >-
  Guide for upholding NormGate architectural invariants, trust boundaries,
  package import isolation (CI architecture tests), OPA encapsulation,
  configuration security, and secret handling.
---

# NormGate Architecture & Trust Boundaries

## Overview

NormGate is designed with rigorous architectural boundaries to ensure that the core runtime remains independent of any industry, regulation, model provider, or agent framework.

Violations of these boundaries are not merely code review comments—they are enforced by automated unit tests in `test/architecture/` that immediately fail CI if violated.

---

## When to Use

- When adding new Go packages or organizing imports across `internal/`.
- When reviewing trust boundaries and deciding where authentication, validation, or authorization occurs.
- When working with configuration loading, environment variables, or secret files.
- When diagnosing `TestNGF001ImportBoundary` or `TestNGF007Vocabulary` test failures.

---

## DO THIS / NEVER DO THIS (Guardrails for AI Models)

| Never Do This (Common Model Mistakes) | Do This Instead (NormGate Standard) |
| :--- | :--- |
| ❌ Never import `cmd/`, `sdk/`, or `web/` inside `internal/`. | ✅ `internal/` packages may only import standard library, contracts, or peer internal packages. |
| ❌ Never import OPA SDK (`open-policy-agent/opa`) outside `internal/policy/opa/`. | ✅ Use the `policy.Engine` interface in `internal/policy/engine.go`. |
| ❌ Never introduce industry-specific words (`patient`, `banking`, `diagnosis`) in schemas. | ✅ Use domain-neutral vocabulary (`principal`, `tenant`, `destination`, `classification`). |
| ❌ Never fail open when a policy, fact, or secret is missing. | ✅ Always fail closed with a structured error or denial. |
| ❌ Never log or format config objects containing raw API keys. | ✅ Redact secrets in formatting; invoke `APIKey()` only at the exact point of use. |

---

## Allowed vs. Forbidden Package Dependencies

```text
[ cmd/normgate ] (CLI entry point)
       │
       ▼
[ internal/api ] [ internal/enforcement ] [ internal/policy ]
       │                      │                      │
       ▼                      ▼                      ▼
[ internal/contracts ] ◄─────────────────────────────┘
       │
       ▼ (Schemas in schemas/v1/)

FORBIDDEN INWARD IMPORTS:
❌ internal/...  ──►  cmd/...
❌ internal/...  ──►  sdk/...
❌ internal/...  ──►  web/...
❌ internal/...  ──►  extensions/...
❌ internal/* (except internal/policy/opa) ──► github.com/open-policy-agent/opa
```

---

## Automated Architectural Invariants (`test/architecture/`)

### 1. Package Import Boundaries (`TestNGF001ImportBoundary`)

The core Go runtime lives in `internal/`. It has strict isolation rules:
- **No Inward Imports**: `internal/` code **MUST NEVER** import from:
  - `cmd` (CLI entry points)
  - `sdk` (Python or TypeScript SDKs)
  - `web` (Management console)
  - `extensions` / `domains` / `domain-packs` (Future plugins)
- **OPA SDK Encapsulation**: The Open Policy Agent SDK (`github.com/open-policy-agent/opa`) is **strictly encapsulated** inside `internal/policy/opa/`. No other package in the entire repository may import OPA types or packages directly. All interactions must go through the engine interface in `internal/policy/engine.go`.

### 2. Core Domain Neutrality (`TestNGF007Vocabulary`)

Core schemas under `schemas/v1/*.json` must remain completely generic:
- **Forbidden Vocabulary**: Words matching `(?i)\b(hipaa|phi|fhir|patient|diagnosis|healthcare|banking|student|education)\b` are strictly blocked.
- **Rationale**: Domain-specific concepts belong in extension packs, not in the core runtime contracts.

### 3. Layout Invariance (`TestNGF001Layout`)

Core architectural directories must exist and remain stable:
`cmd/normgate`, `internal/api`, `internal/config`, `internal/contracts`, `internal/enforcement`, `internal/identity`, `internal/normalization`, `internal/policy`, `internal/receipt`, `internal/storage`, `internal/telemetry`, `api/openapi`, `schemas/v1`, `policies/baseline`, `testkit/fixtures`, `docs/architecture/decisions`.

---

## Trust Boundaries Summary

| Boundary | Untrusted Input | Trusted Component | Enforced Rule |
| :--- | :--- | :--- | :--- |
| **Caller $\to$ Gateway** | Headers, asserted identities, parameters | Authenticated Gateway Middleware | Caller claims never authenticate a principal. Authenticate credentials and derive identity. |
| **Model/Classifier $\to$ Policy** | Proposed content labels, classifications | Deterministic Policy Engine | Model classifications are input facts; they never grant authority. |
| **Policy Source $\to$ Activation** | Bundles, tar archives, manifests | Atomic Policy Manager | Verify SHA-256 digests, compile, and run scenario tests before atomic swap. |
| **Fact Source $\to$ Policy** | Dynamic external data, risk scores | Configured Authoritative Resolver | Facts must be signed/validated with validity windows (`resolved_at <= occurred_at < expires_at`). Missing required facts fail closed. |
| **Decision $\to$ Downstream** | Arguments, destinations | Final-point Enforcement Adapter | Verify signed Ed25519 permit and payload digest immediately before downstream call. |
| **Service $\to$ Evidence** | Decisions, outcomes | Minimized Receipt Writer | Digests and references only; no raw payloads or secrets in receipts or telemetry. |

---

## Configuration & Secret Handling

NormGate configuration (`internal/config`) enforces strict parsing:
- **Strict Decoders**: Reject unknown fields, duplicate keys, YAML aliases, multiple documents, custom tags, unsafe timeouts, and fail-open settings.
- **One-Pass Expansion**: `${ENV:NAME}` and `${FILE:/absolute/path}` references must occupy the entire scalar and expand exactly once. They are never recursively evaluated.
- **Secret Bounds**: Secret files must be regular files, $\le 64\text{ KiB}$.
- **Redacted Diagnostics**: Printing or dumping configuration models automatically redacts API keys and secrets. Only the explicit `APIKey()` method provides access to raw credentials at execution time.

---

## Mandatory Pre-Completion Architecture Checklist

- [ ] Run `go test ./test/architecture` and ensure both `TestNGF001ImportBoundary` and `TestNGF007Vocabulary` pass.
- [ ] No `internal/` file imports `cmd/`, `sdk/`, `web/`, or `extensions/`.
- [ ] No file outside `internal/policy/opa/` imports OPA.
- [ ] All error handlers fail closed (deny/block) when dependencies are unavailable.
- [ ] Secret values are never printed in logs, test output, or string representations.
