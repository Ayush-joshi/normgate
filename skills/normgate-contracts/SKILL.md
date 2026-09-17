---
name: normgate-contracts
description: >-
  Guide for evolving NormGate JSON schemas, executing multi-language code generation
  (Go, Python, TypeScript, OpenAPI), enforcing domain-neutral vocabulary, and maintaining
  byte-identical Canonical JSON v1 and SHA-256 vector parity across runtimes.
---

# NormGate Contracts & Code Generation

## Overview

In NormGate, JSON Schemas under `schemas/v1/*.schema.json` are the authoritative single source of truth for all data shapes, envelopes, and protocol contracts. Code in Go, Python, and TypeScript—as well as OpenAPI 3.1 specifications—are strictly generated from these schemas.

Furthermore, NormGate enforces strict cross-language canonical serialization (Canonical JSON profile v1) and SHA-256 hashing to guarantee that operations hash to byte-identical digests regardless of whether they are processed in Go, Python, or TypeScript.

---

## When to Use

- When adding or modifying fields in any core contract (e.g. `EventEnvelope`, `Decision`, `Permit`, `Receipt`, `Obligation`).
- When introducing a new schema file under `schemas/v1/`.
- When diagnosing `make check-generated` drift or type mismatches across SDKs.
- When working on Canonical JSON serialization or adding shared normalization test vectors.

---

## DO THIS / NEVER DO THIS (Engineering Guardrails)

| Never Do This (Anti-Patterns) | Do This Instead (NormGate Standard) |
| :--- | :--- |
| ❌ Never edit `internal/contracts/v1/types.gen.go` directly. | ✅ Edit `schemas/v1/*.schema.json` and run `make generate`. |
| ❌ Never edit `sdk/python/.../types/__init__.py` or TS types directly. | ✅ Edit `schemas/v1/*.schema.json` and run `make generate`. |
| ❌ Never introduce industry terms (`patient_id`, `bank_account`, `hipaa`). | ✅ Use generic domain-neutral terms (`principal_id`, `resource_id`, `classification`). |
| ❌ Never use floating point `number` types in canonical contracts. | ✅ Use `"type": "integer"` between `-9007199254740991` and `9007199254740991`. |
| ❌ Never leave `"additionalProperties"` unspecified. | ✅ Always set `"additionalProperties": false` on every object definition. |
| ❌ Never commit schema changes without running `make check-generated`. | ✅ Always run `make generate && make check-generated` before committing. |

---

## Authoritative Schemas in `schemas/v1/`

The 14 core contracts in `schemas/v1/` include:
- `event-envelope.schema.json`: Canonical event wrapping principal, operation, destination, context.
- `decision.schema.json`: Policy outcome (`allow`, `deny`, `transform`, `require_approval`), reasons, obligations.
- `permit.schema.json`: Cryptographic permit binding identity, payload digest, policy revision, nonce, expiration.
- `receipt.schema.json`: Hash-chained execution and decision receipts.
- `obligation.schema.json`: Obligations (`remove_paths`, `mask_values`, `restrict_destination`, etc.).
- `operation.schema.json`, `destination.schema.json`, `principal.schema.json`, `resource.schema.json`, `external-fact.schema.json`, `configuration.schema.json`, `policy-manifest.schema.json`, `error-envelope.schema.json`, `test-case.schema.json`.

---

## Concrete Schema Template (Draft 2020-12)

When creating a new schema file `schemas/v1/my-contract.schema.json`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://normgate.dev/schemas/v1/my-contract.schema.json",
  "title": "MyContract",
  "type": "object",
  "additionalProperties": false,
  "required": [
    "schema_version",
    "id",
    "created_at"
  ],
  "properties": {
    "schema_version": {
      "type": "string",
      "const": "1.0"
    },
    "id": {
      "type": "string",
      "minLength": 1,
      "maxLength": 128
    },
    "created_at": {
      "type": "string",
      "format": "date-time"
    },
    "labels": {
      "type": "array",
      "items": { "type": "string" },
      "x-normgate-set": true
    },
    "metadata": {
      "type": "object",
      "additionalProperties": true
    }
  }
}
```

> [!CRITICAL]
> **Domain Neutrality Rule (`NG-F007`)**:
> Core schemas must NEVER contain industry-specific or domain-specific vocabulary.
> CI tests (`test/architecture/dependencies_test.go`) assert that words matching `(?i)\b(hipaa|phi|fhir|patient|diagnosis|healthcare|banking|student|education)\b` do not appear anywhere in `schemas/v1/*.json`. Use generic abstractions (`data_references`, `classifications`, `destinations`, `obligations`).

---

## Step-by-Step Workflow for Modifying Contracts

```text
1. Edit or create schemas/v1/<name>.schema.json
2. Run generator: make generate
3. Verify clean generation: make check-generated
4. Run compiler checks:
   - Go: go vet ./internal/contracts/...
   - TypeScript: npm run typecheck
   - Python: python3 -m compileall -q sdk/python/src
5. Run architecture check: go test ./test/architecture -run TestNGF007Vocabulary
6. Run canonical vector tests: make test-contract
```

---

## Canonical JSON Profile v1 & Vector Parity (`NG-F004`)

NormGate uses a custom, strict Canonical JSON specification (documented in [docs/architecture/canonicalization.md](../../docs/architecture/canonicalization.md)).

### Core Rules of the Profile
1. **Size & Nesting Limits**: Maximum 1 MiB payload size, maximum 64 nesting levels (depth 0 at root).
2. **Numbers**: Only integer JSON number tokens between `-9007199254740991` and `9007199254740991` (inclusive). Rejects decimals, exponents (`1.0`, `1e0`). Normalizes `-0` to `0`.
3. **Strings & Unicode**: Normalized to Unicode NFC. Rejects invalid UTF-8, unpaired surrogates, and more than 30 consecutive Unicode Mark characters.
4. **Key Sorting**: Object keys are sorted by Unicode scalar value (UTF-8 lexicographic order). (Note: JS must not use default UTF-16 sort).
5. **Array & Set Handling**: Array order is preserved unless the RFC 6901 pointer is marked as an `x-normgate-set` in the profile. Sets contain unique strings, normalized and sorted.
6. **Timestamps**: Explicit timestamp paths must be converted to UTC with uppercase `T` and `Z`, trailing fractional zeros stripped, years between 0001–9999.
7. **Omission**: Only non-authoritative transport fields are omitted; `/operation/arguments/transport` is NOT omitted.
8. **Hashing**: Text digests use `sha256:` followed by 64 lowercase hex characters.

### Adding Shared Vectors
When touching canonicalization code or testing new edge cases:
1. Add the vector to `testkit/fixtures/normalization/vectors.json`:
```json
{
  "id": "vector_new_test_case",
  "raw": "{\"b\": 1, \"a\": \"test\"}",
  "canonical": "{\"a\":\"test\",\"b\":1}",
  "sha256": "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
}
```
2. Run vector validation across all 3 languages:
```bash
make test-contract
```

---

## Mandatory Pre-Completion Contracts Checklist

- [ ] Every modified or added schema is in `schemas/v1/*.schema.json`.
- [ ] No generated files (`types.gen.go`, `__init__.py`, `types/index.ts`, `openapi.gen.json`) were edited by hand.
- [ ] `make generate` executed.
- [ ] `make check-generated` passes with exit code 0.
- [ ] `TestNGF007Vocabulary` passes (zero domain words).
- [ ] `npm run typecheck` passes.
- [ ] `make test-contract` passes in Go, Python, and Node.
