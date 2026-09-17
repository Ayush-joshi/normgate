---
name: normgate-policy
description: >-
  Guide for authoring, evaluating, and testing deterministic policies using embedded OPA
  and Rego v1 in NormGate. Covers layer composition, strict determinism constraints,
  bundle lifecycle management, YAML scenario testing, policy CLI commands, and mutation testing.
---

# NormGate Policy Development & Lifecycle

## Overview

NormGate's policy engine is an embedded, deterministic Open Policy Agent (OPA) runtime. It compiles, tests, explains, distributes, and atomically activates content-addressed policy bundles.

The policy runtime never relies on language models to grant authority. It maps canonical operations to one of four deterministic outcomes:
- `allow`
- `deny`
- `transform`
- `require_approval`

---

## When to Use

- When writing or editing Rego modules (`*.rego`) under `policies/`.
- When adding or modifying baseline policy rules and reason codes.
- When creating YAML scenario test cases in `policies/<bundle>/tests/`.
- When using the `normgate policy` CLI for building, inspecting, explaining, diffing, or activating bundles.
- When verifying policy conflict resolution or running policy mutation tests (`scripts/mutate_policy.py`).

---

## DO THIS / NEVER DO THIS (Guardrails for AI Models)

| Never Do This (Common Model Mistakes) | Do This Instead (NormGate Standard) |
| :--- | :--- |
| ❌ Never use `time.now_ns()` or `time.Now()` in Rego. | ✅ Use the event's canonical timestamp: `input.occurred_at`. |
| ❌ Never call external APIs or use `http.send` in policies. | ✅ Pass pre-resolved, verified data via `input.external_facts`. |
| ❌ Never import OPA SDK (`open-policy-agent/opa`) outside `internal/policy/opa/`. | ✅ Use the `policy.Engine` interface in `internal/policy/engine.go`. |
| ❌ Never omit `default decisions := []` in Rego modules. | ✅ Every entry point module must define `default decisions := []`. |
| ❌ Never invent uncataloged reason codes. | ✅ Register reason codes in `reasons.json` with description and remediation. |
| ❌ Never delete or alter a test scenario to hide an unexpected denial. | ✅ Fix the rule logic or document the semantic diff. |

---

## Strict Determinism Rules in Rego

NormGate policies execute under hard deterministic bounds:

1. **No System Clock**: Never call `time.now_ns()` or read wall-clock time. Evaluation timestamp (`evaluated_at`) is bound to the canonical event's timestamp.
2. **No Network or Filesystem Access**: OPA built-ins that interact with network, files, or environment are blocked at compile time.
3. **Approved Built-ins Only**: Only approved, side-effect-free built-ins are enabled.
4. **External Data via `ExternalFact`**: Dynamic external data (e.g. user risk scores, clearance levels, device health) must enter as pre-resolved, authenticated `ExternalFact` records with expiration timestamps (`resolved_at <= occurred_at < expires_at`).
5. **Strict Compilation Limits**: Max 4 MiB bundle size, 1 MiB per file, 64 modules, 2,048 top-level rules, lexical nesting depth 64, and 50 ms evaluation timeout.

---

## Concrete Rego v1 Module Template

Every Rego module in NormGate must follow this exact pattern:

```rego
package normgate.baseline.rules

import rego.v1

# MANDATORY: Entry points must define default empty decisions array
default decisions := []

# Rule 1: Monotonic Deny example
decisions contains decision if {
    input.operation.destination.id == "restricted-sink"
    decision := {
        "rule_id": "deny_restricted_sink",
        "outcome": "deny",
        "reason_codes": ["ng.policy.destination_restricted"],
        "obligations": []
    }
}

# Rule 2: Transformation example
decisions contains decision if {
    input.operation.kind == "tool_call"
    input.operation.name == "export_data"
    decision := {
        "rule_id": "transform_redact_pii",
        "outcome": "transform",
        "reason_codes": ["ng.policy.redact_pii"],
        "obligations": [
            {
                "schema_version": "1.0",
                "id": "redact_email",
                "kind": "remove_paths",
                "paths": ["/arguments/email", "/arguments/phone"]
            }
        ]
    }
}
```

