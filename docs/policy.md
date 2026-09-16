# Deterministic policies

Phase 2 provides local policy evaluation and a library for safe policy lifecycle
management. It does not execute protected operations. The enforcement service,
caller authentication, permits and receipts are Phase 3 work.

## CLI

```sh
make build
./bin/normgate policy init ./my-policy
./bin/normgate policy lint ./my-policy
./bin/normgate policy test ./my-policy
./bin/normgate policy build ./my-policy --out ./policy.tar.gz
./bin/normgate policy inspect ./policy.tar.gz
./bin/normgate policy explain --bundle ./policy.tar.gz --event ./event.json
./bin/normgate policy diff ./old.tar.gz ./policy.tar.gz
./bin/normgate policy activate --bundle ./policy.tar.gz --state ./policy-state
./bin/normgate policy status --state ./policy-state
./bin/normgate policy rollback --revision sha256:REPLACE_WITH_EXACT_REVISION --state ./policy-state
```

`event.json` contains a canonical v1 EventEnvelope object, without a fixture
wrapper. Unknown fields and invalid schema values are rejected. `explain` prints
the runtime decision, contributing rule IDs and per-obligation provenance. The
reason catalog supplies developer descriptions and remediation. `diff` includes
changed paths, before/after content and digests, including rules, data and scenarios;
review that semantic change alongside expectations when changing policy behavior.

`init` refuses an existing path; `build` refuses to overwrite a file. Build and
activation both compile and run the bundle's scenarios. Syntax, input, and source
failures return stable codes without echoing policy or event contents. Compilation
diagnostics include the source path, row and column. Exit codes are 0 for success,
1 for validation/evaluation/lifecycle failure, and 2 for command usage errors.

## Bundle format

A directory or deterministic gzip-compressed tar archive contains:

| File | Purpose |
| --- | --- |
| `manifest.json` | Existing v1 PolicyManifest contract, identity/version, owned namespaces, dependencies, fact sources and digests |
| `layers.json` | Ordered policy layers with `name`, `namespace`, unique integer `priority` |
| `*.rego` | Rego v1 modules; packages must exactly match declared layer namespaces |
| `data.json` | Static policy data under the sole top-level `config` key |
| `reasons.json` | Reason code to `{description, remediation}` mapping |
| `tests/*.yaml` or `tests/*.json` | Exact decision and explanation scenario assertions |
| `dependencies.json` (optional) | Embedded v1 manifests for dependencies supplied in this same artifact |

Dependencies use exact `id@major.minor.patch` pins. Missing, incompatible or cyclic
dependencies fail validation. Namespace ownership cannot overlap, including parent
and child namespaces. Minimum core versions must have the current core major and
be no greater than 0.2.0. All modules and data are supplied in the single archive;
dependency manifests do not trigger external downloads or grant signature trust.
The root manifest declares the union of external-fact sources the artifact needs.

The source digest hashes a sorted sequence of file paths and SHA-256 file hashes,
with the manifest serialized as JSON and both self-referential digest fields set
to zero hashes. The build digest repeats this with the source digest filled in and
only the build digest zeroed. The build digest is the immutable policy revision.
All supplied files, including tests and reason metadata, participate. Tar entry
order, permission metadata and modification times do not affect the revision;
`build` fixes those fields and emits reproducible archive bytes.

Archive readers verify both digests. They reject traversal, absolute paths,
backslashes, duplicate entries, links, special files, truncated gzip/tar and
oversized inputs. Limits are 4 MiB of file content, 1 MiB per file, 256 files,
64 modules, 2,048 top-level rules, lexical nesting and rule dependency depth 64,
128 cached revisions per engine, and 50 ms per evaluation. Compile uses a 5-second
context budget with checks between bounded parsing/compilation stages; OPA's
synchronous AST compiler is not preempted mid-stage. Evaluation is context-aware
and supports cancellation during execution.

`signing_identity` is `unsigned` or a nonempty `key:` reference. It is metadata;
inspection always reports `signature_verified: false`. Remote signature trust and
key distribution remain Phase 5 work. HTTPS and OCI sources require an independent
SHA-256 pin and never infer authenticity from the signing identity.

## Evaluation and composition

Layers are `baseline`, `organization`, `tenant`, `application`, `local_override`.
Baseline is mandatory. Each configured layer has one distinct namespace and an
increasing unique priority. Every module entry point defines `default decisions :=
[]`; matching rules publish an array of partial decisions:

```rego
package example.tenant

default decisions := []
decisions := [{
    "rule_id": "confirm_external",
    "outcome": "require_approval",
    "reason_codes": ["ng.policy.external_approval"],
    "obligations": [{
        "schema_version": "1.0",
        "id": "external_approval",
        "kind": "require_approval",
        "reviewer_group": "owners",
    }],
}] if input.operation.destination.id == "external"
```

Declare the reason code and namespace before using this example. A configured
layer returning no decision denies the operation. To make a constraint optional,
write an explicit allow for its non-applicable case. Layer names are assigned by
the engine; policy cannot impersonate another layer. Duplicate rule IDs in a
layer, unknown reasons and malformed outputs fail closed.

