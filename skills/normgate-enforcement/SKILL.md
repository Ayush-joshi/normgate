---
name: normgate-enforcement
description: >-
  Guide for implementing NormGate runtime gateways (Inference, Tools/MCP, Retrieval/Memory),
  enforcement adapters, Ed25519 signed permits, obligation execution engines,
  anti-replay nonces, and downstream execution security verification.
---

# NormGate Enforcement Runtime & Gateways

## Overview

In NormGate, authorization is enforced outside the language model at the exact point where an operation executes. A decision endpoint alone does not protect a system—enforcement requires cryptographic permits, deterministic obligation execution, final-point verification, and anti-bypass controls.

This skill outlines the standard architecture and implementation patterns for all NormGate gateways (Inference, Tools/MCP, Storage/Memory, and HTTP service).

---

## When to Use

- When building or modifying runtime gateways (`cmd/normgate`, `internal/enforcement`, `internal/api`).
- When implementing obligation handlers (`remove_paths`, `mask_values`, `tokenize_values`, `limit_records`, `restrict_destination`).
- When working with cryptographic permits, Ed25519 signing/verification, and nonce replay prevention.
- When implementing downstream execution adapters (tool executors, model proxies, storage connectors).
- When writing negative, bypass, or permit mutation test suites.

---

## DO THIS / NEVER DO THIS (Guardrails for AI Models)

| Never Do This (Common Model Mistakes) | Do This Instead (NormGate Standard) |
| :--- | :--- |
| ❌ Never invoke downstream services without final-point permit verification. | ✅ Verify Ed25519 signature and payload digest immediately before the network call. |
| ❌ Never trust caller-supplied `X-User-Id` or `X-Tenant-Id` headers. | ✅ Authenticate credentials and derive principal/tenant from trusted token claims. |
| ❌ Never proceed with execution if an obligation partially fails. | ✅ If any obligation cannot be executed completely, abort and fail closed with a denial. |
| ❌ Never allow nonces to be reused or evaluated non-atomically. | ✅ Check and consume the permit nonce in an atomic database transaction. |
| ❌ Never stream unbuffered model output directly to callers. | ✅ Buffer streaming segments and evaluate against response policy before delivering bytes. |
| ❌ Never import `cmd/` packages into `internal/enforcement`. | ✅ Keep gateway logic inside `internal/`; CLI entry points in `cmd/` delegate inward. |

---

## The Canonical Enforcement Pipeline

Every request flowing through a NormGate gateway must follow this exact sequence:

```text
1. Untrusted Caller Request
       │
       ▼
2. Middleware & Context Derivation
   - Authenticate caller (credentials, tokens).
   - Derive principal & tenant from authenticated context (never from caller-supplied headers).
   - Resolve authoritative destination and external facts from trusted sources.
       │
       ▼
3. Canonical Normalization
   - Parse and validate input against JSON schema.
   - Construct canonical contracts.EventEnvelope.
       │
       ▼
4. Policy Evaluation
   - Query deterministic policy engine for the active revision.
   - Receive contracts.Decision (allow, deny, transform, require_approval).
       │
       ▼
5. Obligation Execution
   - If decision is deny: return structured error, make 0 downstream calls.
   - If require_approval: return blocked error, make 0 downstream calls.
   - If transform/allow: apply obligations in documented deterministic order.
       │
       ▼
6. Permit Issuance & Binding
   - Generate single-use nonce.
   - Bind principal, tenant, operation, arguments digest (post-transform), destination, policy revision.
   - Sign permit claims with Ed25519 private key.
       │
       ▼
7. Final-Point Permit Verification
   - Verify Ed25519 signature with active public key.
   - Verify payload digest matches actual arguments to be forwarded.
   - Atomically consume nonce in replay store.
       │
       ▼
8. Downstream Execution
   - Forward validated, transformed payload to downstream model, tool, or storage.
       │
       ▼
9. Response Evaluation
   - Evaluate downstream response against response policies (e.g. data leak checks).
   - Withhold response if policy denies.
       │
       ▼
10. Evidence Receipt
   - Write verifiable, hash-chained receipt recording decision and actual outcome.
```

