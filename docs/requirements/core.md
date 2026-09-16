# Core requirements

Phase 1 requirements are executable contracts. Later phase requirements will be added before implementation.

| ID | Requirement | Evidence |
| --- | --- | --- |
| NG-F001 | Required repository boundaries exist; internal code cannot import extensions, domain packages, SDKs, web, or command packages. | `test/architecture` |
| NG-F002 | Versioned Draft 2020-12 schemas reject unknown fields, missing identity, unsupported major versions, invalid operations and inconsistent decisions. | `test/contract`, contract fixture corpus |
| NG-F003 | Go, Python and TypeScript contract artifacts derive deterministically from schemas; stale output fails verification. | `scripts/generate.py --check` |
| NG-F004 | Canonical JSON is deterministic across languages, preserves absent/null distinctions, rejects ambiguous input and hashes exact authoritative data. | normalization golden vectors and fuzz tests |
| NG-F005 | Configuration rejects unknown keys, unsafe fail-open settings and invalid references; flags override file values; diagnostics never expose secrets. | `internal/config` |
| NG-F006 | Public failures use stable reason codes with transport-independent errors and correlation IDs. | `internal/api`, `internal/errors` |
| NG-F007 | Every core contract stays domain neutral; metadata extensions require namespaced keys. | `test/architecture`, contract fixtures |
| NG-F008 | Build verification enforces generation, formatting, static analysis, race, coverage, fuzz and vulnerability checks. | Makefile, root GitHub workflow |
