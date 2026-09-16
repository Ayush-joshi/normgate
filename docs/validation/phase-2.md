# Phase 2 validation record

Date: 2026-09-17. Status: **Phase 2 implementation complete; full local verification
and built-binary CLI smoke passed**. Phase 3 is next.

## Delivered

- Internal Engine boundary with Compile, Evaluate, Explain, Health and idempotent
  Close; a shared fake/OPA conformance suite and pinned OPA v1.20.1 implementation.
- Prepared immutable revisions, canonical event normalization and replay, explicit
  deterministic built-in capabilities, schema-validated outputs, bounded parsing,
  compilation/evaluation contexts and concurrent evaluation.
- Baseline → organization → tenant → application → local override composition,
  fail-closed missing layers and conflicts, approval precedence, deterministic
  obligation merging and full rule/obligation provenance.
- Reproducible content-addressed bundles, digest verification, path/size limits,
  embedded dependency validation, namespace ownership and inspection reports.
- CLI init, lint, build, test, inspect, diff, explain, activate, status and exact
  rollback. Binary/core version is 0.2.0-dev/0.2.0.
- Filesystem, pinned HTTPS and OCI manifest/blob sources, ETags, bounded retry with
  jitter, maximum staleness, immutable persistence and atomic reader activation.
  Startup recovery re-verifies/retests active or previous known-good artifacts.
- Generic baseline policy with 27 scenarios, developer reasons/remediation, and
  documentation in [policy development](../policy.md).
- Architecture enforcement, 90% coverage gates for both policy packages, four
  policy fuzz targets and six behavioral composition mutants in `make verify`.

No public schema changes were required. OPA raises the Go module/workspace language
baseline to 1.26. The existing Go 1.26.8 CI toolchain meets that requirement.

## Environment and checks

`make verify` completed successfully. The final CLI version alignment was also
checked with focused CLI/architecture tests and the built-binary smoke flow.

macOS ARM64, Apple M4, Go 1.26.8, Python 3.12.14, Node.js 24.4.0;
`govulncheck` v1.8.0. Tests run against the working repository and pinned dependency
caches. Local HTTPS test servers require permission outside the filesystem/network
sandbox; no external application, registry or deployment was modified.

| Check | Result |
| --- | --- |
| Generated artifacts, formatting, vet, TypeScript, architecture | Pass |
| Unit, CLI, fake/OPA conformance and contract tests | Pass |
| Baseline policy scenarios | 27/27 pass |
| Cross-language canonical vectors | 39/39 in Go, Python and TypeScript |
| Race detector, including simultaneous activation/evaluation | Pass |
| Policy statement coverage | 94.35% (required 90%) |
| OPA statement coverage | 93.49% (required 90%) |
| Repository statement coverage | 92.59% (required 85%); no production package excluded |
| Fuzz smoke | Pass; all seven targets ran for 30 seconds in the final gate |
| Mutation tests | 6/6 behavioral mutants killed |
| License review | 128 pinned license files verified, including the complete OPA module graph |
| Go vulnerability scan | No affected symbols or imported packages; four advisories in unused modules in the dependency graph |
| npm audit | Zero vulnerabilities |
| Binary/CLI smoke | Pass; 0.2.0-dev, init/lint/test/build/inspect/explain/diff/activate/status/rollback and overwrite protection |

Failure tests cover archive traversal/links/duplicates/corruption, source checksum
mismatch, interrupted responses, redirects, missing/incompatible/cyclic dependencies,
namespace/priority collisions, unknown reasons/obligations, failing scenario or
candidate health checks, invalid persisted state, failed persistence, stale-policy
cutoff, 304 handling, exact rollback, and recovery from a corrupt active artifact.

Replay regressions verify canonical set/timestamp equivalence and that later
source-buffer changes cannot alter a prepared revision. Evaluation tests include
cancellation during an expensive query. Concurrency stress tests require successful
valid decisions and allow only explicit deadline errors with empty decisions when
race instrumentation exceeds the unchanged 50 ms budget. All other errors fail.

## Performance

Command: `go test -run '^$' -bench=. -benchmem -benchtime=2s ./internal/policy/...`.
The fixture mix cycles through all 27 checked-in baseline scenarios. Values are
local measurements, not hosted CI or production SLO claims.

| Benchmark | Mean | p50 | p95 | p99 | Bytes/op | Allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Cold compile | 1.636 ms | — | — | — | 650,516 | 11,683 |
| Cached valid allow | 0.602 ms | 0.543 ms | 1.031 ms | 1.330 ms | 216,977 | 5,165 |
| Complete baseline fixture mix | 0.585 ms | 0.649 ms | 1.217 ms | 1.402 ms | 176,015 | 4,208 |

The fixture p95 is below the phase target of 5 ms. Policy evaluation includes
canonical validation, composition and explanation construction; it does not execute
network or filesystem operations.

## Scope and remaining phases

- Signing identity is validated metadata; inspection explicitly reports signatures
  unverified. Remote trust/key distribution is Phase 5; HTTPS/OCI require trusted
  checksum pins in this phase.
- Engine and Manager health/status are Go APIs and CLI output. The HTTP service,
  authenticated principal derivation, authoritative fact resolvers, permits,
  obligation execution and protected downstream operations remain Phase 3.
- Baseline identity/destination records are example registrations. They do not
  authenticate arbitrary caller-supplied IDs. The future service must supply trusted
  canonical identity and context.
- State directories have one manager/writer. Archives are retained for rollback;
  operators must manage disk retention. The engine caps revision caching at 128
  and fails closed when full; it does not evict a revision used by a reader.
- OPA's synchronous AST compiler has bounded input and between-stage context
  checks; in-process resource isolation is not a production sandbox guarantee.
- This is local verification. Hosted CI, independent security review and production
  release assurance remain separate work.
