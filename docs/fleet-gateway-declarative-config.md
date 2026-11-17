# fleet-gateway: declarative, API-driven route and policy configuration

Design, 2026-11-17. Status: the four owner decisions of 2026-11-17 are applied (section 7);
the rest is a proposal. Claims marked **(unverified)** have not been checked against a source yet.

Related: [MCP gateway plan](mcp-gateway-plan.md), [#2331](https://github.com/navikt/copilot/issues/2331)
(Level 1), [#2332](https://github.com/navikt/copilot/issues/2332) (Level 3),
[#2294](https://github.com/navikt/copilot/issues/2294) (kill switch without deploy),
[#2287](https://github.com/navikt/copilot/issues/2287) (dynamic registry).

## Summary

- **Mechanism: (d) hybrid, in a separate control service (decided).** The control service owns
  versioned `Route`, `RouteOverride` and `GatewayPolicy` documents in its own Nais Postgres, behind
  an API that takes the whole spec on `PUT`. The first client is a YAML file in the team's own repo,
  applied from CI with GitHub OIDC. Humans use the my-copilot admin pages. A CRD can be added later
  as another client without changing the model.
- **One source of truth.** The gateway's route table and the registry entry for a gateway route are
  both projections of the same `Route` document. mcp-registry is a consumer: it reads the
  projection and nobody writes a gateway entry by hand. `allowlist.json` keeps servers that are not
  behind the gateway.
- **Gateway consumes a compiled snapshot** by polling (ETag, about 15 s, Texas m2m token),
  re-validates it, keeps the last good snapshot in memory and in its own Postgres, and fails static
  when the control service is down, with an alert on snapshot age.
- **Policy is evaluated in the gateway process** by a small fixed evaluator (tool name, principal
  type, client, optionally group). No OPA or Cedar now.
- **Level 1 is the first slice of this, not a detour**, if the file gets the final field names and
  the gateway reads routes through a `RouteSource` interface from day one. Existing routes without
  `tools` stay pass-all; every new route must declare `tools`.
- **Group rules wait for a spike.** Until groups in the team OBO token (and overage) are proven,
  Level 3 rules use tool, principal type and client only.
- One thing an API cannot remove: a new upstream needs an `accessPolicy.outbound` rule on the
  gateway and an inbound rule on the team app. Route *creation* still involves a deploy on both
  sides. Everything after creation (tools, rules, kill switch, audience, exposure) can be API-only.

## 1. What exists today (read from the repo)

| Thing | Where | Notes |
|---|---|---|
| Route list | `apps/fleet-gateway/pilot-routes.{dev,prod}.yaml`, parsed in `routes.go` | `route`, `team`, `target` (`cluster:ns:app`), `upstream_url`, `enabled`. Strict decoding (`KnownFields`), upstream must be `<app>.<ns>` of the target namespace or an internal Nav host. Baked into the image, chosen by `PILOT_ROUTES_FILE`. |
| Kill switch | `enabled: false` per route, `GATEWAY_DISABLED` globally | Both need a deploy (#2294). |
| Network and Entra | `.nais/app.yaml` + `{dev,prod}-gcp.yaml` | `outbound_rules` lists every upstream app. No `claims.groups` on the gateway app. |
| Client policy | env `ALLOWED_CLIENT_IDS`, `ALLOWED_REDIRECT_URIS`, `DCR_ENABLED` | Global, not per route. |
| Session | `authorize.go` | Stores NAVident and a `Groups` list taken from the Entra token at login. With no `claims.groups` in the manifest the list is probably empty in practice (not checked against a live token). |
| Team token | `team_token.go` | Texas OBO per call, scope `api://<cluster>.<ns>.<app>/.default`. |
| Audit | `proxy.go`, `audit mcp` line | route, NAVident, JSON-RPC method; tool name comes with #2309. Body is capped at 1 MiB and forwarded byte for byte. |
| Registry, static | `apps/mcp-registry/allowlist.json` | `_meta."io.github.navikt/registry"`: `tools`, `toolRisk`, `owner`, `gateway`, `toolHashes` (not enforced), `audience.teams` (#2310). |
| Registry, dynamic | `write_handlers.go`, `migrations/001_init.sql` | `registry_entries` (pending/approved/rejected), `registry_entry_versions`, `registry_audit`. Writes only with `azp` = my-copilot; admins through an Entra group in the `groups` claim from Texas introspection. Store reloads every 60 s. |

Drift that exists already: a gateway route and its registry entry are two hand-written files in two
apps. Nothing checks that `gateway`, `owner` or `tools` in `allowlist.json` match a route in
`pilot-routes.prod.yaml`.

## 2. Resource model

### 2.1 Keep it to three objects

Gateway API splits route and policy into separate objects because different roles own them. Here
the same team owns the route, the tool rules and the audience, so one document per route is enough.
Split only where the *owner* differs.

| Object | Owner | Purpose |
|---|---|---|
| `Route` | the team | Everything a team declares about one MCP server behind the gateway. |
| `RouteOverride` | gateway operators | Operator state the team must not be able to undo: approval, operator kill switch, approval of external exposure and of agent apps. |
| `GatewayPolicy` | gateway operators | Global settings: global disable, allowed clients (CIMD list, DCR on/off), default limits. One per environment. |

`Policy/ToolRule`, `ClientPolicy`, `AgentGrant`, `Exposure` and `Audience` are **sections of
`Route`**, not objects. They get promoted to objects only if a real need for separate ownership or
reuse across routes appears.

### 2.2 `Route`

```yaml
apiVersion: fleet-gateway.nav.no/v1
kind: Route
metadata:
  name: onboarding            # route is /mcp/<name>; unique per environment
  team: copilot               # Nais team; immutable after creation without operator action
  environment: prod           # dev | prod; one document per environment
spec:
  enabled: true               # the team's own kill switch
  upstream:
    target: prod-gcp:copilot:mcp-onboarding   # cluster:namespace:app, namespace must belong to team
    path: /mcp/gw                             # URL is derived, not free text
  exposure: internal          # internal | external (#2297); external needs operator approval

  tools:                      # Level 1: allowlist. Absent = see section 4.3
    - get_onboarding_status
    - list_checklists

  groups:                     # Level 3: named aliases, so rules do not repeat GUIDs
    saksbehandlere: 00000000-0000-0000-0000-000000000000
  rules:                      # Level 3: first match wins, no match = deny
    - tools: [list_*]
      allow: { principals: [user, delegated-agent] }
    - tools: [update_checklist]
      allow: { principals: [user], groups: [saksbehandlere] }

  clients:                    # optional narrowing of GatewayPolicy; cannot widen it
    allow: []                 # empty = every client the gateway allows
    dcr: true
  agents:
    delegated: []             # azp (cluster:ns:app) allowed to call on behalf of a user (#2304)
    autonomous: []            # allow_apps for app tokens (#2305); needs operator approval

  registry:                   # what is published; everything else stays private
    title: ...
    description: ...
    audience: { teams: [copilot] }   # visibility only (#2289)
    toolRisk: { update_checklist: write }
    examples: []
status:                       # written by the control service, never by the team
  generation: 7
  phase: active               # pending | active | rejected | disabled
  conditions: [...]           # e.g. OboProbe=ok, RegistryPublished=ok
```

Field rules worth fixing early:

- `name`, `team`, `environment` are identity. A change of `team` or of the namespace in
  `upstream.target` is a new registration, not an update.
- `upstream` is `target` + `path`. The URL is derived (`http://<app>.<ns><path>`), which removes
  the free-text `upstream_url` and the validation that guards it today.
- `tools` and `rules` are alternatives. `tools: [a, b]` is shorthand for one rule that allows
  `a` and `b` for every principal the route admits. A document has one or the other.
- `spec.registry` is an explicit list of public fields. Group IDs, agent apps and client lists are
  never in the public projection.

### 2.3 `RouteOverride` and `GatewayPolicy`

```yaml
kind: RouteOverride
metadata: { name: onboarding, environment: prod }
spec:
  approved: true              # first registration
  disabled: false             # operator kill switch; wins over spec.enabled
  disabledReason: ""
  exposureExternalApproved: false
  autonomousAppsApproved: []
```

Effective state = `Route` ∧ `RouteOverride` ∧ `GatewayPolicy`. A team apply never touches the
override, so a CI job that re-applies the team's file cannot re-enable a route an operator closed.

### 2.4 One source of truth

- The gateway's route table = effective state of all active `Route`s for its environment.
- The registry entry for a gateway server, as served by mcp-registry = projection of `Route.spec.registry` + the remote URL
  derived from `name`, `exposure` and the environment's hosts + `owner` = `team` +
  `tools` = the allowlist. `allowlist.json` keeps only servers that are *not* behind the gateway
  (external servers, PR-reviewed as decided in #2093/#2096).
- Result: `gateway`, `owner`, `tools` and the URL cannot drift, because nobody types them twice.

## 3. Mechanism options

### 3.1 Comparison

| | (a) CRD + controller | (b) REST API only | (c) Config in team repo, pushed by CI | (d) Hybrid: spec in Postgres behind an API; file/CRD are clients |
|---|---|---|---|---|
| Ownership / authz | Kubernetes RBAC: the team can write in its own namespace. Strongest and free. | Entra group or GitHub OIDC; we must map caller to team ourselves. | Same as (b); CI is the caller. | Same as (b) for the file client; same as (a) if the CRD client is added. |
| Audit | Kubernetes audit log (Nais-owned) + team's git history. | Our own audit table. | Team's git + our audit table. | Our audit table + version rows (pattern exists in mcp-registry). |
| Validation | OpenAPI schema + CEL; cross-object checks need a webhook or status conditions (async feedback). | Synchronous 4xx with a message. | Synchronous in CI: the deploy job fails. | Synchronous; one validator for every client. |
| Drift | Reconciled continuously. | None between store and gateway; drift against the team's repo if someone uses the API by hand. | Repo vs store can drift until next CI run. | Store is the truth; file client applies whole documents (PUT), so the next run corrects it. |
| Rollout / rollback | `nais deploy`; rollback = redeploy old manifest. | PUT an old version. | Revert commit. | Revert commit, or `POST .../rollback` to a stored version. |
| Gateway consumption | Informer/watch on the CRD cluster-wide: the gateway needs cluster-wide read RBAC, or the controller writes into a store anyway. | Poll or push from the store. | Same as (b). | Poll a compiled snapshot. |
| Kill switch latency | Seconds for watch; but needs `kubectl`/deploy access for the operator. | Poll interval. | CI run (minutes) unless there is also an API. | Poll interval (15 s) through the override. |
| Effort | High: CRD, controller, RBAC, install per cluster, upgrade path. | Medium. | Small on top of (b). | Medium; one more app, reusing the registry patterns for versions, audit, approval and Texas introspection. |
| Dependency on others | Nais platform team must install and accept a CRD they do not own, in every cluster. | None beyond what we run. | None. | None for the first clients. |
| "Fully API-driven and declarative" | Declarative yes; the API is the Kubernetes API. Admin UI and non-Kubernetes callers need a second path. | API yes; declarative only if the API is document-shaped (PUT whole spec). | Yes, with (b) underneath. | Yes. |

### 3.2 Why (d), in a separate control service

1. It needs nothing from another team to start.
2. It is the only option where the admin UI, nav-pilot, a CI job and a later CRD all hit the same
   validator and the same audit trail.
3. The API is document-shaped (`PUT /api/v1/routes/{env}/{name}` with the whole spec, server-side
   `generation`, `If-Match` for concurrent writers), so it is declarative without Kubernetes.
4. A CRD stays possible. A controller would be a thin client that PUTs the document and writes
   `status` back. Decide it when a team or Nais asks; the model above is already CRD-shaped
   (`apiVersion`/`kind`/`metadata`/`spec`/`status`).

**Why a separate service and not mcp-registry (owner decision 2026-11-17).** mcp-registry is
public-facing catalogue data; route documents decide where user traffic with OBO tokens goes. The
two have different readers, different write rules and different blast radius, so they get separate
apps and databases.

| | Separate control service (decided) | Tables in mcp-registry |
|---|---|---|
| Cost | One more app, database, alert set and on-call surface. | Nothing new to run. |
| Blast radius | A registry bug or exposure cannot touch route config, and the reverse. | Registry and gateway share fate on every write. |
| Access | Only the gateway, my-copilot, team CI and mcp-registry talk to it; no public read path. | Route data sits next to a public read API. |
| Hardening | Built for its job from the start. | Registry needs the hardening in [#2215](https://github.com/navikt/copilot/issues/2215) first. |

What the control service reuses from mcp-registry, as patterns and code, not as a shared database:

- version rows per document (`registry_entry_versions`) and an append-only audit table
  (`registry_audit`);
- pending/approved/rejected state for first registration;
- write auth through Texas introspection: `azp` = my-copilot for humans, admins through the Entra
  group in the `groups` claim;
- strict decoding and one validator shared by every write path;
- migrations layout (`migrations/001_init.sql`) and the store's periodic reload.

mcp-registry reads the control service's public projection of gateway routes (title, description,
`audience`, `toolRisk`, examples, `owner`, remote URL, `tools`) and serves it next to the
`allowlist.json` entries. Group IDs, agent apps and client lists never leave the control service.

Why not (a) first: the gateway is not a Nais platform component, and a CRD is cluster-wide
surface the Nais team would have to install, version and support. It also does not cover the admin
UI or the kill switch for operators without cluster access. It is the best *ownership* model, which
is why it stays on the table as a client.

### 3.3 Team authentication for writes

**Decided 2026-11-17:** GitHub OIDC from the team's repo, with the repo bound to the route at
first operator approval, plus the my-copilot admin UI for humans (approvals, kill switch, manual
edits). The options considered:

| Option | How | Trade-off |
|---|---|---|
| GitHub OIDC from the team's repo | CI requests an OIDC token with our audience; the API verifies issuer, `repository`, `ref` (default branch) and maps repo to team. | Declarative from the repo, no secrets. Needs a repo-to-team mapping: either look it up in the Nais API (**unverified** that we can get a service credential for this) or bind the repo to the route at first approval. |
| Entra user via my-copilot | Existing write path (`azp` = my-copilot, NAVident). Team membership from an Entra group or Nais team lookup. | Works today for humans; right for the kill switch and for approvals. Not GitOps. |
| App token from the team's own app (Texas m2m) | Team app calls the API as itself; `azp` proves `cluster:ns:app`. | Proves namespace ownership through Nais itself. Odd fit for config, but strong for one thing: see below. |

Proof of ownership that already exists and costs nothing: OBO to the team app only works when the
team has put fleet-gateway in *its own* `accessPolicy.inbound`. So "the team that owns
`ns:app` consents to this route" is already enforced by Nais. The control service can run an OBO or
m2m probe at registration and record `OboProbe=ok` in `status`. A route pointing at someone else's
app fails that probe. What it does not prove is that the *caller of the API* belongs to the team;
that is what the repo binding or group check is for.

### 3.4 How the gateway consumes it

- **Pull, not push.** `GET /internal/v1/gateway-snapshot?env=prod` on the control service returns every
  effective route, pre-validated and compiled, with an `ETag`. The gateway polls every 15 s with
  `If-None-Match`. Auth: Texas m2m token, `azp` = fleet-gateway. One inbound rule on the
  control service and one outbound rule on the gateway.
- Why not push or watch: the gateway's Postgres goes through PgBouncer in transaction mode (see
  README), so `LISTEN/NOTIFY` is out, and the two apps have separate databases. Two replicas and a
  15 s poll is cheap and has no fan-out logic to get wrong.
- **The gateway re-validates the snapshot itself** with the checks in `routes.go` (path shape,
  internal upstream, namespace match). A compromised or buggy control service must not be able to
  point a route at a public host. The snapshot is applied atomically (swap a pointer); a snapshot
  that fails validation is rejected whole and the old one stays.
- **Fail static.** Last good snapshot stays in memory, and is written sealed to the gateway's own
  Postgres (`SessionStore` already seals values) for cold starts. If there is no stored snapshot
  either, the embedded file is the bootstrap. Snapshot age is a metric with an alert; the gateway
  does not close routes because the control service is slow or down (decided 2026-11-17).
- **Break-glass stays.** `GATEWAY_DISABLED` by deploy still works when the control service is down.
- Kill switch latency: override write + up to one poll interval, about 15 s worst case, against a
  full deploy today. #2294 asks for the deploy path to be measured first; that number is not in
  the issue yet.

### 3.5 What an API cannot make dynamic

- **Network policy.** Every upstream is an `outbound_rules` entry in the gateway manifest, and the
  team app needs fleet-gateway in its inbound rules. Both are Nais manifests and both need a
  deploy. Options: accept that creating a route includes one generated PR to the gateway manifest;
  reach upstreams through their internal ingress instead of service DNS (then it is
  `outbound.external` hosts, still a manifest); or ask Nais whether a broader outbound pattern
  exists (**unverified**; none known). Recommendation: accept it; creating a route is rare and
  deliberate, and CI can fail a `Route` apply whose upstream is not yet in the manifest
  (`status.conditions: NetworkPolicy=missing`).
- **Entra pre-authorization** follows from the team's inbound rule, same deploy.
- **Ingress hosts** for external exposure (#2297) are gateway manifest, one-time.

### 3.6 Prior art (from knowledge)

- **Nais `accessPolicy`**: the team declares inbound/outbound per app in its own `Application`;
  operators turn it into NetworkPolicy and Entra pre-authorized apps. Both sides must agree for
  traffic to flow. This two-sided consent is the pattern to copy, and section 3.3 reuses it.
- **Nais Kafka `Topic`** (**unverified** in detail): a CRD in the team's namespace where the owning
  team lists `acl` entries (team, application, read/write). Closest Nais precedent for "the owner
  declares who may do what" as a team-owned resource. It is the model for a CRD client, if one
  comes.
- **`nais deploy` with GitHub OIDC** (**unverified** in detail): the deploy service accepts a
  workflow's OIDC token and checks that the repository is authorized for the team. Same check we
  would want for the file client.
- **Gateway API `HTTPRoute` + policy attachment**: route owned by app teams, gateway by platform,
  policies attached by reference with a defined precedence. Useful idea: operator override beats
  team spec, which is `RouteOverride`. Not needed: separate policy objects.
- **Istio `AuthorizationPolicy`**: `action`, `rules[].from/to/when`, deny-overrides-allow, and
  "an ALLOW policy present means default deny". That last semantic is what section 4.3 uses.
- **OPA/Rego and Cedar**: general policy engines. Cedar's principal/action/resource shape maps
  well to user/tool/route, and a Go implementation exists (**unverified** how mature). See 4.1.

## 4. Policy evaluation

### 4.1 Where

In the gateway process, in Go. The rules are lookups: is this tool name in a set, is the
principal type allowed, does the group set intersect. That is a few dozen lines plus tests. The
gateway runs with a 64Mi memory limit and two replicas; an OPA sidecar would cost more than the
gateway itself and add a failure mode on every call. Cedar or OPA becomes worth it only if rules
need conditions on tool *arguments* or teams need to write logic. Arguments are deliberately never
read into anything the gateway logs or decides on today; argument-level rules should stay
with the team for good.

Keep the door open cheaply: the evaluator takes a plain input struct and returns a decision and a
rule id. Swapping the engine later touches one function.

### 4.2 Inputs

| Input | Source | Available today |
|---|---|---|
| Route | request path | yes |
| NAVident | gateway session | yes |
| Principal type (`user`, `delegated-agent`, `app`) | `idtyp` and `azp` of the token | user only; agents come with #2304/#2305 |
| Client id | session (CIMD URL or DCR id) | yes |
| Agent `azp` | token | with #2304/#2305 |
| JSON-RPC method and tool name | request body, parsed as in #2309 | with #2309 |
| Groups | see below | not in a usable form |

**Groups are the hard input.** Decided 2026-11-17: spike first (option 4 below, with overage
handling); if the OBO token does not work, Graph lookup (option 3). Until one is proven, Level 3
rules use tool, principal type and client only.

- The gateway session takes `groups` from the Entra token at login, but the gateway manifest sets
  no `claims.groups`. Nais appears to emit only the groups listed there (**unverified**), so
  the list is empty today, and every new team group would be a gateway manifest change and deploy.
  That contradicts "API-driven".
- If instead all security groups were emitted, users with more than about 200 groups get the
  overage claim (`_claim_names`) and no `groups` array at all (**unverified** exact limit). A rule
  would then deny a user who is in the group. Any design that reads `groups` from a token must
  treat a missing claim as "unknown", not "no groups".
- The team's OBO token carries the groups the *team* listed in its own manifest. That is why
  "teams authorize" works today without the gateway knowing any group.

Options for group rules at the gateway:

1. **Do not evaluate groups at the gateway.** Group rules are documentation plus team enforcement.
   Honest, zero cost, but it is not Level 3.
2. **Generate `claims.groups` on the gateway from the declared groups.** Works with Nais as is,
   costs a deploy per new group and relogin for users.
3. **Graph lookup** (`checkMemberGroups`) with an app permission on the gateway, cached per
   session. API-driven, no overage problem, but needs an Entra admin grant outside what Nais
   provisions.
4. **Read `groups` from the OBO token** the gateway already fetches per call. The team's manifest
   stays the source of which groups are emitted, nothing new to deploy on the gateway. Caveats: the
   gateway reads a token addressed to another audience, and overage applies the same way. Needs a
   spike before anyone relies on it.

### 4.3 Default deny or allow

- Route with neither `tools` nor `rules`: everything is forwarded, as today. Needed so the pilot
  route does not break on the day the feature ships.
- Route with `tools` or `rules`: default deny. A `tools/call` for a name that is not allowed gets
  a JSON-RPC error from the gateway and is never forwarded. Audit line gets `decision=deny` and
  the rule id.
- Every new route must declare `tools` (decided 2026-11-17), from Level 1 in the file onwards.
  Pass-all is only for routes that existed before the decision.
- Methods other than `tools/call` (initialize, `tools/list`, resources, prompts) pass in Level 1.
  Filtering the `tools/list` *response* means parsing a possibly streamed response the gateway
  forwards untouched today; do it later, and until then clients may see a tool they get denied on.
- Batches: deny the whole request if any call in it is denied. Simple, and never half-executes.
- Unknown principal type or missing input for a rule that needs it: deny.

### 4.4 How "teams authorize" still holds

The gateway enforces what the team declared, nothing more. The declaration is the team's decision,
written by the team, in the team's repo. The gateway adds no rule of its own beyond authentication,
route binding and operator overrides. The team MCP still receives an OBO token with the user's
identity and must still check it: the gateway allowlist is a coarse outer gate, not a replacement.
The team guide should say this in one sentence, because a tool allowlist reads like access control
and a team could be tempted to drop its own check.

## 5. Migration without breaking the pilot

| Phase | What | Not throwaway because |
|---|---|---|
| **0. Level 1 in the file** (now) | `tools: [...]` on `PilotRoute`. Enforcement on `tools/call` on top of the #2309 parser. `decision` and rule id in the audit line. Routes go through a `RouteSource` interface with one implementation, the file. Internally the route is already the `Route.spec` shape; bump the file to `version: 2` with the final field names (`upstream.target`, `upstream.path`, `tools`, `enabled`) and keep reading `version: 1`. | The evaluator, the schema, the validation and the audit fields are the final ones. Only the loader is replaced. |
| **1. Drift guard** | A test in the repo: every route in `pilot-routes.prod.yaml` has an `allowlist.json` entry whose `gateway`, `owner` and `tools` match. | Cheap, catches today's drift, and becomes the projection test in phase 2. |
| **2. Control service, read path** | New control service with its own Nais Postgres: `routes`, `route_overrides`, `gateway_policies`, version and audit tables on the registry pattern. Seeded by importing the two YAML files. Snapshot endpoint for the gateway, projection endpoint for mcp-registry. Gateway gets a second `RouteSource` (poll + last good snapshot). **Shadow first**: the gateway loads both, serves from the file, logs any difference. Then flip per environment, dev first. `RouteOverride` + operator kill switch in the my-copilot admin pages. | Closes #2294 with the real mechanism. mcp-registry consumes the projection, so the registry entry for gateway servers stops being hand-written (first half of #2287). |
| **3. Team write path** | On the control service: `PUT /api/v1/routes/{env}/{name}`, `GET`, `rollback`. File client: a small CI step that applies `fleet-gateway.yaml` from the team's repo with GitHub OIDC. First registration binds the repo to the route; first registration, external exposure and autonomous apps need operator approval; tool and audience changes are self-service. `pilot-routes.*.yaml` shrinks to the bootstrap route. | This is the declarative API. Admin UI, nav-pilot and a later CRD are clients of it. |
| **4. Level 3** | `rules` on tool, principal type and client. `groups` only after the spike proves a group source (decision 3). `agents.delegated`, then `agents.autonomous` (#2304, #2305). `exposure` (#2297). | Each is a new section of the same document and a new input to the same evaluator. |
| **5. Optional** | CRD client, `tools/list` filtering, `toolHashes` enforcement. | Only on demand. |

Compatibility rules through all phases: a route that existed before 2026-11-17 without `tools` behaves as today; the embedded
file keeps working as bootstrap; the gateway never requires the control service to start.

## 6. Risks

1. **The control service becomes security-critical.** Whoever can write a `Route` can decide where
   user traffic with OBO tokens goes. Mitigations: the gateway re-validates every snapshot, team
   and namespace are immutable, the OBO probe proves the upstream team consents, first registration
   is approved by an operator.
2. **A tool allowlist looks like more protection than it is.** Tool names are chosen by the
   upstream server, which can rename a tool or change what it does. Until `toolHashes` is enforced
   the allowlist limits surface, not behaviour. Teams must keep their own checks.
3. **Group rules that silently deny or silently pass.** Overage or an unlisted group gives an
   empty claim. Treat missing as unknown, and do not ship group rules before the source is proven.
4. **Stale configuration.** Fail static means a route closed in the store stays open on a gateway
   that cannot reach the store. Mitigated by the age alert and the deploy break-glass; not removed.
5. **Building too much.** Three objects, one evaluator, one API. Every further object or engine
   should wait for a team that asks for it.

Smaller: the projection to mcp-registry must not leak group IDs or agent app names (explicit field
list); the control service is one more app to run, patch and alert on.

## 7. Decided

Owner decisions 2026-11-17, also in the decisions log of the [MCP gateway plan](mcp-gateway-plan.md):

1. **Source of truth: a separate control service**, not mcp-registry. It owns versioned `Route`,
   `RouteOverride` and `GatewayPolicy` documents in its own Nais Postgres behind a PUT-whole-spec
   API. The gateway polls its compiled snapshot (ETag, about 15 s, Texas m2m), re-validates and
   fails static. mcp-registry consumes the projection; `allowlist.json` keeps non-gateway servers.
   Trade-off: one more service to run, for clean separation (section 3.2).
2. **Team write auth: GitHub OIDC from the team's repo**, with the repo bound to the route at first
   operator approval, plus the my-copilot admin UI for humans.
3. **Groups: spike first.** Test groups in the team OBO token and overage handling; fall back to
   Graph. Until proven, Level 3 rules use tool, principal type and client only.
4. **Routes without `tools`:** existing routes stay pass-all; new routes must declare `tools`. On
   control-service outage the gateway fails static, with an alert on snapshot age.

Still open: **a deploy at route creation.** Options: accept one generated PR to the gateway
manifest per new upstream; move upstreams to internal ingresses; ask Nais for a broader outbound
pattern. Recommendation: accept it as the one non-API step and raise the question with Nais without
blocking on it.

## 8. Not checked

- No live token was inspected: the claims about `groups` on the gateway session and on OBO tokens
  are from code and memory.
- Nais behaviour marked unverified above was not looked up in `doc.nais.io`.
- Effort is relative (small/medium/high), not estimated in days.
- #2309 and #2310 are open PRs; the analysis assumes both merge as described.
