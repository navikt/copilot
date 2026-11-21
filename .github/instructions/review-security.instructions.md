---
applyTo: "apps/fleet-gateway/**,apps/mcp-*/**,cli/nav-pilot/internal/**/*redact*,apps/copilot-api/**/*.go,apps/copilot-survey/**/*.go"
excludeAgent: "cloud-agent"
---

# Review: security boundaries

Review-only guidance. Flag these in review; do not restate linter findings.

- **Fail closed**: Missing config, parse errors, unknown claims and upstream failures deny. Flag any `default:` or error path that continues or allows.
- **Texas token introspection in MCP servers**: `401` inactive token; `403` for app token, `NAVident` or `azp`; `502` when introspection fails; `503` when auth config is missing. Other `502`/`503` uses (session store, GitHub) are legitimate.
- **`azp`**: Compare against the configured client ID, never against a display name, a prefix or a `contains` check.
- **No secrets or personal data in logs or metric labels**: Tokens, `Authorization`, emails, raw request bodies and participant hashes. MCP servers must not log `NAVident` or other personal data. Exception: the gateway's AuthZEN audit line records `subject.id` (`NAVident`) by design (`SECURITY.md`, Audit). High-cardinality labels (user, token, URL) are also a defect.
- **Body size**: Enforce the limit before buffering (`http.MaxBytesReader`, `Content-Length` check plus a capped read), not after `io.ReadAll`.
- **JSON-RPC envelope**: Validate `jsonrpc`, `method`, and `id` type. Return `-32600` for an invalid request, `-32020` for an `Mcp-Method`/`Mcp-Name` header that does not match the body, and `-32602` for a tool outside the route allowlist (`apps/fleet-gateway/proxy.go`). Batch and notification shapes must not bypass checks.
- **URLs and hosts**: Compare the parsed hostname (`url.Parse`, `Hostname()`), never `strings.Contains` or `HasPrefix` on the raw URL. Check scheme and port.
- **Write tools**: Confirmation must be enforced on the server, not trusted from a client flag or a prompt. Reference: `apps/mcp-dolly/confirm.go` (single-use HMAC confirmation token for `create_order`).
- **Revocation and lifetime**: Sessions and refresh tokens must stop at revocation and expiry, and a cancelled request must release its slot.
- **Old data**: Data written by an earlier version must also fail closed when it lacks a new field.