---

## Obligation Execution Standards

Obligations represent mandatory transformations and runtime restrictions returned by policies:

| Obligation Kind | Behavior | Guardrails |
| :--- | :--- | :--- |
| `remove_paths` | Deletes specified JSON pointers from payload | If path doesn't exist, ignore cleanly; cannot increase payload size. |
| `retain_paths` | Retains only specified JSON pointers | Conservative: if conflict with remove_paths occurs, engine denies. |
| `mask_values` | Masks sensitive string values with pattern | Preserves data type; does not alter unclassified fields. |
| `tokenize_values`| Replaces values with reversible or synthetic tokens | Must be deterministic per session/tenant. |
| `limit_records` | Clamps max array items or query results | Monotonic: lower limit always takes precedence. |
| `limit_bytes` | Restricts maximum payload byte size | Clamped strictly before network transmission. |
| `restrict_destination` | Restricts permitted IP/host sinks | Destination must match configured allowlist. |

### Invariant: All-or-Nothing Obligation Execution
If any obligation cannot be executed completely (e.g. invalid JSON pointer, incompatible type, memory limit exceeded), the operation **MUST fail closed with a denial**. Partial transformations are strictly forbidden.

---

## Cryptographic Permits (`contracts.Permit`)

A permit is a short-lived cryptographic authorization token binding:
- `principal_id` and `tenant_id`
- `operation_kind` and `operation_name`
- `destination_id`
- `arguments_digest`: `sha256:` of the canonicalized transformed arguments.
- `policy_revision`: Content digest of the active policy bundle.
- `nonce`: Cryptographically random UUID/token valid for exactly one use.
- `expires_at`: Timestamp (typically $\le 60$ seconds from issuance).
- `signature`: Ed25519 signature over canonical claims.

### Replay Prevention
Before invoking downstream execution, the permit verifier must atomically check and consume the nonce in a transactional store (e.g. SQLite or Redis). If the nonce has already been consumed or is expired, downstream execution is aborted.

---

## Concrete Negative & Bypass Test Matrix Template

When adding or testing gateway components, implement this test matrix:

```go
func TestPermitVerificationRejectsMutations(t *testing.T) {
	signer, verifier := setupTestKeys(t)
	validPermit := createSignedPermit(t, signer)

	tests := []struct {
		name    string
		mutate  func(p *contracts.Permit)
		wantErr string
	}{
		{
			name: "tampered principal ID",
			mutate: func(p *contracts.Permit) { p.PrincipalId = "attacker" },
			wantErr: "signature verification failed",
		},
		{
			name: "tampered arguments digest",
			mutate: func(p *contracts.Permit) { p.ArgumentsDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000" },
			wantErr: "payload digest mismatch",
		},
		{
			name: "expired permit",
			mutate: func(p *contracts.Permit) { p.ExpiresAt = time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339) },
			wantErr: "permit expired",
		},
		{
			name: "tampered destination ID",
			mutate: func(p *contracts.Permit) { p.DestinationId = "evil-sink" },
			wantErr: "signature verification failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := clonePermit(validPermit)
			tt.mutate(p)
			err := verifier.Verify(context.Background(), p, validPayload)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
```

---

## Mandatory Pre-Completion Enforcement Checklist

- [ ] Downstream executor calls `Verifier.Verify` immediately before the network call.
- [ ] Nonce is atomically consumed before execution begins.
- [ ] Every permit field has an automated negative mutation test asserting execution is blocked.
- [ ] If downstream execution fails or times out, the outcome is recorded as `failed` or `unknown`, never `succeeded`.
- [ ] Caller identity is derived from authenticated context, never unauthenticated headers.
- [ ] Package statement coverage in `internal/enforcement` is $\ge 95\%$.
