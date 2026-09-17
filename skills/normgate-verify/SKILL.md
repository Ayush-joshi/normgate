---
name: normgate-verify
description: >-
  Diagnostic and execution guide for NormGate testing and verification.
  Use when running unit tests, CI checks, or diagnosing failures across the 12-step
  `make verify` gate (coverage thresholds, race conditions, fuzzing, license locks, vulnerabilities).
---

# NormGate Verification & Quality Gate

## Overview

NormGate enforces an unusually strict, sequential 12-step verification gate (`make verify`). No check is silently skipped when tools or network services are unavailable. This skill provides the triage workflow, fast feedback commands, and recovery procedures for each step of the gate.

---

## When to Use

- When running tests during development or before committing changes.
- When `make verify` fails on any step.
- When needing a fast inner-loop check rather than waiting for the full gate.
- When updating dependencies and needing to audit license locks.
- When test coverage drops below the required threshold.

---

## DO THIS / NEVER DO THIS (Engineering Guardrails)

| Never Do This (Anti-Patterns) | Do This Instead (NormGate Standard) |
| :--- | :--- |
| ❌ Never skip `make verify` and claim a task is complete. | ✅ Always run the full `make verify` gate before declaring completion. |
| ❌ Never edit `*.gen.go` or generated types to fix `check-generated`. | ✅ Always edit the source schema in `schemas/v1/` and run `make generate`. |
| ❌ Never silence race conditions by removing `t.Parallel()`. | ✅ Fix the underlying race using mutexes, atomic pointers, or thread-safe copies. |
| ❌ Never lower test coverage thresholds in `scripts/coverage.py`. | ✅ Add unit tests covering missing branches to meet the coverage bar. |
| ❌ Never run `make verify` with `-j` or parallel flags. | ✅ Keep verification sequential (`make verify` runs sequentially by design). |
| ❌ Never delete policy scenario assertions to make tests pass. | ✅ Fix the Rego rule or document the semantic diff if behavior intentionally changed. |

---

## Fast Inner-Loop vs. Full Gate

Do **NOT** run `make verify` on every small file edit—it executes fuzzing campaigns, race detectors, and mutation tests that take substantial time. Use this two-tier approach:

### Tier 1: Fast Inner-Loop (Seconds)

```bash
# 1. Format code
make fmt

# 2. Check generated contract drift
make check-generated

# 3. Fast lint & architecture test
make lint

# 4. Run only the package you are working on
make test-unit PKG=./internal/policy/...
# or a specific test:
go test -v ./internal/policy -run TestBundleValidation
```

### Tier 2: Full Gate (Mandatory Before PR / Task Completion)

```bash
make verify
```

The gate executes these 12 steps sequentially:
1. `check-generated`
2. `lint` (fmt check, go vet, npm typecheck, python compileall, architecture tests)
3. `test-unit`
4. `test-contract`
5. `test-policy`
6. `test-race`
7. `coverage`
8. `test-fuzz-smoke`
9. `test-mutation`
10. `license-check`
11. `vuln`
12. `build`

---

## Step-by-Step Triage for Verification Failures

### 1. `check-generated` Fails
* **Symptom**: `Generated artifacts are stale; run make generate: ...`
* **Root Cause**: You modified JSON schemas under `schemas/v1/` or generated files drifted.
* **Fix**:
  ```bash
  make generate
  make check-generated
  ```
* **Rule**: Never edit `*.gen.go`, `types/__init__.py`, `types/index.ts`, or `openapi.gen.json` directly. Edit the schema in `schemas/v1/` and regenerate.

### 2. `lint` Fails
* **Go format error**: Run `make fmt`.
* **TypeScript typecheck error**: Run `npm run typecheck` to see compiler errors in `sdk/typescript`.
* **Python syntax error**: Run `python3 -m compileall -q scripts sdk/python/src`.
* **Architecture test error (`TestNGF001ImportBoundary` / `TestNGF007Vocabulary`)**:
  - Check if `internal/` code imported `cmd`, `sdk`, `web`, or `extensions`.
  - Check if any OPA SDK import exists outside `internal/policy/opa/`.
  - Check if any schema in `schemas/v1/` contains forbidden domain words (e.g. `patient`, `banking`, `hipaa`, `diagnosis`).

