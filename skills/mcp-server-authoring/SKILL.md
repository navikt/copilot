---
name: mcp-server-authoring
description: Skriv, test og registrer MCP-servere som kjører bak fleet-gateway på Nais, med Texas-auth, verktøydesign for agenter, ROS og OWASP MCP Top 10
license: MIT
compatibility: MCP server on Nais behind fleet-gateway (Kotlin, TypeScript or Go)
metadata:
  domain: platform
  tags: mcp fleet-gateway texas kotlin typescript go owasp security
---

# MCP Server Authoring

Build an MCP server that Nav teams can run behind fleet-gateway. Workflow adapted from Anthropic's `mcp-builder` (Apache-2.0, see `NOTICE`); the Nav layer is added.

## When to Use

- Creating a new MCP server on Nais, or adding tools to one
- Registering a server in the registry and the gateway
- Reviewing an MCP server for security or agent usability

## Workflow

1. **Plan tools.** List the tasks an agent must finish, not the API endpoints. Merge list+filter into one search tool. Aim for few tools.
2. **Implement.** Pick the SDK (see Language notes), add auth (Nav layer), input validation, caps.
3. **Test.** Unit-test each tool. Run the server locally and call it with MCP Inspector. Test the auth rejects (wrong audience, wrong `azp`, app token).
4. **Evaluate.** Write 10 realistic multi-step questions with a known answer. Measure accuracy, number of calls, errors. Fix descriptions first, code second.
5. **Register and ship.** ROS, registry entry, gateway route, then rollout.

## Nav layer: behind fleet-gateway

