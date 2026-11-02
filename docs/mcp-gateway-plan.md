# MCP gateway: plan and path forward

Status 2026-11-02. Epic: [#2083](https://github.com/navikt/copilot/issues/2083). Registry self-service: [#2096](https://github.com/navikt/copilot/issues/2096). Client side: [navikt/cplt#912](https://github.com/navikt/cplt/issues/912), design in [navikt/cplt#947](https://github.com/navikt/cplt/pull/947).

## Goal

Teams ship MCP servers without building their own OAuth. One gateway on Nais handles login, issues per-server tokens and calls each team MCP with Texas on-behalf-of (OBO). Teams register their servers on ki-utvikling. cplt keeps the tokens on the host, out of the agent sandbox.

## Decided architecture

| Topic | Decision | Source |
|---|---|---|
| Ownership | We build, own and run the gateway for good | [#2088](https://github.com/navikt/copilot/issues/2088) |
| Login | The gateway is its own OAuth 2.1 authorization server in front of Entra | [#2089](https://github.com/navikt/copilot/issues/2089) |
| Refresh tokens | Stored in the gateway, encrypted, at most 10 hours | [#2090](https://github.com/navikt/copilot/issues/2090) |
| Writes | Read-only at first; writes later as a consent-page scope | [#2091](https://github.com/navikt/copilot/issues/2091) |
| Audit | One line per tool call, NAVident, 90 days | [#2092](https://github.com/navikt/copilot/issues/2092) |
| Registry approval | Gateway team MCPs: admin queue on ki-utvikling. External servers: PR review | [#2093](https://github.com/navikt/copilot/issues/2093), [#2096](https://github.com/navikt/copilot/issues/2096) |

Owner decisions 2026-11-02:

1. **Login guard for cplt**: a fixed loopback port, held by the host broker before the sandbox starts and listed in the CIMD `redirect_uris`. The short code is UX text only. The callback path has no random segment; `state` and PKCE stay.
2. **Registry writer**: folded into mcp-registry. One app, one database, one validator.
3. **Registry admins**: an Entra group through the `groups` claim from Texas introspection. No email list.
4. **First remote for cplt OAuth**: the gateway, then Figma.
5. **Client registration**: CIMD only in the pilot. DCR (and OpenCode, which needs it) after the pilot, with a per-IP cap and expiry of unused clients.
6. **Registry PRs** [#2102](https://github.com/navikt/copilot/pull/2102) and [#2110](https://github.com/navikt/copilot/pull/2110): fix review findings now, merge after the chained-OBO spike passes.
7. **Privacy**: one compliance document covering refresh tokens and the audit log, written during phase 1.
8. **`offline_access`**: tested in phase 0b. If Entra refuses, #2090 is revisited before phase 1.

## Phases and gates

Each phase starts only when the previous gate holds.

| Phase | What | Gate to pass | Tracked in |
|---|---|---|---|
| 0 spike | Chained OBO through Texas, stub apps in dev-gcp | A team MCP exchanges the gateway's OBO token for its own downstream token. Fails: the gateway approach is killed | [#2084](https://github.com/navikt/copilot/issues/2084), [#2094](https://github.com/navikt/copilot/pull/2094) |
| 0b | Minimal authorization server on the dev stub, CIMD only | One `tools/call` from Copilot CLI via CIMD reaches the pilot as the real user; `offline_access` result recorded | [#2084](https://github.com/navikt/copilot/issues/2084) |
| 1 | cplt host OAuth against the gateway, fixed-port guard, per-route consent, revocation; privacy document | `cplt mcp status` shows `credential_ok`; no token readable in the sandbox; privacy document signed off | [#2085](https://github.com/navikt/copilot/issues/2085), [navikt/cplt#947](https://github.com/navikt/cplt/pull/947) |
| 2 pilot | Prod route to nav-etterlevelse-mcp, reads only, next to its own OAuth server | 5 users, 2 weeks, zero auth incidents, latency target met | [#2086](https://github.com/navikt/copilot/issues/2086) |
| 3 | Template repo, direct clients, second team | A second team ships from the template in under a day | [#2087](https://github.com/navikt/copilot/issues/2087) |

### Registry phases mapped to gateway phases

| Registry phase | Content | Lands with |
|---|---|---|
| 1 | Database in mcp-registry, writer folded in, static file wins on name collision | After gateway phase 0 passes ([#2098](https://github.com/navikt/copilot/issues/2098)) |
| 2 | Submit form and `/admin` approval queue | Gateway phase 2 pilot ([#2099](https://github.com/navikt/copilot/issues/2099)) |
| 3 | Form open to all teams, edit and rollback; the gateway enforces `toolHashes` | Gateway phase 3 ([#2100](https://github.com/navikt/copilot/issues/2100)) |

The `owner`, `gateway` and `toolHashes` fields are already in the registry response format ([#2101](https://github.com/navikt/copilot/pull/2101)).

## External dependencies

| Dependency | Needed for | Status |
|---|---|---|
| Public ingress for the gateway (Nais) | Clients outside the internal network, phase 1 | Not requested yet |
| `offline_access` on a Nais Entra app (Nais) | Refresh tokens, phase 0b | Unverified; tested in 0b |
| `claims` in Texas error responses (Nais) | Conditional Access challenges without a re-login loop | Missing in Texas |
| Privacy sign-off | Refresh tokens and audit with NAVident, before phase 2 | Document written in phase 1 |
| Pilot team agreement | Phase 2 | Not asked yet |

## Open risks

- Chained OBO is unverified until the spike runs.
- Entra may refuse `offline_access`; then the 10-hour session in #2090 does not hold.
- Without Texas `claims`, a Conditional Access policy can lock users in a login loop.
- The writer has never been tested against real Texas introspection.
- A stored refresh token lets the gateway reach any route unless consent is per route.
- OpenCode under cplt waits for DCR, which is out of the pilot.
