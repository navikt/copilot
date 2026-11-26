# Design note: team-internal MCP servers

Status: proposal for review (#2209). Not built.

## Summary

Owner decision 2026-11-26 (#2209): a team-internal MCP server is **hidden
and restricted**.

- A new spec field marks a server internal. Marking it internal reduces
  access, so it publishes without approval. Making it public again
  escalates to the platform team, as every widening change does under #3071.
- fleet-control hides internal servers from everyone outside the owner team,
  in its read APIs and in the registry snapshot. The platform team sees all.
- fleet-gateway refuses callers who are not members of the owner team on an
  internal route. Membership comes from the Nais Console API with the service
  account the gateway already has, cached briefly. Console down means 503.
- `/fleet` in my-copilot gets a switch in the wizard and on the server page,
  and an «Intern» tag.

The risky part is the snapshot contract: an old gateway that ignores a new
field would serve an internal route to everyone. The routes snapshot
therefore moves to `schema_version: 2`, which old gateways refuse.

## Spec field

```yaml
spec:
  visibility: Team   # or Public; omitted means Public
```

One enum, not a boolean, so a later value (for example a list of teams, V6
in #2209) does not need a second field. Validation: `Public` or `Team`, else
422 `Invalid` (`api/servers_validate.go`). Stored in `server_versions.spec`
like every other field; no new column is needed for fleet-control's own
decisions, but the snapshot builder needs it on `model.Server`
(`model/model.go:23-35`), next to `DataClass`.

### Escalation semantics

`Classify` (`model/escalation.go:36-109`) treats an unknown changed field as
an escalation (`:102-105`). That is the safe default but wrong for one
direction. Add a case:

| Change | Escalates | Why |
| --- | --- | --- |
| `Public` (or absent) to `Team` | no | fewer people can see and call it |
| `Team` to `Public` (or absent) | yes, reason `visibility widened` | more people can see and call it |
| first version with `Team` | yes | every first version escalates (`:40-43`) |

So a published public server can go internal by itself (auto-publish,
`api/servers_lifecycle.go:161-186`), and going public again waits for the
platform team (`draft_submitted`, `:128-131`). A table test row per
direction goes in `model/escalation_test.go`.

Interaction with rollback: rollback runs no `Classify`
(`docs/states.md`, question 2). Rolling back from an internal generation to
an earlier public one widens access without approval. Either rollback
refuses a target that widens visibility (409, new reason
`RollbackWidensVisibility`), or this is accepted. See open question 1.

## fleet-control read API

Today the catalogue serves every published server to every caller: the
handlers ignore the caller (`api/catalog.go:47-70`, query at `:84-88`).
`MCPServer` reads already restrict to owner and platform team
(`api/servers_http.go:47-52`), so they need no change.

Change: `listCatalog` and `getCatalogEntry` drop entries whose published
generation has `visibility: Team` unless the caller is in the owner team or
the platform team. A hidden entry answers 404, the same as a missing one, so
a stranger cannot probe names. `consumers` (`api/catalog.go:72`) follows
the same rule.

Agent profiles: `checkGrantServers` (`api/apis_agents.go:178-196`) accepts a
grant on any published server. A profile from another team that grants an
internal server would reveal it (422 text names it) and would be refused at
the gateway anyway. Refuse with the same 422 as "not published" when the
profile's team is not the owner team. See open question 3.

## Registry snapshot

`snapshotItems` (`api/snapshot.go:41-79`) puts each server's public
`server.json` into the registry snapshot, which mcp-registry serves to
everyone. Internal servers are left out of the registry snapshot entirely.
The registry has no notion of a caller today, so "visible to the owner
team" cannot be served there; the owner team finds the server in `/fleet`.

Leaving the item out is a normal snapshot change: no schema change for
mcp-registry, which already refuses unknown `schema_version`
(`apps/mcp-registry/internal/server/fleet_control.go:158-167`). A server that
goes internal disappears from the registry on the next snapshot, the same as
a revocation.

## Routes snapshot contract (fleet-gateway)

Each route item gains one field:

```json
{ "namespace": "team-a", "name": "x", "path": "/mcp/team-a/x", "visibility": "team", ... }
```

`visibility` is `public` or `team`, always present in version 2.

**Versioning.** fleet-gateway refuses any `schema_version` other than 1
(`apps/fleet-gateway/fleet_control.go:175-177`) and keeps its last good or
stored snapshot. That is the fail-closed lever:

- fleet-control serves `schema_version: 1` while no route is internal, and
  `schema_version: 2` as soon as one is. A server can only be marked `Team`
  once the gateway in that cluster understands version 2 (rollout below).
- An old gateway that receives version 2 stops taking updates and alerts on
  snapshot age; it never serves an internal route as public. The cost: every
  other route change also stops reaching it until it is upgraded.
- A new gateway accepts 1 and 2. In version 2, a missing or unknown
  `visibility` is treated as `team` (fail closed), like `data_class`
  (`api/snapshot.go:24-26`).

Version-2 serving is behind a fleet-control setting
(`SNAPSHOT_SCHEMA_MAX=2`), so it cannot go out before the gateway does.

## Gateway enforcement

On a route with `visibility: team`, before proxying:

1. Resolve the caller's identity as the proxy does today (a person; an agent
   profile acts on behalf of a person).
2. Check membership of the route's `namespace` with the Nais Console API,
   reusing `probe.ConsoleTeams` (`apps/fleet-gateway/internal/probe/teams.go:29-80`)
   and the service account the gateway already has in prod.
3. Member: proxy. Not a member: 404, so the route's existence is not
   revealed. Console error or timeout: 503 with `Retry-After`, never a pass.

**Cache.** Positive and negative answers per (team, person) for one minute,
like fleet-control (`consoleTeamsTTL`, `internal/auth/console.go:24-27`). A removed member keeps access
for at most the TTL. Console errors are not cached. `probe.ConsoleTeams` has
no cache today (`teams.go:31`, marked shortcut); the cache belongs there so
both callers share it.

**Agent traffic.** For an agent profile calling on behalf of a person, the
person must be a member. An app without a person (no delegated user) is
refused on internal routes. See open question 4.

Metrics: `fleet_gateway_visibility_denied_total{reason}` (`not_member`,
`console_unavailable`), checked in Mimir before claims are made.

## my-copilot UI

- `/fleet/registrer` wizard: a switch «Bare for teamet» with help text
  «Serveren vises ikke i katalogen eller MCP-registeret for andre team, og
  gatewayen avviser brukere utenfor teamet.»
- `/fleet/mcp-servere/<ns>/<name>`: the same switch. Turning it on publishes
  at once; turning it off shows «Må godkjennes av plattformteamet» before
  saving.
- List and detail pages: an «Intern» `Tag` (Aksel) on internal servers.
- The catalogue page needs no change: it shows what the API returns.

## mcp-registry

No code change. It serves what the snapshot contains, and internal servers
are not in it. Static entries in `allowlist.json` stay public; an internal
server must not be listed there (a check in fleet-control's seed or CI).

## Tests

- `model`: `Classify` rows for both directions and the first version.
- `api`: catalogue and catalogue-entry 404 for a stranger, 200 for the owner
  team and the platform team; registry snapshot leaves internal items out;
  routes snapshot is version 1 with no internal route and version 2 with one;
  `SNAPSHOT_SCHEMA_MAX=1` refuses marking a server `Team` (409, new reason
  `VisibilityUnsupported`); grant from another team refused.
- fleet-gateway: version 1 and 2 parse; missing `visibility` in version 2 is
  `team`; member 200, non-member 404, Console error 503, cache hit skips the
  Console call; an old binary (version-1 parser) refuses a version-2
  snapshot and keeps its previous routes.
- Revert check: remove the visibility check in the proxy and see the
  non-member test fail; remove the schema bump and see the old-parser test
  fail.

## Rollout order

1. **fleet-gateway** (dev, then prod): parses version 1 and 2, enforces
   `team`. Harmless while fleet-control still serves version 1.
2. **fleet-control**: spec field, `Classify` case, catalogue and registry
   filtering, version-2 snapshot behind `SNAPSHOT_SCHEMA_MAX` (default 1).
   Set to 2 in dev, verify with one test server, then prod.
3. **my-copilot**: switch and tag, shown only when fleet-control reports
   that `Team` is supported (for example in the discovery document).

mcp-registry needs no deploy. Rollback: set `SNAPSHOT_SCHEMA_MAX=1` only when
no server is internal; otherwise roll back the gateway last, never first.

## ROS addition

New threat and control for the fleet ROS:

| Threat | Control | Residual |
| --- | --- | --- |
| Internal server served to non-members by an old gateway | schema version 2, refused by old gateways | stale routes on an un-upgraded gateway until it is upgraded |
| Console outage | fail closed, 503 | internal servers unavailable during the outage |
| Removed team member keeps access | one-minute cache | up to one minute |
| Name or existence leaked | 404 in catalogue, grants and gateway | names in audit logs and to the platform team |
| Widening through rollback | open question 1 | depends on the decision |

Visibility is not a data-protection control: an internal server still needs
the data category and ROS its tools require.

## Open questions for the owner

1. **Rollback to a public generation.** Refuse (409) a rollback that widens
   visibility, or accept it because that generation was approved once?
2. **Old gateways stop taking updates.** Is it acceptable that one internal
   server freezes route updates on any gateway still on version 1, or should
   fleet-control serve version 1 without internal routes to old gateways
   (needs the gateway to send its version)?
3. **Grants from other teams.** Refuse a grant on an internal server from a
   profile in another team, or allow it and rely on the gateway refusing the
   person?
4. **Agents without a person.** Refuse app-only calls on internal routes, or
   accept the profile's team as the member?
5. **Platform team at the gateway.** Should the platform team also pass the
   gateway check (for support), or only see the server in fleet-control?
6. **Cache TTL.** One minute like fleet-control, or longer to spare the Console API?
