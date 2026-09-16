# ADR 0004: Minimize evidence payloads

Status: accepted design; receipt persistence deferred to Phase 3.

Receipts store digests, stable data references, policy/configuration/fact versions,
decision results and enforcement outcomes. Raw prompts, arguments, model output
and secret values are not part of the default receipt shape. Error envelopes contain
safe fixed messages and validated correlation IDs rather than parser excerpts.

Replay requires a separately governed snapshot store resolving input references.
Digests alone cannot reconstruct input. Retention, deletion and access control for
that store must be defined before replay is offered in Phase 4. A hash chain makes
changes detectable only relative to a trusted checkpoint; it is not immutable storage.

Evidence: NG-F002 receipt fixtures; NG-F006 error tests.
