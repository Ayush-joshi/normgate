# Trust boundaries

| Boundary | Untrusted input | Trusted component | Rule |
| --- | --- | --- | --- |
| Caller → service | identities, operations, labels, context | authenticated service context | Caller claims do not authenticate a principal; derive identity in Phase 3. |
| Model/classifier → policy | content and proposed labels | deterministic policy engine | Facts never grant authority. Deterministic embedded OPA engine implemented. |
| Policy source → activation | policy artifact, manifest, revisions | verifier and atomic policy store | Verify checksums, compile and test before atomic activation. Implemented; signature trust remains Phase 5. |
| Fact source → policy | values, revisions, freshness | configured authoritative resolver | Bind version and digest; unavailable required facts deny. Phases 2–3. |
| Decision → executor | permit and potentially changed arguments | final enforcement adapter | Verify exact binding immediately before downstream execution. Phase 3. |
| Service → storage/telemetry | decisions, receipts, diagnostics | minimized evidence writer | References and digests by default; no raw payloads. Phase 3. |
| Administrator → configuration | JSON/YAML, reference syntax | strict local configuration loader | Validate without network access; explicit one-pass secret resolution. Implemented. |
| Source schemas → consumers | schema changes and generated code | deterministic generator and CI | JSON Schemas own shape; drift is a build failure. Implemented. |
| Extension → core | plugin results and permissions | isolated extension host | Core imports no domain/extension code. Import check implemented; host begins Phase 5. |

Repository boundaries: command entry points delegate to `internal/cli`. HTTP status
codes live in `internal/api`; core errors contain stable categories. Contracts use
only embedded schemas and disable remote loading. Core code cannot import extension,
domain, SDK, web or command packages. External library licenses and versions are
pinned and checked; that is not a guarantee that dependencies have no defects.
