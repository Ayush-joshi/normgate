# NormGate

NormGate is an open-source, self-hostable policy enforcement and evidence runtime for AI applications and agents.

It is designed to sit between an AI application and the models, tools, retrieval systems, and memory stores that application uses. NormGate normalizes each operation, evaluates deterministic policy, enforces the resulting decision, and records a verifiable receipt.

## Project goals

- Keep the core independent of any industry, regulation, model provider, or agent framework.
- Enforce policy outside the language model at the point where an operation executes.
- Support model requests, tool calls, retrieval, responses, and memory operations.
- Bind authorization to the exact identity, operation, arguments, destination, data classifications, and policy revision.
- Produce replayable, privacy-conscious evidence for every decision.
- Add domain behavior later through independently installable and signed extensions.

## Intended architecture

```text
AI application or agent
        |
        v
NormGate enforcement adapter
        |
        v
Canonical operation -> policy decision -> obligations -> signed permit
        |                                      |
        v                                      v
Model, tool, retrieval, or memory         decision receipt
```

The language model may help classify content or intent, but it never grants authority. The deterministic policy runtime returns one of four decisions:

- `allow`
- `deny`
- `transform`
- `require_approval`

## Planned interfaces

- Headless HTTP decision and enforcement service
- OpenAI-compatible inference gateway
- Generic tool and MCP gateway
- Retrieval and memory enforcement APIs
- Python and TypeScript SDKs
- CLI for policy development, testing, simulation, replay, and audit verification
- Web console for policies, decisions, approvals, evidence, and operations
- OpenTelemetry export
- Signed extension and policy-package distribution

## Status

**Phases 1 and 2 are implemented and locally verified. Phase 3 is next.**

| Phase | Scope | Status |
| --- | --- | --- |
| 1 | Contracts, canonical hashing, configuration and repository foundation | Implemented; local verification passed |
| 2 | Deterministic policy engine and policy lifecycle | Implemented; local verification passed |
| 3 | Enforcement runtime, gateways, permits and receipts | Not started |
| 4 | Control plane, SDK clients and operator console | Not started |
| 5 | Extensions and domain packages | Not started |
| 6 | Production hardening and release | Not started |

### Available now

- Fourteen versioned JSON Schemas, with generated Go structs, Python TypedDict
  models, TypeScript types and OpenAPI 3.1 components.
- Canonical JSON and SHA-256 implementations in all three languages, tested
  against 39 shared vectors and 99 contract fixtures.
- Embedded deterministic OPA engine, layered policy composition, reproducible bundles,
  authoring CLI, atomic activation/recovery/rollback and filesystem/HTTPS/OCI sources.
- Generic baseline policy with 27 exact scenarios and rule/obligation explanations.
- Strict JSON/YAML configuration with environment and file-secret references,
  redacted diagnostics and the `normgate config validate` command.
- Architecture checks, security records and a verification workflow covering
  formatting, tests, race detection, fuzzing, coverage, licenses and vulnerabilities.

The Phase 2 local gate records **92.59% overall Go statement coverage**, with both
policy packages above 93%. See the [validation record](docs/validation/phase-2.md)
for the environment and results, and [GitHub Actions](https://github.com/Ayush-joshi/normgate/actions/workflows/verify.yml)
for hosted run status.

The binary validates configuration and evaluates policies locally through
`normgate policy explain`. It does not yet serve HTTP decisions or execute protected
operations. See [policy development and lifecycle](docs/policy.md) and the
[Phase 2 validation record](docs/validation/phase-2.md). Generated SDK types are
transport models, not HTTP clients or runtime validators. No production or compliance assurance is available.

### Next milestone

Phase 3 starts with one runnable protected tool-call demo: a sample application
calls a tool through NormGate, policy allows, blocks, or removes fields, and a
receipt records the actual execution outcome. The demo must also prove that
changed or reused permits and direct downstream access cannot bypass enforcement.
Approval-required calls stay blocked until trusted approval verification exists;
the approval workflow and console arrive in Phase 4. See the
[Phase 3 acceptance scenarios](plan/phases/PHASE_3_ENFORCEMENT_RUNTIME.md#first-milestone-one-protected-tool-call).

## Develop locally

Use Go 1.26.8+, Python 3.11+ and Node.js 22.18+ (24 recommended).

```sh
make setup
make verify
./bin/normgate version
./bin/normgate policy test policies/baseline
NORMGATE_API_KEY=local-development-key ./bin/normgate config validate --file config.example.yaml
```

See [development instructions](docs/development.md) for configuration, generated
artifacts, test gates and tool overrides.

## Implementation plan

The complete six-phase build plan is documented in [plan/IMPLEMENTATION_ROADMAP.md](plan/IMPLEMENTATION_ROADMAP.md). Each phase also has an implementation-level agent guide:

1. [Foundation](plan/phases/PHASE_1_FOUNDATION.md)
2. [Policy engine](plan/phases/PHASE_2_POLICY_ENGINE.md)
3. [Enforcement runtime](plan/phases/PHASE_3_ENFORCEMENT_RUNTIME.md)
4. [Control plane](plan/phases/PHASE_4_CONTROL_PLANE.md)
5. [Extensions and domain packages](plan/phases/PHASE_5_EXTENSIONS_AND_DOMAIN_PACKS.md)
6. [Production release](plan/phases/PHASE_6_PRODUCTION_RELEASE.md)

## License

Licensed under the [Apache License 2.0](LICENSE).
