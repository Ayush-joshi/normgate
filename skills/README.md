# NormGate Agent Skills Catalog

This directory contains specialized, platform-agnostic agent skills designed to guide any autonomous agent or human engineer working on the **NormGate** repository.

Every skill follows the open standard:
- Self-contained directory containing a `SKILL.md` file.
- Standard YAML frontmatter (`name`, `description`).
- Deterministic, step-by-step procedures, runnable shell commands, and verification checklists.
- Zero proprietary dependencies: consumable by **Antigravity**, **Claude Code**, **Cursor**, **Windsurf**, **Cline**, **GitHub Copilot**, or any autonomous agent framework.

---

## Agent Routing Matrix

When an agent receives a prompt or task, it must consult this routing matrix and load the corresponding skill before planning or modifying code:

| If your task involves... | Load this Skill | Path |
| :--- | :--- | :--- |
| **Running tests, CI checks, or diagnosing failures** (`make verify`, coverage, races, fuzzing, vuln) | `normgate-verify` | [normgate-verify/SKILL.md](normgate-verify/SKILL.md) |
| **Modifying schemas (`schemas/v1/`), code generation, or 3-language vector parity** | `normgate-contracts` | [normgate-contracts/SKILL.md](normgate-contracts/SKILL.md) |
| **Authoring/debugging Rego v1, deterministic engine, policy bundles, CLI, mutation testing** | `normgate-policy` | [normgate-policy/SKILL.md](normgate-policy/SKILL.md) |
| **Building gateways (Inference, Tools/MCP, Storage), permits, obligations, runtime enforcement** | `normgate-enforcement` | [normgate-enforcement/SKILL.md](normgate-enforcement/SKILL.md) |
| **Handling receipts, SQLite audit store, hash-chains, tamper detection, or payload minimization** | `normgate-receipts` | [normgate-receipts/SKILL.md](normgate-receipts/SKILL.md) |
| **Checking import boundaries, package isolation, trust boundaries, credentials, or ADR compliance** | `normgate-architecture` | [normgate-architecture/SKILL.md](normgate-architecture/SKILL.md) |
| **Writing table-driven tests, fake downstreams, negative/bypass testing, benchmarks, or fuzzers** | `normgate-tdd` | [normgate-tdd/SKILL.md](normgate-tdd/SKILL.md) |
| **Authoring ADRs, Phase Validation Records, API/spec sync, and technical documentation** | `normgate-docs` | [normgate-docs/SKILL.md](normgate-docs/SKILL.md) |

---

## Execution Protocol for Autonomous AI Agents

To ensure reliable, defect-free implementation, all working agents follow this deterministic 6-step protocol:

```text
┌─────────────────────────────────────────────────────────────┐
│ 1. ROUTE & GROUND                                           │
│    - Check the Routing Matrix above.                        │
│    - Read the targeted SKILL.md before proposing any edit.  │
│    - Review the "DO THIS / NEVER DO THIS" table.            │
└──────────────────────────────┬──────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. MANDATORY TDD (TEST FIRST)                               │
│    - Write a failing Go table test or YAML policy scenario. │
│    - Run the test: verify it fails for expected reasons.   │
│    - Never write implementation code before the test.       │
└──────────────────────────────┬──────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. MINIMAL IMPLEMENTATION                                   │
│    - Write minimal code to pass the test.                   │
│    - Strictly observe import boundaries (internal/ rules).  │
│    - Maintain determinism: no wall clock, no network in     │
│      policy evaluation.                                     │
│    - If schemas changed: edit schemas/v1/, NEVER .gen files.│
└──────────────────────────────┬──────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. INNER-LOOP QUALITY CHECKS                                │
│    - Run: make fmt                                          │
│    - Run: make check-generated                              │
│    - Run: make lint                                         │
│    - Run: make test-unit PKG=./internal/<target>/...        │
└──────────────────────────────┬──────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────┐
│ 5. FULL GATE VERIFICATION                                   │
│    - Run: make verify                                       │
│    - All 12 sequential checks MUST return exit code 0.      │
│    - Zero silently skipped steps.                           │
└──────────────────────────────┬──────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────┐
│ 6. PRE-COMPLETION CHECKLIST & DOCS                          │
│    - Check off the binary checklist in the skill.           │
│    - Update ADRs or documentation if architecture changed.  │
│    - Conclude task with verifiable proof of passed checks.  │
└─────────────────────────────────────────────────────────────┘
```

---

## Non-Negotiable Engineering Rules for Agents

Every agent operating in this repository must uphold these non-negotiable rules:

1. **Mandatory TDD**: Always write a failing test first (Go table test or YAML policy scenario) before implementing any rule, feature, or bugfix.
2. **Never Skip the Verification Gate**: Before completing any task, run `make verify`. It runs 12 sequential checks and must remain 100% green.
3. **Strict Import Boundaries**: Core packages under `internal/` must NEVER import `cmd`, `sdk`, `web`, or `extensions`. The OPA SDK must NEVER be imported outside `internal/policy/opa/`.
4. **Zero Domain Terms in Core**: Schemas under `schemas/v1/*.json` must remain completely generic. Never introduce industry-specific vocabulary (e.g. `patient`, `banking`, `hipaa`, `diagnosis`).
5. **Deterministic Policy Evaluation**: Never use wall-clock time, system clocks, network calls, or non-deterministic built-ins in policy evaluation. External values enter as `ExternalFact` records.
6. **Payload Minimization**: Never log or store raw arguments, prompts, or sensitive payloads in receipts or telemetry; store canonical SHA-256 digests and references instead.
7. **Multi-Language Parity**: Any changes affecting Canonical JSON (profile v1) or SHA-256 hashing must produce byte-identical results across Go, Python, and TypeScript.
8. **No Editing Generated Files**: Never edit `*.gen.go`, `__init__.py`, `types/index.ts`, or `openapi.gen.json` directly. Edit `schemas/v1/*.schema.json` and run `make generate`.

---

## Directory Organization & Discoverability

- **Primary Directory**: `skills/<skill-name>/SKILL.md` (root-level, easily discovered by any agent framework).
- **Workspace Symlink**: `.agents/skills` points to `../skills` to provide zero-config discovery for tools like Google Antigravity that inspect `.agents/`.
