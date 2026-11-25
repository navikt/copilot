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

## Guidelines (source of truth)

The full rules, each linked to OWASP MCP Top 10 and the MCP `2026-07-28` spec, live in one place: [Retningslinjer for MCP-servere](https://ki-utvikling.nav.no/mcp/tilby-server/retningslinjer) (`apps/my-copilot/src/app/(nb)/(mcp)/mcp/tilby-server/retningslinjer/page.tsx`). Do not repeat them here; read the page. Registration steps: [fleet-gateway for teams](https://ki-utvikling.nav.no/mcp/tilby-server).

Checklist:

- [ ] Texas introspection; `azp` is fleet-gateway, `NAVident` present, app tokens rejected; OBO, no passthrough.
- [ ] Read tools first; writes only after ROS sign-off.
- [ ] Write tools: the server enforces confirmation itself (two-step preview + short-lived token, as `mcp-dolly` `create_order`); a client prompt or `input_required` alone is not enough.
- [ ] Explicit `tools:` list in route and registry, no `passAllTools`.
- [ ] Caps on input, request body, page size and response size; timeouts upstream.
- [ ] `Mcp-*` headers checked against the body (`-32020`).
- [ ] No tokens or personal data in logs.
- [ ] Static descriptions; all four annotations set; annotations are hints only.
- [ ] Per-tool metrics with bounded labels (tool name, outcome).
- [ ] Output is an API: `outputSchema` + `structuredContent` (also the serialized JSON as a `content` text block, for clients that ignore `structuredContent`); only add fields.
- [ ] Never truncate silently: cut on characters, not bytes, with a marker and a tool hint; lists carry `truncated` and the remaining count.
- [ ] Oversized backend replies are errors, not truncated data.
- [ ] Free text written by others is marked or left out of what the model reads.
- [ ] User ID from the token (`NAVident`), never from tool arguments.
- [ ] No citizen or case data; never log content.
- [ ] Few tools; no demo tools in prod.
- [ ] Negative tests: expired token, app token, wrong `azp`, missing `NAVident`, wrong tool, oversized body.
- [ ] Fresh login before writes once the owner has set the rule (#2964, not decided).
- [ ] One ROS per server (template: `fleet-gateway/ros-mcp-server-mal.md` in the private `navikt/copilot-intern` repo).

## Registry and gateway route

- Registry: add an entry to `apps/mcp-registry/allowlist.json`. The `name` (e.g. `io.github.navikt/kodeverk`) must not contain the transport.
- Gateway: add a route to `apps/fleet-gateway/pilot-routes.prod.yaml` (and dev) with an explicit `tools:` list. Only the operator uses `passAllTools`. `tools` in the route and in the registry must match.
- Network: the route alone does not make the server reachable. Add the server to `outbound_rules` in `apps/fleet-gateway/.nais/dev-gcp.yaml` and `.nais/prod-gcp.yaml` (`application`, plus `namespace` when the server is outside `copilot`).

## Further reading

[OWASP MCP Top 10](https://owasp.org/www-project-mcp-top-10/), [MCP authorization](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization), [tools spec](https://modelcontextprotocol.io/specification/2026-07-28/server/tools), [Writing tools for agents](https://www.anthropic.com/engineering/writing-tools-for-agents).

## Language notes

- **Kotlin:** official [Kotlin MCP SDK](https://github.com/modelcontextprotocol/kotlin-sdk) with Ktor (`mcpStreamableHttp`). The README says nothing on auth or spec version: add the Texas check as a Ktor plugin and test the `2026-07-28` behaviour yourself.
- **TypeScript:** official [TypeScript SDK](https://github.com/modelcontextprotocol/typescript-sdk) (v2 follows `2026-07-28`, Streamable HTTP). Validate tokens before the transport sees the request.
- **Go:** no SDK needed; copy the pattern from `apps/mcp-kodeverk` (reference for auth, caps, pagination, output schema, compliance test `mcp2026_compliance_test.go`).

## References

- `apps/mcp-kodeverk` — read-only reference server
- `apps/mcp-dolly` — write tool (`create_order`) with caps, quota, ownership, audit line
- `apps/fleet-gateway/README.md` — routes, tool lists, limits
- [fleet-gateway for teams](https://ki-utvikling.nav.no/mcp/tilby-server)
- [Retningslinjer for MCP-servere](https://ki-utvikling.nav.no/mcp/tilby-server/retningslinjer)
- [Anthropic mcp-builder](https://github.com/anthropics/skills/tree/main/skills/mcp-builder) (source of the workflow, Apache-2.0)
