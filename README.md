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

NormGate is in the planning stage. No production implementation or compliance assurance is available yet.

The complete six-phase build plan is documented in [plan/IMPLEMENTATION_ROADMAP.md](plan/IMPLEMENTATION_ROADMAP.md). Each phase also has an implementation-level agent guide:

1. [Foundation](plan/phases/PHASE_1_FOUNDATION.md)
2. [Policy engine](plan/phases/PHASE_2_POLICY_ENGINE.md)
3. [Enforcement runtime](plan/phases/PHASE_3_ENFORCEMENT_RUNTIME.md)
4. [Control plane](plan/phases/PHASE_4_CONTROL_PLANE.md)
5. [Extensions and domain packages](plan/phases/PHASE_5_EXTENSIONS_AND_DOMAIN_PACKS.md)
6. [Production release](plan/phases/PHASE_6_PRODUCTION_RELEASE.md)

## License

The project is intended to use the Apache License 2.0. A `LICENSE` file will be added before the first distributable release.
