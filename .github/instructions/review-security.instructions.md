---
applyTo: "apps/fleet-gateway/**,apps/mcp-*/**,cli/nav-pilot/internal/**/*redact*,apps/copilot-api/**/*.go,apps/copilot-survey/**/*.go"
excludeAgent: "cloud-agent"
---

# Review: security boundaries

Review-only guidance. Flag these in review; do not restate linter findings.

- **Fail closed**: Missing config, parse errors, unknown claims and upstream failures deny. Flag any `default:` or error path that continues or allows.
- **Texas status codes**: Inactive token `401`, wrong app, `NAVident` or `azp` `403`, Texas unreachable `502`, config missing `503`. Flag a status that leaks a different class (for example `500` or `401` for Texas down).
- **`azp`**: Compare against the configured client ID, never against a display name, a prefix or a `contains` check.
- **No secrets or personal data in logs or metric labels**: Tokens, `Authorization`, `NAVident`, emails, raw request bodies and participant hashes. High-cardinality labels (user, token, URL) are also a defect.
- **Body size**: Enforce the limit before buffering (`http.MaxBytesReader`, `Content-Length` check plus a capped read), not after `io.ReadAll`.
- **JSON-RPC envelope**: Validate `jsonrpc`, `method`, and `id` type. Return `-32600` for an invalid request and `-32020` for the gateway's own rejection. Batch and notification shapes must not bypass checks.
- **URLs and hosts**: Compare the parsed hostname (`url.Parse`, `Hostname()`), never `strings.Contains` or `HasPrefix` on the raw URL. Check scheme and port.
- **Write tools**: Confirmation for tools that change state must be enforced on the server, not trusted from a client flag or a prompt.
- **Revocation and lifetime**: Sessions and refresh tokens must stop at revocation and expiry, and a cancelled request must release its slot.
- **Old data**: Data written by an earlier version must also fail closed when it lacks a new field.