### Composition & Conflict Resolution Rules

When multiple layers or rules produce partial decisions:
1. **Any applicable `deny` produces a final `deny`** (monotonic deny).
2. **Any applicable `require_approval` prevents a direct `allow`**.
3. **Compatible transformations merge in deterministic obligation-ID order**.
4. **Conflicting obligations produce a `deny`**.
5. **If no applicable terminal decision is produced, the engine defaults to `deny` (fail-closed)**.

---

## Concrete YAML Scenario Template (`tests/*.yaml`)

Every policy rule must have exact scenario assertions in `policies/<bundle>/tests/`:

```yaml
id: deny-unregistered-destination
description: Ensure operations targeting unregistered destinations are blocked.
event:
  schema_version: "1.0"
  event_id: "evt_test_001"
  occurred_at: "2026-09-17T00:00:00Z"
  principal:
    id: "user_alice"
    tenant_id: "tenant_main"
  operation:
    kind: "tool_call"
    name: "calculator"
    destination:
      id: "unknown_destination"
      type: "tool"
  purpose: "testing"
  context:
    application_id: "app_demo"
    environment: "test"
expected:
  outcome: "deny"
  reason_codes:
    - "ng.policy.unknown_destination"
  rule_ids:
    - "deny_unregistered_destination"
  obligations: []
```

Run scenarios with:
```bash
./bin/normgate policy test policies/baseline
```

---

## Policy CLI Reference

Compile the binary first:
```bash
make build
```

| Command | Purpose |
| :--- | :--- |
| `./bin/normgate policy init ./my-policy` | Scaffold a new policy bundle directory with manifest, layers, rego, reasons, and tests. |
| `./bin/normgate policy lint ./my-policy` | Validate syntax, manifest schema, deterministic built-ins, and layer namespaces. |
| `./bin/normgate policy test ./my-policy` | Run all YAML scenarios in the bundle's `tests/` directory. |
| `./bin/normgate policy build ./my-policy --out bundle.tar.gz` | Compile and build a deterministic, content-addressed gzip archive. |
| `./bin/normgate policy inspect bundle.tar.gz` | Inspect manifest, layers, modules, rule counts, and source/build digests. |
| `./bin/normgate policy explain --bundle bundle.tar.gz --event event.json` | Evaluate an event and output detailed rule match provenance and obligation rationale. |
| `./bin/normgate policy diff old.tar.gz new.tar.gz` | Show semantic diffs between two bundle revisions. |
| `./bin/normgate policy activate --bundle bundle.tar.gz --state ./policy-state` | Atomically activate a bundle into a state directory (runs tests first). |
| `./bin/normgate policy status --state ./policy-state` | Check active revision, previous revision, last refresh time, and health. |
| `./bin/normgate policy rollback --revision sha256:... --state ./policy-state` | Safely roll back to an exact prior verified revision. |

---

## Policy Mutation Testing

To prove that your tests actually verify boundary logic and conflict resolution:
```bash
make test-mutation
# or directly:
python3 scripts/mutate_policy.py
```
This script introduces temporary mutations (flipping allow/deny, reversing obligation priority) into the policy engine code and asserts that unit and scenario tests catch the bug. If a mutation survives undetected, add scenario tests for that edge condition.

---

## Mandatory Pre-Completion Policy Checklist

- [ ] All Rego entry points define `default decisions := []`.
- [ ] No Rego code uses `time.now_ns`, `time.Now()`, `http.send`, or non-deterministic built-ins.
- [ ] Every new rule has at least one allow scenario, one deny scenario, and boundary scenario in `tests/*.yaml`.
- [ ] All reason codes are declared in `reasons.json`.
- [ ] `./bin/normgate policy test <bundle>` passes 100%.
- [ ] `make test-mutation` kills all behavioral mutants.
- [ ] OPA imports remain strictly inside `internal/policy/opa/`.