Any deny wins; approval prevents direct allow; compatible transformations merge
in stable obligation-ID order. Different operations on overlapping JSON pointers,
conflicting destination restrictions, or different definitions of one obligation
ID deny. Identical obligation IDs coalesce while retaining every contributing rule
in explanation provenance. Retain-path rules merge only when their sets match;
mixed retain/field transforms are conservatively treated as conflicts. Multiple
approval groups are cumulative. Record/byte caps remain cumulative constraints.
Deny decisions contain no executable obligations.

OPA types remain inside `internal/policy/opa`. The compiler uses an explicit list
of deterministic built-ins. Network, clock, random, runtime introspection, print,
filesystem and unapproved functions are unavailable. Strict compilation rejects
unused variables, recursive dependencies, missing entry defaults, orphaned helper
rules, unknown literal reasons and invalid literal obligations. Dynamic outputs
are validated against the same contracts at evaluation time.

Inputs normalize Unicode, declared string sets and timestamps. Transport metadata
is excluded. Decisions use the canonical event time as `evaluated_at`; they never
read wall-clock time. IDs bind the normalized event and revision. External facts
are required by declared source, must have unique IDs and be valid at event time
(`resolved_at <= occurred_at < expires_at`). Undeclared facts remain in the replay
digest but are hidden from Rego, including through aliases and dynamic references.
The caller must authenticate authoritative identity/context and resolve trustworthy
facts before invoking the engine; a registered principal ID alone is not proof of
caller authentication.

## Scenarios

A scenario contains `id`, `event`, and `expected` with exact `outcome`, sorted
`reason_codes`, ordered `obligations`, and sorted `rule_ids`. An optional
`expected_error` asserts a stable engine error for a noncanonical event. The runner
rejects unknown fields, duplicate IDs/keys, YAML aliases/anchors/custom tags,
multiple documents, and missing tests. JSON is valid YAML; the baseline uses that
unambiguous subset. These are internal policy scenarios, not the public TestCase
transport contract containing an already-complete Decision.

The generic baseline includes 27 scenarios covering identity registration, tenant
isolation, all eight known operation kinds, operation-name registration, exact
destination registration, declared purpose and application/environment context.
Empty required identity/purpose/context fields and unknown operation kinds fail
canonical validation before Rego. The template registers only explicit example
identities and destinations; replace them and update scenarios for deployment.

## Activation and sources

`policy.Manager` stages bounded input, verifies archive digests, compiles, checks
engine health and executes every bundled scenario before persisting an immutable
archive and atomically switching the active snapshot. Readers use one revision
for the entire evaluation. Failed candidates retain the active revision and set a
redacted `last_error`. State files use temporary-file write, fsync, rename and
directory fsync. Use one manager/writer per state directory.

Startup re-verifies and retests the saved active artifact; if it cannot recover,
only the saved previous healthy revision is eligible for fallback. Missing or
corrupt artifacts never synthesize an allow. Rollback accepts an exact persisted
revision, retests it, and clears remote ETags. Artifacts are retained for exact
rollback; storage retention and cache eviction are explicit future operator work.
The bounded revision cache rejects further distinct revisions rather than evicting
an in-use revision. A restart reconstructs it from durable policy state.

Source implementations are `FileSource`, `NewHTTPSource`, and `NewOCISource`, behind
`Source.Fetch(ctx, etag)`. HTTPS rejects insecure URLs, credentials, query strings,
fragments and redirects; requests have a 30-second cap. OCI uses a pinned OCI image
manifest with artifact and single-layer media type
`application/vnd.normgate.policy.v1+tar+gzip`, verifies its digest, then verifies
layer digest and size. Custom HTTP clients can provide registry authentication;
there is no implicit credential discovery or login. Only distribution fetches use
network access; policy evaluation never does.

`Manager.Refresh` applies a source update. A 304 refreshes freshness only for the
same already-active source and checksum pin. A changed remote revision requires a
new pin/source configuration. `Manager.Run` polls with bounded exponential retry
and jitter. `Status` exposes source, revision, previous revision, ETag, refresh time,
readiness and last error. Readiness and evaluation fail when maximum staleness is
reached, even if refresh has stopped. These are library health APIs; HTTP service
routes arrive in Phase 3. CLI lifecycle commands default to 24 hours maximum
staleness and accept `--max-staleness`.

## Verification

```sh
make test-unit PKG=./internal/policy/...
make test-policy
make test-contract
make test-race
make test-fuzz-smoke
make coverage
make test-mutation
make benchmark-policy
make verify
```

Policy and OPA packages each require at least 90% statement coverage. Architecture
tests reject OPA imports elsewhere. Four policy fuzz targets exercise archive and
manifest parsing, composition monotonicity/order independence, and explanation
serialization. Mutation tests use temporary Go overlays and must produce actual
behavioral test failures. Benchmarks report cold compile, cached allow evaluation,
and the complete baseline fixture mix, including p50/p95/p99 and allocations.
