---
name: normgate-tdd
description: >-
  Guide for executing NormGate's mandatory Test-Driven Development (TDD) workflow.
  Covers writing failing table-driven Go tests, constructing negative/bypass test matrices,
  using test doubles from `testkit/`, and implementing fuzzing targets.
---

# NormGate Test-Driven Development (TDD)

## Overview

In NormGate, **TDD is mandatory**:
- Every new rule or policy behavior begins as a failing YAML scenario in `policies/<bundle>/tests/`.
- Every engine, gateway, or runtime behavior begins as a failing Go table-driven test.
- Every fixed bug receives both a narrow regression test and an integration-level test.
- Updating golden files or test assertions alone without documenting semantic rationale is strictly forbidden.

---

## When to Use

- Before implementing any new feature, obligation, policy rule, or gateway endpoint.
- When fixing an issue or addressing security findings.
- When designing negative test suites (tamper detection, replay prevention, bypass attempts).
- When writing fuzzing smoke targets (`testing.F`) or benchmarks (`testing.B`).

---

## DO THIS / NEVER DO THIS (Guardrails for AI Models)

| Never Do This (Common Model Mistakes) | Do This Instead (NormGate Standard) |
| :--- | :--- |
| ❌ Never write implementation code before writing a failing test. | ✅ Write the test first, run it, assert it fails, then implement. |
| ❌ Never test only the "happy path" on security-critical logic. | ✅ Test negative mutations: expired permits, altered nonces, bad signatures. |
| ❌ Never update golden files or test expectations to hide a regression. | ✅ Fix the code defect or document the semantic diff in the PR/commit. |
| ❌ Never use real network calls or wall clocks in unit tests. | ✅ Use in-memory test doubles, deterministic fixtures, and synthetic events. |
| ❌ Never write monolithic, unorganized test functions. | ✅ Use table-driven tests with subtests (`t.Run`) and descriptive test case names. |

---

## The NormGate TDD Cycle

```text
1. Specify & Fail
   └─► Write a test asserting the expected behavior.
   └─► Run it: assert that it FAILS for the expected reason (not a syntax error).

2. Implement Minimal Solution
   └─► Write just enough code to make the test pass.
   └─► Maintain architectural boundaries and determinism.

3. Refactor & Protect
   └─► Format code: make fmt
   └─► Verify race safety: go test -race ./internal/...
   └─► Verify coverage: make coverage

4. Full Regression Check
   └─► Run the 12-step gate: make verify
```

---

## Table-Driven Test Idiom in Go

Use Go table-driven tests with subtests (`t.Run`) for all internal packages:

```go
func TestObligationExecution(t *testing.T) {
	tests := []struct {
		name        string
		obligation  contracts.Obligation
		payload     map[string]any
		wantPayload map[string]any
		wantErr     bool
	}{
		{
			name: "remove_paths removes targeted pointer",
			obligation: contracts.Obligation{
				Kind:  "remove_paths",
				Paths: []string{"/user/ssn"},
			},
			payload:     map[string]any{"user": map[string]any{"name": "Alice", "ssn": "000-00-0000"}},
			wantPayload: map[string]any{"user": map[string]any{"name": "Alice"}},
			wantErr:     false,
		},
		{
			name: "missing required target fails closed",
			obligation: contracts.Obligation{
				Kind:  "mask_values",
				Paths: []string{"/missing/field"},
			},
			payload: map[string]any{"other": "value"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExecuteObligation(tt.obligation, tt.payload)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ExecuteObligation() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.wantPayload) {
				t.Errorf("got %v, want %v", got, tt.wantPayload)
			}
		})
	}
}
```

---

## Designing Fake Downstream Test Doubles

When testing gateways or enforcement adapters, never call real network endpoints. Implement an in-memory double that records received payloads:

```go
type FakeDownstream struct {
	mu           sync.Mutex
	CallCount    int
	LastPayload  map[string]any
	ReturnError  error
	ReturnResult map[string]any
}

func (f *FakeDownstream) Execute(ctx context.Context, payload map[string]any) (map[string]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.CallCount++
	f.LastPayload = payload
	if f.ReturnError != nil {
		return nil, f.ReturnError
	}
	return f.ReturnResult, nil
}
```

---

## Designing Negative & Bypass Test Suites

For security-sensitive code (permits, obligations, gateways), a successful happy-path test is insufficient. You must test:

1. **Permit Tampering**: Mutate one claim at a time (change tenant, change arguments hash, change expiration) and assert that downstream execution is prevented (zero downstream calls).
2. **Replay Attacks**: Submit an identical permit concurrently and sequentially. Assert that only the first call succeeds and all replays fail.
3. **Fail-Closed Downstreams**: Simulate downstream timeouts, connection resets, and malformed responses. Verify that receipts record status `unknown` or `failed`, never `succeeded`.
4. **Boundary Numbers**: In Canonical JSON, test values at `9007199254740991` (max safe integer), `0`, `-0`, and `-9007199254740991`.

---

## Writing Fuzz Targets (`testing.F`)

Fuzz targets prevent parser crashes and infinite loops under untrusted inputs:

```go
func FuzzEventEnvelopeParse(f *testing.F) {
	// Seed corpus with known valid fixtures
	f.Add([]byte(`{"schema_version":"1.0","event_id":"evt_1"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Target should gracefully return an error or valid envelope, never panic or hang
		_, _ = ParseEventEnvelope(data)
	})
}
```
Run fuzz targets during development with:
```bash
go test ./internal/... -fuzz=FuzzEventEnvelopeParse -fuzztime=10s
```

---

## Mandatory Pre-Completion TDD Checklist

- [ ] A failing test was written and observed failing before implementation started.
- [ ] Table-driven tests use `t.Run` with descriptive subtest names.
- [ ] Negative and tamper test cases are included for every new security property.
- [ ] No real network or wall-clock dependencies exist in unit tests.
- [ ] `go test -race ./...` passes with zero data races reported.
