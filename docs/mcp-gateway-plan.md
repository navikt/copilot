# MCP gateway: plan and path forward

Status 2026-11-16. Epic: [#2083](https://github.com/navikt/copilot/issues/2083). Registry self-service: [#2096](https://github.com/navikt/copilot/issues/2096). Client side: [navikt/cplt#912](https://github.com/navikt/cplt/issues/912), design in [navikt/cplt#947](https://github.com/navikt/cplt/pull/947) (merged 2026-11-04).

## Goal

Teams ship MCP servers without building their own OAuth. One gateway on Nais handles login, issues per-server tokens and calls each team MCP with Texas on-behalf-of (OBO). Teams register their servers on ki-utvikling. cplt keeps the tokens on the host, out of the agent sandbox.

## Decided architecture

| Topic | Decision | Source |
|---|---|---|
| Ownership | We build, own and run the gateway for good | [#2088](https://github.com/navikt/copilot/issues/2088) |
| Login | The gateway is its own OAuth 2.1 authorization server in front of Entra; Entra-direct revisited before phase 3 if Nais can support it | [#2089](https://github.com/navikt/copilot/issues/2089) |
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

## Token flow

Unverified marks behaviour that no spike or Nais documentation has confirmed yet.

1. **Agent to cplt host broker.** No token. The broker keeps credentials out of the sandbox ([navikt/cplt#947](https://github.com/navikt/cplt/pull/947), section 14).
2. **cplt broker to gateway authorization server.** Authorization code with PKCE. The client is identified by a CIMD document (`oauth-client.json`) that my-copilot will publish (#2085); the callback goes to the fixed loopback port the broker holds. The gateway issues a short-lived access token and a rotating refresh token, stored by cplt in a `0600` file in its state dir.
3. **Gateway to Entra ID.** The gateway is the Entra confidential client that Nais provisions (`azure.application`). It sends the user to Entra to log in and keeps the Entra refresh token encrypted for at most 10 hours ([#2090](https://github.com/navikt/copilot/issues/2090)). Whether Entra issues it via `offline_access` is unverified (phase 0b).
4. **cplt broker to gateway route.** Bearer gateway access token. The gateway checks the token, consent for that route, and writes one audit line with NAVident ([#2092](https://github.com/navikt/copilot/issues/2092)). Opaque: cplt cannot check `aud` and relies on the gateway binding the token to the route (RFC 8707). JWT: cplt refuses to send a token whose `aud` is not the route.
5. **Gateway to team MCP.** The gateway gets an Entra access token for itself from the stored refresh token (unverified), then Texas OBO exchanges it for one scoped to the team MCP. The team MCP validates it and its `accessPolicy.inbound` admits only the gateway.
6. **Team MCP to downstream API.** A second Texas OBO on the OBO token. Unverified: this is what the phase 0 spike tests.

Texas introspection is not documented to check `azp` or `groups` for you; each hop that relies on them must check them itself (unverified).

## Phases and gates

Each phase starts only when the previous gate holds.

| Phase | What | Gate to pass | Tracked in |
|---|---|---|---|
| 0 spike | Chained OBO through Texas, stub apps in dev-gcp | A team MCP exchanges the gateway's OBO token for its own downstream token. Fails: the gateway approach is killed | [#2084](https://github.com/navikt/copilot/issues/2084), [#2094](https://github.com/navikt/copilot/pull/2094) |
| 0b | Minimal authorization server on the dev stub, CIMD only | One `tools/call` from Copilot CLI via CIMD reaches the pilot as the real user; an audit line names NAVident and the tool; `offline_access` result recorded | [#2084](https://github.com/navikt/copilot/issues/2084) |
| 1 | cplt host OAuth against the gateway, fixed-port guard, per-route consent, revocation; privacy document | `cplt mcp status` shows `credential_ok`; no token readable in the sandbox; privacy document signed off | [#2085](https://github.com/navikt/copilot/issues/2085), design merged in [navikt/cplt#947](https://github.com/navikt/cplt/pull/947) |
| 2 pilot | Prod route to nav-etterlevelse-mcp, reads only, next to its own OAuth server | 5 users, 2 weeks, zero auth incidents, latency target met, one survey answer per user | [#2086](https://github.com/navikt/copilot/issues/2086) |
| 3 | Template repo, direct clients, second team | A second team ships from the template in under a day | [#2087](https://github.com/navikt/copilot/issues/2087) |

### Registry phases mapped to gateway phases

| Registry phase | Content | Lands with |
|---|---|---|
| 1 | Database in mcp-registry, writer folded in, static file wins on name collision | After gateway phase 0 passes ([#2098](https://github.com/navikt/copilot/issues/2098)) |
| 2 | Submit form and `/admin` approval queue | Gateway phase 2 pilot ([#2099](https://github.com/navikt/copilot/issues/2099)) |
| 3 | Form open to all teams, edit, deprecate and rollback; the gateway enforces `toolHashes` | Gateway phase 3 ([#2100](https://github.com/navikt/copilot/issues/2100)) |

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

## Setup checklist

In order. "Owner" means a human with the access; "agent" means repo work an agent can do.

| # | What | Where | Who | Tracked in |
|---|---|---|---|---|
| 1 | Run the chained-OBO spike, record the result, then delete the stub apps | dev-gcp, `spikes/2084-obo/` in [#2094](https://github.com/navikt/copilot/pull/2094) | Owner saves a dev token and runs `gcloud auth login`; agent calls the chain, tests the missing inbound grant, deletes the apps, posts on #2084, closes #2094 | [#2084](https://github.com/navikt/copilot/issues/2084) |
| 2 | Phase 0b: test whether Entra issues a refresh token (`offline_access`) to the gateway's `azure.application` (unverified) | Gateway app, Entra | Agent builds the test; owner verifies | [#2084](https://github.com/navikt/copilot/issues/2084) |
| 3 | Public ingress: see External dependencies | Nais | Owner | [#2083](https://github.com/navikt/copilot/issues/2083) |
| 4 | `claims` in Texas errors: see External dependencies | Nais | Owner | [#2083](https://github.com/navikt/copilot/issues/2083) |
| 5 | Choose the loopback port and publish `oauth-client.json` with that `redirect_uris` entry | my-copilot, path set in #2085 | Owner picks port; agent publishes | [#2085](https://github.com/navikt/copilot/issues/2085) |
| 6 | Registry phase 1: replace the all-zero `admin_group_id` with the real Entra group | mcp-registry vars in [#2110](https://github.com/navikt/copilot/pull/2110) | Owner supplies ID; agent wires | [#2098](https://github.com/navikt/copilot/issues/2098) |
| 6b | Gateway phase 1: provision the refresh-token encryption key | Gateway secret | Owner supplies secret; agent wires | [#2090](https://github.com/navikt/copilot/issues/2090) |
| 7 | Wire `accessPolicy`: gateway outbound to team MCPs, team MCP inbound from gateway only | Gateway and team MCP `.nais/*.yaml` | Agent (gateway); team owner (team MCP) | [#2083](https://github.com/navikt/copilot/issues/2083) |
| 8 | Privacy sign-off: see External dependencies | etterlevelse document in copilot-intern | Agent drafts; owner gets sign-off | [#2085](https://github.com/navikt/copilot/issues/2085) |

## Open decisions

1. Which loopback port cplt uses (checklist item 5).
2. Gateway access token format: opaque or JWT. Opaque: cplt cannot check `aud` and relies on the gateway binding the token to the route (RFC 8707). JWT: cplt refuses to send a token whose `aud` is not the route.
3. Where the refresh-token encryption key lives: Nais secret or KMS.

## Decisions log

### 2026-11-16: registry writes, personal data, database outage, spike gate

Owner decisions, recorded in [#2098 (comment)](https://github.com/navikt/copilot/issues/2098#issuecomment-6924375982) and implemented in [#2110](https://github.com/navikt/copilot/pull/2110):

1. **Write access.** The registry write endpoints accept only tokens whose `azp` is my-copilot. `ALLOWED_WRITE_AZP` lists `cluster:namespace:app` names, resolved to client ids through Nais `AZURE_APP_PRE_AUTHORIZED_APPS`; the manifest sets `{{cluster}}:copilot:my-copilot`, and the Entra app uses `tenant: nav.no`. Any other `azp` gets 403, and the rejection reason is not logged. An empty allowlist closes writes entirely.
2. **Personal data.** Audit and version rows store the caller's NAVident, not the email address. Audit rows (`registry_audit`) are deleted after 90 days. Version rows (`registry_entry_versions`) are kept for version history, but `submitted_by` is cleared after 90 days. `registry_entries.owner_navident` is kept while the entry exists, a deliberate exception to the 90-day rule, because ownership decides who may update the entry. The privacy document ([#2092](https://github.com/navikt/copilot/issues/2092)) must cover this exception. Logs carry no email address.
3. **Database outage.** Reads keep serving `allowlist.json`, writes answer 503, and the app retries connect and migrate in the background. The app does not exit fail-closed.
4. **Spike gate stands.** [#2102](https://github.com/navikt/copilot/pull/2102) and [#2110](https://github.com/navikt/copilot/pull/2110) merge together after the chained-OBO spike ([#2084](https://github.com/navikt/copilot/issues/2084)) passes.

Registry architecture: [`apps/mcp-registry/README.md`](../apps/mcp-registry/README.md#architecture).
