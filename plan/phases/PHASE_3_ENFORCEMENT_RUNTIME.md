# Phase 3: Enforcement Data Plane, Gateways, and Receipts

## Outcome

At the end of this phase, a real request can enter a NormGate gateway, become a canonical event, receive a deterministic decision, satisfy required transformations, execute against a downstream service only with a valid permit, have its response checked, and produce a verifiable receipt.

## Entry conditions

- Phase 2 policy engine and baseline policy pass all conformance tests.
- Canonical hashing and policy revisions are stable.
- Policy evaluation meets its latency target.

## Mandatory TDD workflow

Develop each vertical runtime slice through an end-to-end failing test before implementing its internal layers. Begin with fake downstream services and fixed policy fixtures. A slice is complete only when tests prove both the intended path and bypass attempts.

For security-sensitive code, a successful-path test is insufficient. Each permit field and obligation must have a negative test showing that mutation or omission prevents execution.

## First milestone: one protected tool call

Deliver one runnable local workflow before expanding to inference, MCP, retrieval,
and memory. This milestone proves the runtime path; it does not replace the full
Phase 3 scope or completion gates.

Use a small sample application that proposes a generic tool call, a synthetic
payload, and an instrumented fake downstream service. No external model account
is required. Route the request through authentication, canonical normalization,
policy evaluation, required transformations, signed permit verification, execution,
response checking, and SQLite receipts. Implement the portions of workstreams 1,
2, 3, 5, and 7 needed for this path together rather than leaving receipts until
after every gateway is built.

The demonstration must include reproducible commands, example policy and inputs,
expected decisions, downstream call counts and received payloads, and receipt
inspection and verification. Keep raw synthetic payload inspection in the test
downstream; runtime receipts and telemetry retain their minimized defaults.

### Acceptance scenarios

| Scenario | Required observable result |
| --- | --- |
| Authorized call | The downstream executes once and receives exactly the authorized payload; the receipt records the actual outcome. |
| Policy denial or cross-tenant access | The caller receives a structured denial and the downstream receives zero calls. |
| Required field removal | The downstream receives only the transformed payload; the permit binds that payload. A failed transformation prevents execution. |
| Changed arguments, destination, or identity after authorization | Final-point verification rejects the operation and the downstream receives zero calls. |
| Reused or expired permit | No additional downstream execution occurs, including under concurrent reuse. |
| Approval required | Preserve the `require_approval` decision, return a structured blocked result, and make zero downstream calls. A caller-supplied approval flag cannot authorize execution. |
| Direct downstream access | The sample application's identity cannot bypass the gateway using the deployment's network and credential configuration. |
| Denied response | A downstream result that fails response policy is withheld from the caller; evidence distinguishes execution from response delivery. |
| Missing authoritative context or unavailable required dependency | Fail closed before execution; no fabricated identity, labels, or successful outcome. |
| Downstream failure or interrupted execution | Evidence distinguishes the policy decision from execution failure or an unknown outcome; restart preserves receipt integrity and replay prevention. |

### Trust and approval boundaries

- Derive principal and tenant from authenticated context. Resolve destinations and
  policy-required classifications or external facts from configured trusted sources;
  caller or model assertions alone do not establish authority. Synthetic fixtures
  must make their trusted source explicit.
- A decision endpoint alone does not protect a tool. Document which credentials
  and network controls force execution through the enforcement point, and test a
  direct-access attempt from the application's actual deployment identity.
- Phase 3 enforces approval requirements. The approval queue, reviewer workflow,
  and resumption experience remain Phase 4. Until trusted approval verification is
  available, approval-required operations stay blocked without automatic retries
  or a temporary bypass. Phase 4 must reevaluate approved operations immediately
  before execution.
- Receipts provide evidence of observed decisions and outcomes. Hash-chain
  verification needs a trusted checkpoint; replay needs separately governed input
  snapshots, as specified in [ADR 0004](../../docs/architecture/decisions/0004-minimized-receipts.md).

## Workstream 1: Service API and authenticated context

### Tests first

- HTTP contract tests for every endpoint and error envelope
- Authentication tests for missing, invalid, expired, and wrongly scoped credentials
- Tests proving caller headers cannot override authenticated tenant or principal
- Request-size, timeout, cancellation, idempotency, and content-type tests
- Readiness behavior when policy or storage is unavailable

### Implementation

Implement:

```text
POST /v1/decisions
POST /v1/permits/verify
POST /v1/enforcements
POST /v1/responses/evaluate
GET  /v1/policies/status
GET  /v1/health/live
GET  /v1/health/ready
```

Use middleware in this order:

```text
request ID -> limits -> authentication -> tenant context -> decoding
-> normalization -> policy deadline -> response encoding -> telemetry
```

Keep transport DTOs separate from canonical contract types. Log only request IDs, digests, and safe metadata.

## Workstream 2: Permit issuance and verification

### Tests first

Build a mutation matrix that changes one signed property at a time:

- Principal
- Tenant
- Operation kind
- Target
- Arguments
- Data references and classifications
- Purpose
- Destination
- Policy revision
- External-fact digest
- Expiration
- Nonce

Every mutation must fail verification. Add replay, unknown-key, rotated-key, malformed-signature, clock-skew, and concurrent-use tests.

### Implementation

- Use Ed25519 keys behind a `Signer` and `Verifier` interface.
- Serialize signed claims with Phase 1 canonicalization.
- Make permits short-lived and valid for one operation.
- Store nonce consumption atomically before downstream execution.
- Support overlapping verification keys during rotation.
- Return stable failure reasons without revealing signature internals.
- Bind transformed-payload digest when transformation obligations apply.

