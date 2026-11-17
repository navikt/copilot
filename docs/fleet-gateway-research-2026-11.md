# MCP gateway: declarative configuration and policy, external research

Accessed 2026-11-17 (all sources). Scope: a small Go MCP gateway on Kubernetes that acts as an
OAuth 2.1 authorization server for MCP clients (PRM, CIMD, DCR, PKCE), exchanges user tokens for
upstream tokens with Entra ID On-Behalf-Of (OBO), enforces a per-route tool allowlist, and plans
declarative `Route` / `RouteOverride` / `GatewayPolicy` documents served by a separate control service.

Sources were web searches and a handful of page fetches. **(unverified)** marks a claim taken
from search summaries or third-party blogs and not checked against the primary source.

## 1. Comparison

| Product | Config model | Tool-level policy | Identity / downstream | Licence, maturity | Self-host on K8s |
|---|---|---|---|---|---|
| agentgateway (LF, ex-Solo.io, Rust) | Gateway API + `AgentgatewayPolicy`, `AgentgatewayBackend` CRDs (`agentgateway.dev/v1alpha1`); standalone YAML | CEL under `spec.backend.mcp.authorization`; vars `mcp.tool.name`, `mcp.tool.target`, `jwt.*`; actions Allow/Require; default allow | JWT auth, MCP OAuth; RFC 8693 / Entra OBO via built-in STS (blog 2026-07). Several OBO modes are enterprise tier **(unverified)** | Apache-2.0, LF since Aug 2025, very active, alpha APIs | Yes |
| kgateway | Gateway API controller; agentgateway is its AI data plane | Via agentgateway | Via agentgateway | Apache-2.0, CNCF | Yes |
| Envoy AI Gateway | `MCPRoute` CRD on Envoy Gateway | `spec.securityPolicy.authorization`: JWT scopes/claims, tool selection, CEL | MCP OAuth with `protectedResourceMetadata`; no native per-user token exchange (issue #2036) | Apache-2.0, v0.6, active | Yes |
| Microsoft MCP Gateway | K8s reverse proxy + management API | Per-server/tool RBAC **(unverified)** | Tied to Entra/AKS **(unverified)** | MIT **(unverified)** | Yes, Azure-leaning |
| Azure API Management MCP | Managed APIM policies | APIM policy XML | Entra, OAuth | Commercial, managed | No |
| AWS Bedrock AgentCore Gateway | Managed API | Interceptors/Cedar-based policy **(unverified)** | Cognito/OIDC, outbound OAuth | Commercial, managed | No |
| Cloudflare MCP portals / Access | Managed | Per-portal server/tool selection **(unverified)** | Access as IdP proxy | Commercial | No |
| Kong AI Gateway (MCP) | Kong plugins / decK | ACL plugin per tool **(unverified)** | OAuth2 plugin | OSS core + enterprise | Yes |
| Docker MCP Gateway | CLI/profile files | Server enable/disable, tool filter | Local secrets | OSS; single-user, not a multi-user control plane | Not its target |
| IBM ContextForge | Admin API + DB, federation | RBAC, plugins | JWT, OAuth to upstreams | Apache-2.0 **(unverified)** | Yes |
| Pomerium (MCP) | Pomerium policy (PPL) | Route-level, PPL | Pomerium as OAuth AS in front of MCP, upstream token handling **(unverified)** | Apache-2.0 core **(unverified)** | Yes |
| Obot | Platform: hosting + catalog + gateway | Multi-role RBAC | OAuth | MIT **(unverified)** | Yes |
| MCPJungle | Registry + gateway | Tool groups | Bearer tokens | MPL-2.0 **(unverified)** | Yes |
| Kuadrant MCP Gateway | Gateway API + Kuadrant policies | AuthPolicy (Authorino) | OIDC | Apache-2.0 | Yes; distinct API from Envoy AI GW |

Not one of these combines CIMD/DCR as the authorization server, per-user Entra OBO to many team
upstreams, and team-owned route documents with operator overrides. agentgateway comes closest.

## 2. What we can lean on

| Item | Verdict | Reason |
|---|---|---|
| Gateway API policy attachment (GEP-713, GEP-2648 direct attachment) | **Borrow the model** | `targetRefs` plus per-ancestor `status.ancestors[].conditions` cleanly separate owner and operator. We do not need CRDs to copy the shape. |
| Kubernetes `metav1.Condition` (`type`, `status`, `reason`, `message`, `observedGeneration`, `lastTransitionTime`) | **Adopt** | Universally understood; works in JSON over an API as well as in a CRD. |
| agentgateway CEL authorization vocabulary | **Borrow** | `mcp.tool.name`, `jwt.<claim>` and Allow/Require are becoming the de facto names. Our fixed rule evaluator can use the same field names and later accept CEL. |
| agentgateway as data plane | **Ignore for now, revisit** | See section 3. |
| Envoy AI Gateway `MCPRoute` | **Borrow names** | `backendRefs`, `securityPolicy.authorization.rules`; it lacks per-user token exchange, so not adoptable. |
| CEL (`github.com/google/cel-go`) | **Adopt later, optional** | Native Go, small, same language the Kubernetes and gateway ecosystem uses. A good escape hatch for `when:` expressions. |
| Cedar (`github.com/cedar-policy/cedar-go`, 1.x) | **Hold** | Mature Go lib, principal/action/resource fits tool calls well, analysable. Overkill until rules exceed allow-lists by tool, principal type, client and group. |
| OPA/Rego | **Ignore** | Heavier runtime and language than this needs. |
| OpenFGA | **Ignore** | Relationship graphs (team member of X) are already answered by Entra groups. |
| AuthZEN 1.0 (OpenID Authorization API) | **Borrow the shape** | Use `subject` / `action` / `resource` / `context` as the evaluator's input struct and decision log; costs nothing and keeps an external PDP possible. No Go client needed. |
| MCP tool annotations (`readOnlyHint`, `destructiveHint`, ...) | **Use as hints only** | The spec calls them untrusted hints. Use them for defaults and UI, never as the authorization decision; operator-set `toolRisk` stays authoritative. |
| MCP registry `_meta` | **Adopt convention** | Publisher data goes under `io.modelcontextprotocol.registry/publisher-provided` (max 4 KB **(unverified)**); sub-registries add their own reverse-DNS key in responses. Our own key is correct. |
| MCP auth 2026-07-28 | **Adopt** | DCR deprecated (12-month window), CIMD preferred, RFC 9207 `iss` in authorization responses, credentials keyed by issuer, sessions and `initialize` removed, `server/discover` required. |
| ID-JAG / enterprise-managed authorization | **Track** | Stable extension per secondary sources **(unverified)**. Lets an IdP grant access to the gateway's AS without per-user consent. Relevant when clients can do it. |
| RFC 8693 token exchange, `act` claim | **Borrow for agents** | Delegated-agent tokens should carry the agent in `act`/`azp` and the user in `sub`. Entra OBO is not RFC 8693, so keep the Entra path as is. |
| SPIFFE / GitHub OIDC workload identity | **Borrow for autonomous agents** | Accept federated workload tokens as client assertions (RFC 7523) instead of client secrets. |

## 3. Build vs adopt

**Verdict: keep building; borrow models and names; re-evaluate agentgateway in 6 months.**

What agentgateway could replace: the reverse proxy, JSON-RPC parsing, tool filtering in `tools/list`,
CEL authorization, rate limits, OpenTelemetry. That is the smallest and best-tested part of our gateway.

What it would not replace without enterprise features or custom code: the OAuth AS with CIMD/DCR
and consent, the per-user Entra OBO to many upstream audiences (open-source status unclear), and the
team/operator ownership model with a registry projection.

Costs and risks of adopting now:
- Two runtimes (Rust proxy plus our Go AS/control service) and a second deploy and on-call surface.
- Alpha CRD APIs (`v1alpha1`) that have already changed shape between minor versions.
- CRD-only Kubernetes config (standalone YAML is not promoted to CRDs everywhere **(unverified)**)
  fits poorly with a control service that publishes snapshots over HTTP.
- Vendor gravity: the strongest auth features are sold in an enterprise edition.
- Migration later is cheap *if* our documents use the same vocabulary, because the route table
  can then be compiled to `AgentgatewayBackend` + `AgentgatewayPolicy` instead.

Lock-in is lowest if we keep: Gateway API-shaped documents, `metav1.Condition` status, CEL-compatible
rule fields, and AuthZEN-shaped decision logs.

## 4. Concrete changes to the design

1. **Field names (Gateway API / agentgateway alignment)**
   - `spec.upstream` -> `spec.backendRef` (`{ cluster, namespace, name, path }`); keep the derived URL.
   - `RouteOverride.spec.targetRef: { kind: Route, name }` instead of matching by name implicitly.
   - `rules[].tools` -> `rules[].matches[].tool` (glob); `allow` -> `action: Allow|Deny` with
     `from: { principalTypes, clients, groups }`. Add optional `when:` (CEL string), rejected until enabled.
   - `metadata.team` -> `metadata.labels` or `spec.owner`; keep `metadata` K8s-shaped
     (`name`, `labels`, `annotations`, `generation`).
2. **Policy model**: decision = deny unless (route enabled) AND (no operator override disables) AND
   (client allowed by GatewayPolicy AND by route narrowing) AND (first matching rule allows). Override
   can only narrow. Input struct follows AuthZEN: `subject{type,id,groups}`, `action{name:"tools/call"}`,
   `resource{route,tool}`, `context{client_id,azp,act}`. Log the same struct per decision.
3. **Status**: each document gets `status.observedGeneration` and `status.conditions[]`:
   `Accepted` (validated; reasons `Invalid`, `TargetNotFound`, `NamespaceNotOwned`),
   `Programmed` (in a snapshot the gateway has loaded; reason `Pending`, `Stale`),
   `ResolvedRefs` (upstream reachable / network rule present), `Overridden` (operator override active).
   Gateway reports snapshot ETag back so `Programmed` is true only after the gateway confirms load.
4. **Registry `_meta`**: keep `_meta."<our-reverse-dns>/registry"` in our responses; derive
   `tools`, `owner`, `gateway`, `audience` from the Route. Do not use publisher annotations for risk.
5. **Auth spec**: plan DCR off by default for new clients (CIMD first), add RFC 9207 `iss` to
   authorization responses, stop relying on `Mcp-Session-Id` for anything, support `server/discover`
   passthrough.
6. **Agents**: model delegated agents as `azp` + `act` and autonomous agents as workload-federated
   client assertions; leave ID-JAG as a tracked extension.

## 5. Sources (accessed 2026-11-17)

- agentgateway tool access: https://agentgateway.dev/docs/kubernetes/main/documentation/mcp/tool-access/
- agentgateway authorization: https://agentgateway.dev/docs/kubernetes/main/security/authorization/
- agentgateway MCP authz (standalone): https://agentgateway.dev/docs/standalone/main/documentation/configuration/security/mcp-authz/
- agentgateway token exchange: https://agentgateway.dev/docs/kubernetes/main/documentation/security/backend-authn/token-exchange/mcp/
- agentgateway Entra OBO blog: https://agentgateway.dev/blog/2026-07-12-agentgateway-token-exchange-jwt-assertion-entra-obo/
- Governed MCP platform write-up: https://aaif.io/blog/building-a-governed-mcp-platform-with-agentgateway
- Auth patterns OSS vs enterprise (community): https://github.com/rvennam/agentgateway-auth-patterns
- Envoy AI Gateway MCP: https://aigateway.envoyproxy.io/docs/capabilities/mcp/
- Envoy AI Gateway token exchange proposal: https://github.com/envoyproxy/ai-gateway/issues/2036
- Gateway API GEP-713: https://gateway-api.sigs.k8s.io/geps/gep-713/ ; GEP-2648: https://gateway-api.sigs.k8s.io/geps/gep-2648/
- Policy attachment reference: https://gateway-api.sigs.k8s.io/reference/policy-attachment
- MCP authorization 2025-11-25: https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization
- MCP auth 2026-07-28 changes (secondary): https://ssojet.com/blog/mcp-authorization-spec-2026-07-28-what-changed
- CIMD / enterprise management: https://aaronparecki.com/2025/11/25/1/
- MCP registry aggregators: https://modelcontextprotocol.io/registry/registry-aggregators
- ToolHive registry `_meta` example: https://docs.stacklok.com/toolhive/reference/registry-schema-upstream
- Tool annotations (secondary): https://www.mcpbundles.com/guides/tool-annotations
- cedar-go: https://pkg.go.dev/github.com/cedar-policy/cedar-go
- Gateway comparisons (vendor-biased): https://composio.dev/content/best-mcp-gateway-for-developers , https://www.getmaxim.ai/articles/best-open-source-mcp-gateways-in-2026
- Gateway vs proxy authorization: https://cerbos.dev/blog/authorizing-mcp-tool-calls-at-the-gateway-or-inside-the-proxy

Not checked against primary sources: AgentCore, Cloudflare, Kong, Pomerium, Microsoft MCP Gateway
and ContextForge feature details; AuthZEN 1.0 status; the ID-JAG extension text.

## Verification 2026-11-17

Primary sources only (modelcontextprotocol.io spec pages, fetched 2026-11-17). Revision ids are the spec's own `YYYY-MM-DD` strings; the pages carry no separate publication date.

### Claims

1. CONFIRMED. https://modelcontextprotocol.io/specification/versioning: "The **current** protocol version is **2026-07-28**." The draft changelog (post-2026-07-28) holds only a CIMD draft-03 bump and a `_meta` schema fix.
2. PARTLY CONFIRMED, wording differs. Changelog 2026-07-28, Deprecated item 4: "Deprecate the OAuth 2.0 Dynamic Client Registration Protocol (RFC7591) as a client registration mechanism in favor of Client ID Metadata Documents (PR #2858). It remains available for backwards compatibility with authorization servers that do not support Client ID Metadata Documents." But CIMD is SHOULD, not required: authorization page "Authorization servers and MCP clients **SHOULD** support OAuth Client ID Metadata Documents"; DCR "**MAY**". Client priority (SHOULD): 1 pre-registered, 2 CIMD if `client_id_metadata_document_supported`, 3 DCR fallback if `registration_endpoint`, 4 prompt user. DCR page: "Dynamic Client Registration is deprecated. New implementations should use Client ID Metadata Documents instead." Deprecated features stay at least 12 months (versioning page). Draft changelog: implementations supporting CIMD "MUST follow" draft-ietf-oauth-client-id-metadata-document-03 (was -00).
3. PARTLY REFUTED. Server side is SHOULD, not MUST. Changelog minor item 7: "Authorization servers **SHOULD** include the `iss` parameter in authorization responses per RFC 9207, and MCP clients **MUST** validate a present `iss` against the recorded issuer before redeeming the authorization code (SEP-2468)." Authorization page: servers that send `iss` MUST set `authorization_response_iss_parameter_supported: true`. Client table: flag true + `iss` absent -> reject; flag false/absent + `iss` present -> still compare; both absent -> proceed. Simple string comparison, no normalisation. Also applies to error responses. "A future revision ... is expected to upgrade ... from SHOULD to MUST."
4. CONFIRMED, and larger than stated. Changelog major 1: "Remove protocol-level sessions and the `Mcp-Session-Id` header from the Streamable HTTP transport" (SEP-2567); cross-call state uses server-minted handles passed as tool arguments. Major 2: "Make MCP stateless: remove the `initialize`/`notifications/initialized` handshake" (SEP-2575). Replaced by: per-request `_meta` keys `io.modelcontextprotocol/protocolVersion`, `.../clientCapabilities` (+ SHOULD `clientInfo`); `MCP-Protocol-Version` header; new mandatory `server/discover` RPC; `UnsupportedProtocolVersionError`. Also: new required POST headers `Mcp-Method`, `Mcp-Name` (+ `Mcp-Param-*` via `x-mcp-header`, SEP-2243); HTTP GET endpoint and `resources/subscribe` replaced by `subscriptions/listen` (POST response stream); SSE resumability (`Last-Event-ID`) removed (major 9); server-initiated requests replaced by multi round-trip requests (`input_required`). Back-compat with 2025-11-25 and earlier remains via the handshake path (versioning page).
5. CONFIRMED. Tools page (2026-07-28): "clients **MUST** consider tool annotations to be untrusted unless they come from trusted servers." Wording is "untrusted", not "hints". Draft tool-annotation trust work lives in `experimental-ext-tool-annotations` (Experimental).
6. CONFIRMED as a stable extension, not core and not draft. https://modelcontextprotocol.io/docs/extensions/overview lists "Enterprise-Managed Authorization" (`io.modelcontextprotocol/enterprise-managed-authorization`) as **Stable** in `modelcontextprotocol/ext-auth`, spec dir `specification/stable/enterprise-managed-authorization/2026-07-28/`, origin SEP-990. Flow: client exchanges the IdP ID token for an ID-JAG (RFC 8693, `requested_token_type=urn:ietf:params:oauth:token-type:id-jag`), then presents it to the MCP AS via `urn:ietf:params:oauth:grant-type:jwt-bearer`. ID-JAG itself is an IETF OAuth WG draft (`draft-ietf-oauth-identity-assertion-authz-grant`), not an RFC; vendors track different draft revisions. Extensions are opt-in. Lifecycle: Stable changes only with a new core revision.

### Impact on apps/fleet-gateway (origin/main, read-only)

- `iss`: OK. `authorize.go:433` and `:624` set `iss` = BaseURL on both success and error redirects; `consent.go:109` too; `prm.go:41` advertises `authorization_response_iss_parameter_supported: true`. Already matches the 2026-07-28 text. No change.
- Sessions: no break. `proxy.go` only binds `Mcp-Session-Id` values the upstream issues on a <300 response, and checks ownership of client-sent ids. Under 2026-07-28 upstreams issue none, so the binding (#2251) becomes inert, not harmful. Keep it for 2025-11-25 clients. No `initialize` dependence in gateway code (only `echo/main.go` test upstream speaks `initialize`, protocol 2025-03-26).
- GAP, not verified end to end: `allowedUpstreamRequestHeaders` (`proxy.go:29`) is Content-Type, Accept, Mcp-Session-Id, Mcp-Protocol-Version, Last-Event-Id. New-revision clients send `Mcp-Method`, `Mcp-Name`, `Mcp-Param-*` on POST. The proxy would strip them. An upstream that checks headers against the body would answer `HeaderMismatch`. Needs an allowlist addition (or regenerate from body) before 2026-07-28 clients are supported. I did not read the Streamable HTTP transport page for the exact mismatch rule or whether it applies when a gateway rewrites headers.
- Smaller 2026-07-28 items: GET/`Last-Event-Id` path becomes legacy; `subscriptions/listen` is a long POST SSE stream and falls under `SSEMaxDuration`/`SSEIdleTimeout`/`SSEMaxBytes` caps; the gateway has no `server/discover` handling (passes through if the route allows POST).
- DCR (#2270): still needed. Deprecated but retained, minimum 12 months, and priority is pre-registered > CIMD > DCR. Gateway restricts CIMD to `ALLOWED_CLIENT_IDS`; a client with no allowlisted CIMD and no pre-registration (OpenCode 1.19, assumed to use DCR; not checked here) needs `/register`. Already flag-gated (`DCR_ENABLED`, default false). Keep; plan removal once clients move to CIMD. Gateway already advertises `client_id_metadata_document_supported: true`.
- Follow-up: CIMD fetch/validation (`authorize.go:208`) was written to draft-00; the post-2026-07-28 draft says implementations MUST follow draft-03. Diff the two drafts.
- ID-JAG / enterprise-managed auth: gateway implements none; stable extension, opt-in, no forced change. Could matter for Entra-driven policy later; not a blocker.
