# Development

Run commands from the `normgate/` Git repository. GitHub workflows, Dependabot
and CODEOWNERS live in its root `.github/`.

Use Go 1.26.8 or later supported Go (module language baseline 1.26; required by
OPA), Python 3.11+ and Node.js 22.18+ (24 used in CI). The pinned vulnerability scanner requires Go 1.26.

```sh
make setup
make generate
make verify
make build
./bin/normgate version
NORMGATE_API_KEY=local-development-key ./bin/normgate config validate --file config.example.yaml
```

`make verify` checks generated artifacts, formatting, vet, TypeScript compilation,
architecture, unit/contract tests, three-language canonical vectors, race detection,
coverage, baseline policy scenarios, composition mutation tests, 30-second fuzz
campaigns per target, dependency licenses, vulnerabilities and binary compilation. Scheduled CI runs longer fuzz campaigns. No check is
silently skipped when a tool or network service is unavailable.

`GO`, `GOFMT`, `PYTHON`, `NODE`, `NPM` and `GOVULNCHECK` can select local tools.
`make setup` downloads checksum-pinned modules, locked npm packages and the pinned
scanner into `.tools/bin`. Runtime config validation itself performs no network access.
`make fmt` formats Go source; `make check-generated` never rewrites files and also
detects stale files in an uncommitted checkout. Regenerate after changing schemas.

Configuration JSON/YAML rejects unknown fields, duplicate keys, aliases, multiple
documents, custom tags, unsafe timeouts and fail-open settings. Explicit CLI flags
override file scalars before reference resolution. `${ENV:NAME}` and
`${FILE:/absolute/path}` must occupy the whole scalar and expand once. Secret files
must be regular, at most 64 KiB; one final LF or CRLF is removed. A reference value
is data and is not recursively evaluated. Use the explicit `APIKey()` accessor only
where credentials are needed. Standard formatting and JSON diagnostics omit it.
Policy URLs support local absolute `file:` sources or `https:` without embedded
credentials, query strings or fragments. Validation does not fetch policies.

When changing dependencies, inspect their licenses and update
`scripts/licenses.lock.json` intentionally. Changes to a license file, module
version, npm version or replacement module fail review checks. The full dependency
graph includes Apache-2.0, MIT, MIT-0, ISC, BSD-2-Clause, BSD-3-Clause and MPL-2.0
licenses, including combined license notices. Preserve the applicable licenses and
notices in distributable bundles; the lock records each reviewed file and hash.

Phase work follows the active guide under `plan/phases/`: record a requirement, add a
failing behavioral test, implement, run the affected tests, then the phase gate.
See [policy development](policy.md) for Phase 2 commands, bundle format, source
interfaces and deterministic evaluation limits. Protected-operation execution
remains Phase 3 work. Consult [agent skills](../skills/README.md) for deterministic
workflows covering verification, schemas, policies, enforcement, receipts, and testing.

Hosting administration: assign the confirmed CODEOWNERS team, enable private
vulnerability reporting and require the verification job in branch protection.
These remote settings have not been inspected or changed by the local implementation.
