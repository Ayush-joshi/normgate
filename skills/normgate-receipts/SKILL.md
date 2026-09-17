---
name: normgate-receipts
description: >-
  Guide for handling verifiable receipts, hash-chained SQLite evidence stores,
  tamper detection (`normgate audit verify`), and ADR 0004 payload minimization
  standards in NormGate.
---

# NormGate Evidence, Receipts & Audit Storage

## Overview

NormGate produces verifiable, tamper-evident receipts for every policy decision and downstream execution. Receipts provide non-repudiable proof of what operation was proposed, what policy evaluated it, what decision was rendered, and what the actual downstream outcome was.

Evidence is governed by strict privacy and minimization standards: **receipts record references and cryptographic digests, never raw user prompts, model responses, or sensitive payloads**.

---

## When to Use

- When implementing or modifying receipt writers in `internal/receipt` or `internal/storage`.
- When designing SQLite evidence schemas, queries, or index migrations.
- When working with cryptographic hash chaining and chain validation algorithms.
- When implementing or using `normgate audit verify` or `normgate audit export`.
- When reviewing code to prevent accidental PII or credential leakage into evidence.

---

## DO THIS / NEVER DO THIS (Engineering Guardrails)

| Never Do This (Anti-Patterns) | Do This Instead (NormGate Standard) |
| :--- | :--- |
| ❌ Never store raw user prompts or tool arguments in receipts. | ✅ Store the canonical `sha256:` digest of the arguments. |
| ❌ Never store raw model output or tool response bodies in receipts. | ✅ Store the response digest and execution outcome status. |
| ❌ Never record an interrupted execution as `succeeded`. | ✅ Mark incomplete executions as `unknown` or `failed`. |
| ❌ Never compute receipt hash without chaining the previous hash. | ✅ Include `previous_receipt_hash` in canonical bytes before hashing. |
| ❌ Never mix multiple tenants into one sequential hash chain. | ✅ Maintain an independent, strictly ordered hash chain per tenant. |
| ❌ Never log raw payloads or secrets in OpenTelemetry spans. | ✅ Export only spans with operation names, status, and event digests. |

---

## Architecture: Two-Phase Linked Receipts

NormGate records evidence in two phases linked by `decision_id`:

```text
1. Policy Evaluation
   └─► Decision Receipt (decision_id, tenant_id, event_digest, policy_revision, outcome, obligations)
            │
            ▼ (Permit issued & downstream invoked)
2. Downstream Execution
   └─► Enforcement Receipt (decision_id, permit_id, status: succeeded/failed/unknown, duration_ms, prev_hash, receipt_hash)
```

Linking these records ensures that a crash or disruption between authorization and execution is transparently recorded: an incomplete execution has status `unknown` and can never be mistaken for `succeeded`.

---

## Payload Minimization (ADR 0004)

NormGate enforces strict data minimization across all receipts and telemetry:

| Data Type | Allowed in Receipts? | What is Stored Instead? |
| :--- | :--- | :--- |
| Raw prompts / chat history | ❌ **PROHIBITED** | Canonical SHA-256 digest of normalized payload |
| Tool call arguments | ❌ **PROHIBITED** | SHA-256 argument digest; names of applied obligations |
| Downstream tool response | ❌ **PROHIBITED** | SHA-256 response digest; execution status (`succeeded`, `failed`) |
| API keys / credentials | ❌ **PROHIBITED** | Redacted; omitted from evidence entirely |
| Tenant & Principal IDs | ✅ Allowed | Identifiers for audit correlation |
| Destination ID & Op Name | ✅ Allowed | Canonical resource identifiers (e.g. `slack_tool`, `calculator`) |
| Policy Revision & Rules | ✅ Allowed | Content-addressed SHA-256 hash of policy bundle and matched rule IDs |

---

## Cryptographic Hash Chaining

To ensure receipts cannot be retroactively modified or reordered:
1. Receipts are partitioned by `tenant_id`.
2. Each receipt contains a `previous_receipt_hash`.
3. The receipt's own `receipt_hash` is computed as:
   $$\text{receipt\_hash}_n = \text{SHA-256}(\text{receipt\_hash}_{n-1} \parallel \text{CanonicalJSON}(\text{ReceiptClaims}_n))$$

### Concrete Go Chaining Snippet

```go
func ComputeReceiptHash(prevHash string, claims contracts.ReceiptClaims) (string, error) {
	canonicalBytes, err := normalization.Canonicalize(claims)
	if err != nil {
		return "", fmt.Errorf("canonicalization failed: %w", err)
	}
	hasher := sha256.New()
	hasher.Write([]byte(prevHash))
	hasher.Write(canonicalBytes)
	return fmt.Sprintf("sha256:%x", hasher.Sum(nil)), nil
}
```

### Verifying Chain Integrity
To verify the evidence store against tampering:
```bash
./bin/normgate audit verify --store /path/to/receipts.db
```
The verifier:
- Re-reads all records in sequence.
- Recalculates canonical JSON bytes for each receipt.
- Verifies that `previous_receipt_hash` matches record $n-1$.
- Flags any gap, deletion, or modified record.

---

## Storage Engine Patterns (SQLite)

```sql
CREATE TABLE IF NOT EXISTS receipts (
    decision_id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    sequence_number INTEGER NOT NULL,
    event_digest TEXT NOT NULL,
    policy_revision TEXT NOT NULL,
    outcome TEXT NOT NULL,
    execution_status TEXT NOT NULL,
    previous_receipt_hash TEXT NOT NULL,
    receipt_hash TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE(tenant_id, sequence_number)
);
CREATE INDEX IF NOT EXISTS idx_tenant_seq ON receipts(tenant_id, sequence_number);
```

1. **Write-Ahead Logging (WAL)**:
   - Always open SQLite connections with `_journal_mode=WAL` and `_busy_timeout=5000` to support concurrent readers alongside the single writer.
2. **Crash Resilience**:
   - Write operations must use atomic transactions (`BEGIN IMMEDIATE ... COMMIT`).
   - If an unexpected process crash occurs during downstream execution, startup recovery scans for open decisions without matching enforcement receipts and marks them `unknown`.

---

## Mandatory Pre-Completion Receipts Checklist

- [ ] Receipts contain digests and references only; zero raw prompts, arguments, or payloads.
- [ ] Receipts are linked by `decision_id`.
- [ ] Each tenant has an independent, sequential hash chain.
- [ ] Interrupted or failed executions are recorded as `unknown` or `failed`.
- [ ] `normgate audit verify` passes on test databases.
- [ ] Package statement coverage in `internal/receipt` is $\ge 95\%$.
