# API conventions for Phase 3

Phase 1 ships OpenAPI components only; it does not publish service routes.

- HTTP route major versions use `/v1`. Every public object includes `schema_version`.
  JSON is required; unknown object fields are rejected except namespaced extensions.
- Public errors follow `ErrorEnvelope`: `schema_version`, `request_id`, `code`,
  `message`. Codes use `ng.validation`, `ng.authentication`, `ng.policy`,
  `ng.enforcement`, `ng.storage`, `ng.upstream`, or `ng.internal` namespaces.
  Exception strings and payload values must not enter public messages.
- Correlation IDs are 1–256 ASCII letters, digits, `.`, `_`, `:`, or `-`.
  Phase 3 generates a fresh ID when none is supplied and echoes it in responses.
  The foundation error mapper uses `uncorrelated` for invalid internal inputs.
  IDs are observability context, never an authentication mechanism.
- Cursor pagination uses opaque, tenant-bound continuation tokens, a default
  limit of 50 and a maximum of 200. Collection responses include `items` and
  an optional `next_cursor`. Concrete collection schemas begin in Phase 4.
- Mutation endpoints accepting `Idempotency-Key` scope it to authenticated tenant,
  principal and route. A repeated key with a changed canonical request digest
  returns a conflict; a matching key returns the stored outcome without executing
  downstream again. Expiration and durability must be published with each endpoint.
- Request `Content-Type` and response `Accept` negotiate `application/json` within
  the route major. Unsupported majors or media types fail explicitly. Minor
  version compatibility follows ADR 0005; no guessing or implicit downgrade.

These are binding implementation decisions for later phases, not claims of HTTP
features already available in the foundation CLI.
