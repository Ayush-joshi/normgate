# ADR 0002: Fail closed at authoritative boundaries

Status: accepted; configuration enforcement implemented in Phase 1.

Unavailable authoritative policy, invalid context, unsupported operations and
failed required dependencies must deny execution. The model may supply facts but
cannot grant permission. There is no fail-open configuration switch: `fail_closed`
must be true. Future local caches must preserve this invariant when stale or invalid.

Availability may decrease during dependency failures. Operational readiness and
explicit denial reasons are preferred to bypassing authorization. Phase 2 and 3
tests must prove this invariant in executable policy and gateway paths.

Evidence: NG-F005, `TestNGF005Invalid`.
