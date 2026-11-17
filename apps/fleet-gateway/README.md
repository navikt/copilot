# fleet-gateway

OAuth 2.1 authorization server fronting Microsoft Entra ID for Nav's internal MCP gateway (`nav-fleet`).

Step 1 implements the authorization server only: protected resource metadata, authorization server metadata, CIMD client verification, consent, and Entra authorization-code redemption. Downstream team MCP routing and Texas OBO exchange (`TeamTokenSource`) follow in step 2.

## Endpoints

- `GET /.well-known/oauth-protected-resource` — RFC 9728 Protected Resource Metadata
- `GET /.well-known/oauth-protected-resource/{resource}` — Resource-specific PRM (must match a configured pilot route)
- `GET /.well-known/oauth-authorization-server` — RFC 8414 Authorization Server Metadata
- `GET /authorize` — OAuth 2.1 authorization endpoint (CIMD allowlist, exact `redirect_uri`, PKCE `S256`, RFC 8707 `resource`, RFC 9207 `iss`)
- `POST /consent` — Consent submission with CSRF cookie check
- `GET /oauth2/callback` — Entra ID reply URL, verified against signed `__Host-fleet_entra_state` cookie
- `POST /token` — Single-use, 60-second authorization code exchange with PKCE verification
- `GET /health` — Liveness probe
- `GET /ready` — Readiness probe

## Configuration

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `BASE_URL` | Yes | — | External gateway origin without trailing slash |
| `ALLOWED_CLIENT_IDS` | Yes | — | Comma-separated allowlist of CIMD `client_id` URLs |
| `ALLOWED_REDIRECT_URIS` | Yes | — | Comma-separated allowlist of exact `redirect_uri` values |
| `AZURE_APP_CLIENT_ID` | Yes | — | Nais-injected Entra client ID |
| `AZURE_APP_CLIENT_SECRET` | Yes | — | Nais-injected Entra client secret |
| `AZURE_OPENID_CONFIG_TOKEN_ENDPOINT` | Yes | — | Nais-injected Entra token endpoint |
| `AZURE_APP_WELL_KNOWN_URL` | Conditional | — | Nais-injected OpenID discovery URL (required unless `AZURE_OPENID_CONFIG_AUTHORIZATION_ENDPOINT` is set) |
| `AZURE_OPENID_CONFIG_AUTHORIZATION_ENDPOINT` | Conditional | — | Direct override for Entra authorization endpoint |
| `ENTRA_SCOPE` | No | `openid api://<own client id>/.default` (plus `offline_access` for `refresh`) | Scope requested from Entra |
| `COOKIE_SIGNING_SECRET` | No | Random 32 bytes | HMAC key (>= 32 bytes) for `__Host-fleet_entra_state` cookie |
| `PORT` | No | `8080` | HTTP listen port |
| `LOG_LEVEL` | No | `INFO` | `DEBUG`, `INFO`, `WARN`, or `ERROR` |

Empty `ALLOWED_CLIENT_IDS` or `ALLOWED_REDIRECT_URIS` fails closed at startup.

## Development

```bash
mise run install  # Download Go modules
mise run lint     # Run golangci-lint
mise run test     # Run tests with -race
mise run check    # Run fmt, vet, staticcheck, lint, test
mise run build    # Build binary to bin/fleet-gateway
```

## References

- [MCP gateway plan](../../docs/mcp-gateway-plan.md)