### 3. `test-contract` Fails
* **Symptom**: Parity mismatch between Go, Python, and TypeScript on canonical vectors.
* **Fix**:
  - Run the three checks individually to pinpoint the diverging language:
    ```bash
    go test -v ./test/contract
    python3 scripts/check_vectors.py
    node scripts/check_vectors.mjs
    ```
  - Ensure any new test vectors in `testkit/fixtures/normalization/vectors.json` are valid JSON with canonical SHA-256 hashes.

### 4. `test-policy` Fails
* **Symptom**: Baseline scenario assertions failed.
* **Fix**:
  ```bash
  ./bin/normgate policy test policies/baseline
  ```
  - Inspect the failed scenario YAML in `policies/baseline/tests/`.
  - Check whether `outcome`, `reason_codes`, `rule_ids`, or `obligations` changed.
  - Never blindly update scenario golden assertions without documenting the semantic diff.

### 5. `test-race` Fails
* **Symptom**: Data race reported by Go race detector (`WARNING: DATA RACE`).
* **Fix**:
  - Identify the concurrent reads/writes reported in the stack trace.
  - Ensure mutex protection (`sync.RWMutex`), atomic pointers (`atomic.Pointer`), or thread-safe copies are used.
  - Pay special attention to concurrent policy evaluation during atomic policy swaps (`policy.Manager`).

### 6. `coverage` Fails
* **Symptom**: Overall or per-package statement coverage dropped below project thresholds.
  - Overall threshold: $\ge 90\%$ (Phase 2 achieved 92.59%).
  - Policy packages (`internal/policy`, `internal/policy/opa`): $\ge 90\%$.
  - Enforcement packages (`internal/enforcement`, `internal/receipt`): $\ge 95\%$.
* **How to Diagnose and Bump Coverage**:
  ```bash
  make coverage
  # Inspect uncovered functions:
  go tool cover -func=coverage.out | grep -v "100.0%" | head -n 20
  ```
  - Find lines with 0.0% coverage: typically error checks, nil pointer handling, or default switch cases.
  - Add targeted table test cases that explicitly trigger those branches (e.g., passing invalid JSON, cancelled context, or nil pointers).

### 7. `test-fuzz-smoke` Fails
* **Symptom**: A fuzzer found a crasher or infinite loop within 30 seconds.
* **Fix**:
  - Check the crashing input written to `testdata/fuzz/...`.
  - Add a regression unit test reproducing the exact input.
  - Ensure inputs have size limits (e.g., 1 MiB limit, 64-level nesting limit in canonical JSON).

### 8. `test-mutation` Fails
* **Symptom**: Mutation testing detected that a mutation was NOT caught by tests.
* **Fix**:
  ```bash
  python3 scripts/mutate_policy.py
  ```
  - This script applies code mutations (e.g. flipping boolean conditions, changing conflict resolution order) and asserts that the test suite fails. If the test suite passes under a mutation, your tests have a gap. Add a test asserting that specific edge behavior.

### 9. `license-check` Fails
* **Symptom**: Dependency license mismatch or unapproved dependency license.
* **Fix**:
  ```bash
  python3 scripts/licenses.py
  ```
  - When dependencies change, inspect their licenses. Allowed licenses: `Apache-2.0`, `MIT`, `MIT-0`, `ISC`, `BSD-2-Clause`, `BSD-3-Clause`, `MPL-2.0`.
  - If the dependency is approved, update `scripts/licenses.lock.json` intentionally.

### 10. `vuln` Fails
* **Symptom**: `govulncheck` or `npm audit` flagged known vulnerabilities.
* **Fix**:
  - Review the CVE report.
  - Update the affected module in `go.mod` or `package.json` to the patched version.
  - Update `go.sum` / `package-lock.json` and re-run `make vuln`.

---

## Mandatory Pre-Completion Verification Checklist

Before finishing any task, run through this binary checklist:
- [ ] `make fmt` executed cleanly.
- [ ] `make check-generated` passes without differences.
- [ ] `make lint` passes (go vet, tsc, python, architecture tests).
- [ ] `make test-unit` passes.
- [ ] `make test-contract` passes across Go, Python, and Node.
- [ ] `make test-policy` passes.
- [ ] `make test-race` passes without data races.
- [ ] `make coverage` passes project coverage thresholds.
- [ ] Full `make verify` returns exit code 0.
- [ ] `git status` shows no accidental uncommitted drift in generated files.
