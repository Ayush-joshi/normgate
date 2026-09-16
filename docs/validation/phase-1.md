# Phase 1 validation record

Date: 2026-09-15. Status: **foundation implementation complete; local gate passed**.
At this Phase 1 checkpoint, Phase 2 had not started. No policy decisions or downstream operations were executed
by the Phase 1 binary.

## Delivered

- Go module/workspace, repository boundaries, CLI, formatting and CI configuration.
- Fourteen versioned Draft 2020-12 schemas: the twelve planned public contracts,
  plus configuration and public errors. Ninety-nine contract fixtures cover valid,
  invalid and version/extension compatibility cases.
- Deterministically generated Go structs, Python TypedDict models, TypeScript
  types and OpenAPI 3.1 components. Go performs authoritative runtime validation;
  generated language types alone are not validators.
- Go/Python/TypeScript canonical JSON and SHA-256 implementations with 39 shared
  golden vectors, strict Unicode/number parsing, timestamp normalization, explicit
  set paths and explicit transport omissions.
- Strict JSON/YAML configuration, one-pass environment and file references,
  standalone validation, redacted diagnostics and safe correlated public errors.
- Initial threat model, trust boundaries, API conventions, five architecture
  decisions, Apache-2.0 license and development instructions.

## Validation performed

`make verify` passed on a fresh source export of all tracked and untracked project
files, excluding Git metadata, dependency directories and build artifacts. Locked
npm dependencies were installed afresh. Go used its content-addressed module/build
caches. The tested implementation, schemas, generators and fixtures were compared
byte-for-byte with the working source after the run.

Toolchain: Go 1.26.8, Python 3.12.14, Node.js 24.4.0, TypeScript 5.9.3;
macOS ARM64. `govulncheck` is pinned to v1.8.0. CI is configured for Linux with
the same Go version, Python 3.12 and Node 24.

| Check | Result |
| --- | --- |
| Generated artifacts and explicit drift injection | Pass; modifying a generated file is rejected |
| Go formatting/vet and TypeScript compilation | Pass |
| Unit, contract and architecture tests | Pass |
| Cross-language canonical bytes and digests | 39/39 vectors pass in all three languages |
| Race detector | Pass |
| Contract statement coverage | 93.62% (required 90%) |
| Normalization statement coverage | 94.57% (required 90%) |
| Configuration statement coverage | 95.00% (required 90%) |
| Repository statement coverage | 94.55% (required 85%); no production package excluded |
| Fuzz smoke | Three targets, 30 seconds each; pass |
| Dependency license review gate | Nine reviewed dependency licenses verified |
| Go vulnerability scan | No vulnerabilities found |
| npm audit | Zero vulnerabilities |
| Binary build and CLI smoke | `version` and example configuration validation pass |

The first vulnerability scan rejected `golang.org/x/text` v0.34.0 for
[GO-2026-5970](https://pkg.go.dev/vuln/GO-2026-5970). It was upgraded to v0.39.0;
the dependency graph and unchanged license hashes were reviewed again. Invalid
UTF-8 rejection has a dedicated regression test. Additional regressions address
timestamp permissiveness, standalone config validation, allow/transform ambiguity
and Go's stream-safe Unicode normalization behavior.

## Scope and remaining administration

- This record captures local verification, not a hosted CI result. Check
  [GitHub Actions](https://github.com/Ayush-joshi/normgate/actions/workflows/verify.yml)
  for the latest run. Branch protection and private vulnerability reporting have
  not been verified or changed; assign a confirmed CODEOWNERS team and configure
  required checks as repository administration work.
- `api/openapi` contains components; service routes arrive in Phase 3. SDK HTTP
  clients and the console arrive in Phase 4. Empty future directories preserve
  architectural boundaries and do not claim those components are implemented.
- Permit schemas describe binding fields but do not verify signatures, freshness,
  identity consistency or replay. Phase 3 must implement these checks and freeze
  complete event/permit normalization profiles before issuing permits.
- Independent security review, performance SLOs, production deployment and release
  assurance remain later work. Hosted Linux verification is tracked by the CI
  workflow separately from this local validation record.

Next: Phase 2's test-first `PolicyEngine` boundary, deterministic OPA evaluation
and policy lifecycle, following `plan/phases/PHASE_2_POLICY_ENGINE.md`.
