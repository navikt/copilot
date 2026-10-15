---
applyTo: "apps/**/*.go,apps/**/.nais/*.yaml"
---

# Go services and NAIS manifests instructions

## Authentication and trust boundaries

- **JWT and caller validation (`copilot-api`, `copilot-survey`)**: Validate JWT signatures against JWKS (`use` must be `"sig"` or empty). Rate-limit JWKS refreshes on unknown `kid` misses (`minRefreshInterval`) and never hold the JWKS cache lock across outbound network I/O. Verify `azp` against the configured caller allowlist before trusting `X-On-Behalf-Of`.
- **`X-On-Behalf-Of` handling**: Only trust `X-On-Behalf-Of` when the caller's `azp` matches a machine-to-machine client (`copilot-cli` or `copilot-survey`). For OBO user tokens (`oid` / `NAVident` present), resolve the user from token claims and ignore `X-On-Behalf-Of`.
- **`copilot-cli` token verification**: Verify `ghu_` tokens via `POST /applications/{client_id}/token`, validate `check.User.Login` against `validGitHubUsername`, and verify active `navikt` membership fail-closed (`403` on non-member, `502` on upstream failure). Deduplicate concurrent cold-token checks with `singleflight` before spending rate-limit burst tokens.
- **`copilot-survey` anonymity**: Keep `survey_participation` and `survey_answers` unlinked. Insert participation (`ON CONFLICT DO NOTHING`) and answer rows inside a single PostgreSQL transaction so failed writes never burn a user's participation token. Never log `NAVident`, email, SAML `nameId`, or participant hashes.
- **`mcp-onboarding` OAuth 2.1**: Require PKCE (`S256`) on `/oauth/authorize` and `/oauth/token`. Build metadata URLs from configured `BaseURL`, never from `r.Host` or `X-Forwarded-Host`. Enforce TTL checks on read and cap in-memory session and token maps.

## Input, upstream, and response safety

- **Bounded reads**: Wrap incoming request bodies with `http.MaxBytesReader` (or decode with `atEOF` where strict JSON schema validation applies) and wrap upstream HTTP response bodies with `io.LimitReader` before `io.ReadAll` or `json.NewDecoder`.
- **URL construction**: Escape path segments with `url.PathEscape` and query values with `url.QueryEscape` before building GitHub API or internal service URLs. Do not return raw `*url.Error` values when the URL contains a username or token.
- **GraphQL error checking**: Always inspect `gqlResp.Errors` on GitHub GraphQL responses before caching results or writing batch scan output to BigQuery (`copilot-adoption`, `copilot-api`). A HTTP 200 GraphQL response with `errors` must not be treated as a valid empty result.
- **Sanitized client errors and metrics**: Log detailed errors server-side with `slog` (sanitizing user-controlled strings in `mcp-onboarding` with `logSafe`) and return generic error messages or RFC 7807 problem details to clients. Bound Prometheus metric labels (`path`, `tool`, `server`) to fixed allowlists so untrusted inputs cannot explode series cardinality.
- **NAIS manifests**: Ensure `.nais/*.yaml` inbound and outbound `accessPolicy` rules match the actual service communication paths documented in `SECURITY.md`.
