# ADR 0003: Bind permits to exact operations

Status: accepted design; signing and enforcement deferred to Phase 3.

Permits bind the principal, tenant, operation digest, destination, classifications,
purpose, policy revision, fact digest, obligations digest, nonce and validity
interval. The normalized event contains argument digests and data references;
an adapter must compute those digests from the actual final arguments. It must
never trust a caller to supply the digest of arguments it will execute.

Any post-decision mutation requires reevaluation. An approved request must also be
reevaluated immediately before execution. Ed25519 verification alone will not prove
nonce freshness, current policy validity, tenant consistency or satisfied obligations.

Phase 1 validates permit shape only. NG-F002 fixtures reject missing binding fields.
Phase 3 must freeze full binding profiles and test mutation, expiry and replay.
