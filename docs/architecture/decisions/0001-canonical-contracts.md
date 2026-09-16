# ADR 0001: Schema-first deterministic contracts

Status: accepted for Phase 1.

JSON Schema Draft 2020-12 is the contract source of truth. Each public object has a
schema version. Go structs, Python TypedDict models, TypeScript types and OpenAPI
3.1 components are generated; runtime Go validation uses embedded schemas. Static
models never substitute for validating untrusted input.

Canonicalization uses the explicit NormGate v1 profile and shared golden vectors.
Safe integers and strict Unicode prevent common cross-runtime reinterpretation.
This limits number representation intentionally; fractional facts must be encoded
as versioned strings or external references until a later contract defines them.

Evidence: NG-F002, NG-F003, NG-F004. Source documentation:
[JSON Schema 2020-12](https://json-schema.org/draft/2020-12).