Reference: [fleet-gateway for teams](https://ki-utvikling.nav.no/verktoy/fleet-gateway/for-team).

- [ ] No public ingress. Clients reach the server only through fleet-gateway.
- [ ] Validate every token with Texas introspection. Check audience (issued to your server), that a `NAVident` claim exists, and that `azp` is fleet-gateway's Entra client ID. `azp` holds the client ID, not the name: find the entry named `<cluster>:copilot:fleet-gateway` (e.g. `prod-gcp:copilot:fleet-gateway`) in `AZURE_APP_PRE_AUTHORIZED_APPS` using `NAIS_CLUSTER_NAME`, and compare `azp` with its `clientId` (see `apps/mcp-kodeverk/auth.go`). Reject app (machine) tokens. Fail closed when the lookup or any config fails.
- [ ] 401 for missing/invalid/expired token, 403 for missing rights. Never use the session id as authentication.
- [ ] Call your own API with Texas OBO. Never forward the incoming token (token passthrough).
- [ ] `accessPolicy.inbound` allows fleet-gateway only; `accessPolicy.outbound` allows your API and Texas only.
- [ ] Secrets from Nais secrets only. Never in code, logs, errors or `x-mcp-header`.

Protocol:

- Implement MCP `2026-07-28`; keep the legacy-version window the gateway still supports (check the registry's listed protocol versions).
- Always validate the `MCP-Protocol-Version`, `Mcp-Method` and (for `tools/call`) `Mcp-Name` headers against the body, and reject a missing mandatory header. Do this even when no tool mirrors a param. Answer a mismatch with HTTP `400` and JSON-RPC `-32020`. Validate `Mcp-Param-*` headers too where a tool declares them (see `apps/mcp-kodeverk/mcp2026_compliance_test.go`).
- Answer `405` to `GET` when you offer no stream.
- Set a request body cap (`MaxBytesReader` in Go) and timeouts on every upstream call; server write timeout above the sum of upstream timeouts.

## Tool design for agents

- Names: service + resource (`kodeverk_search`), `A-Za-z0-9_-.`, unique, 1-128 chars.
- Descriptions are written for a new colleague: when to use and not use, formats, abbreviations, one example. Keep them static and in source control (no text from external sources: tool poisoning).
- Unambiguous params (`user_id`, not `user`). Validate with JSON Schema: `minimum`, `maximum`, `maxLength`, `enum`, `additionalProperties: false`.
- Errors the model can act on: `isError: true` with what is wrong, the valid format and an example. JSON-RPC errors only for invalid requests.
- Set all four annotations on every tool: `readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`.
- **Annotations are hints, not a security boundary.** Enforce on the server.
- Write tools:
  - [ ] Server-side caps (count, age), quota per day, ownership check, audit line.
  - [ ] Two-step confirm: first call returns a preview plus a short-lived confirm token; the second call must present it. A description saying "always confirm with the user" is not enough.
  - [ ] Safe to retry: idempotency key or a clear "already created" reply.
  - [ ] Check access on every call; a state handle is a name, not a right.
- Do not fetch user-supplied URLs (SSRF). If you must: https only, block private/loopback/link-local, validate redirects, restrict egress with `accessPolicy`.
- Treat upstream data and user free text as data, never as instructions. Add no instructions to results.

## Efficiency

- [ ] Small default page size, hard page cap, deterministic order.
- [ ] Return `truncated: true`, `nextOffset` and one sentence on how to narrow the search.
- [ ] Response byte cap (kodeverk uses 32 KiB).
- [ ] `structuredContent` + `outputSchema`, and the same JSON serialised in a text block for legacy clients.
- [ ] Only needed fields; translate ids to names; optional `response_format` (`concise`/`detailed`).
- [ ] Timeouts on all upstream calls; limit concurrent upstream calls; collapse identical calls (`singleflight`).
- [ ] Cache only shared reference data. Never cache per-user data across users.
- [ ] Fixed tool order; use `ttlMs`/`cacheScope` on `tools/list`.
- [ ] Rate-limit tool calls; answer 429 with `Retry-After`.

## Logging

- Log who (gateway audits the user), tool, outcome, correlation id.
- Do not log tokens, free text, or inputs with personal data. If you log `NAVident` on purpose as an audit trail, say so in the ROS.

## ROS

One ROS per server. Template: `fleet-gateway/ros-mcp-server-mal.md` in the private `navikt/copilot-intern` repo. Write tools stay out of the gateway route until the ROS is signed off.

## Registry and gateway route

- Registry: add an entry to `apps/mcp-registry/allowlist.json`. The `name` (e.g. `io.github.navikt/kodeverk`) must not contain the transport.
- Gateway: add a route to `apps/fleet-gateway/pilot-routes.prod.yaml` (and dev) with an explicit `tools:` list. Only the operator uses `passAllTools`. `tools` in the route and in the registry must match.
- Network: the route alone does not make the server reachable. Add the server to `outbound_rules` in `apps/fleet-gateway/.nais/dev-gcp.yaml` and `.nais/prod-gcp.yaml` (`application`, plus `namespace` when the server is outside `copilot`).

## OWASP MCP Top 10 mapping

[OWASP MCP Top 10](https://owasp.org/www-project-mcp-top-10/)

| ID | Risk | Practice here |
|---|---|---|
| MCP01 | Token Mismanagement & Secret Exposure | Nais secrets only; OBO, no passthrough; no tokens in logs or query strings |
| MCP02 | Privilege Escalation via Scope Creep | Least privilege, read-only first; caps, quota and ownership on writes |
| MCP03 | Tool Poisoning | Static descriptions in source control; test them in CI; pin dependencies by hash |
| MCP04 | Software Supply Chain Attacks & Dependency Tampering | Pinned deps, Dependabot, official SDKs |
| MCP05 | Command Injection & Execution | JSON Schema validation, body cap, no shell, no user URLs (SSRF) |
| MCP06 | Prompt Injection via Contextual Payloads | Output is data; no instructions in results; sanitise upstream text |
| MCP07 | Insufficient Authentication & Authorization | Texas audience + `azp` + `NAVident`; 401/403; no session auth |
| MCP08 | Lack of Audit and Telemetry | Who, tool, outcome, correlation id; no personal data |
| MCP09 | Shadow MCP Servers | Only registry servers, only through the gateway route |
| MCP10 | Context Injection & Over-Sharing | Response caps, only needed fields, no cross-user cache |

More: [MCP authorization](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization), [security best practices](https://modelcontextprotocol.io/specification/2025-11-25/basic/security_best_practices), [tools spec](https://modelcontextprotocol.io/specification/2026-07-28/server/tools), [multi round-trip requests](https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/mrtr) (`input_required` for confirmation where clients support it), [Writing tools for agents](https://www.anthropic.com/engineering/writing-tools-for-agents).

## Language notes

- **Kotlin:** official [Kotlin MCP SDK](https://github.com/modelcontextprotocol/kotlin-sdk) with Ktor (`mcpStreamableHttp`). The README says nothing on auth or spec version: add the Texas check as a Ktor plugin and test the `2026-07-28` behaviour yourself.
- **TypeScript:** official [TypeScript SDK](https://github.com/modelcontextprotocol/typescript-sdk) (v2 follows `2026-07-28`, Streamable HTTP). Validate tokens before the transport sees the request.
- **Go:** no SDK needed; copy the pattern from `apps/mcp-kodeverk` (reference for auth, caps, pagination, output schema, compliance test `mcp2026_compliance_test.go`).

## References

- `apps/mcp-kodeverk` — read-only reference server
- `apps/mcp-dolly` — write tool (`create_order`) with caps, quota, ownership, audit line
- `apps/fleet-gateway/README.md` — routes, tool lists, limits
- [fleet-gateway for teams](https://ki-utvikling.nav.no/verktoy/fleet-gateway/for-team)
- [Anthropic mcp-builder](https://github.com/anthropics/skills/tree/main/skills/mcp-builder) (source of the workflow, Apache-2.0)