## Workstream 3: Obligation execution

### Tests first

For every obligation, test valid application, missing target, incompatible value type, overlapping paths, order independence, output-size growth, and prohibited partial completion.

Add property tests proving:

- Transformation never introduces a previously absent classified field.
- Retain-only cannot increase the field set.
- Record and byte limits are monotonic.
- Reapplying an idempotent transformation produces identical output.

### Implementation

Implement an obligation registry inside the core for:

- `remove_paths`
- `retain_paths`
- `tokenize_values`
- `mask_values`
- `limit_records`
- `limit_bytes`
- `restrict_destination`
- `require_confirmation`
- `require_approval`
- `prohibit_retention`
- `evaluate_response`

Execute transformations in a documented deterministic order. If any required obligation cannot be enforced completely, deny before forwarding.

## Workstream 4: Inference gateway

### Tests first

Create a fake OpenAI-compatible server and write end-to-end tests for:

- Non-streaming and streaming requests
- Upstream errors and malformed responses
- Cancellation and client disconnect
- Retryable and non-retryable failures
- Input transformation before forwarding
- Output denial before delivery
- Streaming output stopped at the first denied buffered segment
- Model alias resolution and destination mismatch
- No raw body in logs or telemetry

### Implementation

- Implement the minimum compatible endpoints required by the project contract rather than cloning an entire provider API.
- Normalize messages, model, tools, response mode, and destination metadata into an event.
- Evaluate and transform before opening an upstream connection.
- Buffer streaming output in configurable bounded segments for response evaluation.
- Disable retries after any response bytes have reached the caller.
- Record upstream request ID, status, usage metadata, and digest without storing content by default.

## Workstream 5: Tool and MCP gateways

### Tests first

- Tool discovery filtering
- Unknown tool denial
- Input-schema mismatch
- Argument transformation
- Permit verification immediately before invocation
- Tool result classification and response evaluation
- MCP server identity mismatch
- Timeout, cancellation, duplicate invocation, and downstream failure
- Attempts to call the downstream server directly in the deployment test network

### Implementation

- Normalize tools as destination-scoped resources.
- Filter discovery results according to principal and purpose.
- Validate arguments against the advertised schema before policy evaluation.
- Bind the validated normalized arguments to the permit.
- Return structured denial to the agent without presenting it as a tool transport failure.
- Include downstream outcome in the enforcement receipt.

## Workstream 6: Retrieval and memory APIs

### Tests first

Create fake stores and tests for read, write, update, and delete operations. Cover tenant mismatch, maximum result count, classified metadata, transformed results, unregistered store, and failure after authorization but before completion.

### Implementation

Provide canonical authorize-and-enforce APIs that storage integrations can call even when no gateway protocol is available. Define operation-specific request types as compositions of the canonical model rather than new policy inputs.

## Workstream 7: Receipts and telemetry

### Tests first

- Hash-chain verification and tamper detection
- Idempotent receipt insertion
- Crash between decision and enforcement result
- Recovery of incomplete receipts
- Payload minimization assertions
- Trace/decision correlation
- Telemetry exporter outage without enforcement bypass

### Implementation

- Store decision and enforcement receipts separately but link them by decision ID.
- Chain receipts per tenant and include previous hash, policy revision, event digest, permit ID, decision, obligations, enforcement result, and timestamps.
- Mark interrupted executions as `unknown`, never `succeeded`.
- Implement `normgate audit verify` and `normgate audit export`.
- Export OpenTelemetry without including classified payload values.

## Test and coverage gates

- `internal/enforcement`, `internal/receipt`, and permit code: at least 95% Go statement coverage
- Remaining new runtime packages: at least 85%
- Every permit claim has a mutation rejection test.
- Every obligation has positive, negative, conflict, and idempotency tests where applicable.
- Full end-to-end tests run with fake downstreams on every pull request.
- Race tests cover nonce consumption, streaming cancellation, policy reload, and receipt writes.
- Fuzz targets cover public request decoding, streaming frames, tool schemas, permits, and receipt parsing.

## Required validation

```bash
make test-unit
make test-contract
make test-integration
make test-e2e
make test-race
make test-fuzz-smoke
make coverage
normgate audit verify --store test-output/receipts.db
make verify
```

## Suggested pull-request sequence

Keep the first three pull requests focused on the first milestone, with executable
tests as each part lands. Complete its acceptance scenarios before expanding the
protocol and operation coverage.

1. Tool-call acceptance harness, service skeleton, authentication, and trusted context
2. Signed permits, final-point verification, replay prevention, and SQLite evidence for allow/deny tool execution
3. Field removal, response checking, blocked approval behavior, bypass/failure/restart tests, and reproducible demo instructions
4. Remaining permit rotation and obligation coverage, audit tooling, and telemetry requirements
5. Complete generic tool gateway and MCP discovery/invocation
6. Inference gateway, including streaming and cancellation
7. Retrieval and memory enforcement APIs, then full cross-gateway verification

## Completion checklist

- [ ] The first protected tool-call demo is reproducible and every acceptance scenario passes.
- [ ] Trusted context and deployment controls prevent caller impersonation and direct downstream bypass.
- [ ] Approval-required operations remain blocked without trusted approval verification; no placeholder bypass exists.
- [ ] No protected downstream path executes without final-point permit verification.
- [ ] Every permit field is cryptographically bound and mutation-tested.
- [ ] Required transformations are complete or the operation is denied.
- [ ] Streaming denial prevents disallowed buffered content from reaching callers.
- [ ] Interrupted operations are recorded accurately.
- [ ] Logs and telemetry exclude raw classified payloads by default.
- [ ] Runtime coverage and full repository verification pass.
