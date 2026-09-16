# ADR 0005: Independent contract versions

Status: accepted for Phase 1.

Public object versions use `major.minor`, beginning with `1.0`. The first major has
a fixed vocabulary and rejects unknown fields. A later `1.x` record is accepted
only when it remains valid under the existing schema. Namespaced `extensions`
keys allow optional non-authoritative metadata without reserving domain vocabulary.
Unsupported majors fail explicitly. New required fields, operation meanings,
normalization rules or authority-bearing metadata require a major-version review.

Unknown namespaced metadata never grants authority. Components must not silently
interpret extensions as core policy fields. Binary/package semantic versions are
independent from schema versions; Phase 2 defines policy package compatibility.

Fixture changes and regeneration require reviewer assessment. Acceptance of a
`1.1` version string does not mean arbitrary future fields or behaviors are supported.

Evidence: NG-F002 compatibility fixtures and `TestNGF002SafeDecode`.
