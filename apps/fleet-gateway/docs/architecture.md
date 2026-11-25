# fleet-gateway architecture

fleet-gateway is an OAuth 2.1 authorization server in front of Entra ID and an
MCP reverse proxy for the routes in `pilot-routes.*.yaml`. This page records the
current package layout, the target layout and the order of the moves (#2926).

## Current structure

About 6,500 lines of non-test Go sit in `package main`, almost all as methods on
`*Gateway`:

- `main.go`: `Gateway`, `newGateway`, store wiring, the mux, logging and rate-limit
  middleware, health handlers and the `lockout` subcommand.
- `authorize.go`, `consent.go`, `token.go`, `dcr.go`, `prm.go`: the OAuth
  endpoints, the Entra round trip, CIMD fetch, consent and DCR.
- `team_token.go`: OBO and refresh strategies for upstream team tokens.
- `proxy.go`, `inflight.go`: the MCP proxy, SSE streaming and concurrency slots.
- `routes.go`, `routetable.go`, `fleet_control.go`: route loading, validation and the route table.
- `store_gateway.go`, `session.go`: typed wrappers and aliases over `internal/session`.
- `config.go`: env parsing, validation and the generic `ttlMap`.
- `lockout.go`: a thin shim over `internal/lockout`.

`internal/` already holds `audit`, `config` (env helpers), `identity`, `lockout`,
`policy` and `session`.

## Target layout

Each package owns one domain. Interfaces live at the consumer, and only where
there are two implementations or a test seam that tests use.

| Package | Owns | Key type or interface |
|---|---|---|
| `internal/ratelimit` | Per-IP limiter, rate-limit log sampler, client IP from the trusted ingress | `Limiter`, `LogSampler`, `ClientIP` |
| `internal/routing` | `routing.Route`, YAML loading and validation, tool policy, `routing.Table` | `Load`, `NewTable`. Fleet-control already supplies routes in dev (#2641, `fleetControlRoutes` and `swapRoutes` in `main`); no interface, since the two sources have different shapes |
| `internal/inflight` | Global, per-user and per-route concurrency slots | `Slots`, `PerKey` |
| `internal/ttlmap` | Bounded generic TTL map | `Map[V]` |
| `internal/session` | Store interface, memory and Postgres stores, sealing, key ring (exists) | `Store` |
| `internal/oauth` | Typed records over the store: pending auth, codes, sessions, consent. Refresh grace and DCR clients move with `oauth/server` | `PendingAuth`, `IssuedCode`, `Session`, `HashToken`; functions take a `session.Store` |
| `internal/oauth/entra` | Entra authorize endpoint, code and refresh redemption, ID-token checks, state cookie signing | `Client`, `SignState`, `VerifyState`, `ValidateIDTokenClaims` |
| `internal/teamtoken` | OBO and refresh strategies | `OBO`, `Refresh`. Their interface, `TeamTokenSource`, stays at the consumer in `main` until `proxy` moves |
| `internal/policy`, `internal/lockout`, `internal/audit`, `internal/identity` | As today | `lockout.Manager` |
| `internal/metrics` | Collectors and `Observe*` helpers, `Init`, the `/metrics` server | none |
| `internal/proxy` | MCP handler, header allowlists, SSE streaming | defines the team-token interface it consumes; uses `routing` |
| `internal/httpx` | `WriteJSON`, `StatusWriter`, `MarkShed`. Error pages and security headers move with `oauth/server` | none |
| `main` | Config loading, `newGateway` wiring, the mux, the `lockout` subcommand | none |

The OAuth handlers (`authorize`, `token`, `revoke`, `register`, consent, PRM) stay
in `main` until `oauth` and `entra` exist. They then move as one package
`internal/oauth/server`, because they share `pendingAuth`, the cookie key and the
redirect allowlist.

## Import-cycle risks

- `metrics` takes a `routing.Route`. `routing` must not import `metrics`; metrics
  imports `routing` or takes the route name and app as strings.
- `Config` is read everywhere. Packages take their own small option structs, built
  in `main`; none of them imports `main`'s `Config`.
- `teamtoken` reads the route table and the store. It imports `routing` and
  `session`; `routing` imports neither.
- `markShed` writes a reason onto `statusResponseWriter`. Both belong in `httpx`, so
  `proxy` and `main` share them without a cycle.
- `lockout` already imports `session`. `oauth` may import `lockout`, never the
  other way round.

## Order of moves

One package per PR, each behaviour-neutral with the same test count:

1. `ratelimit`, together with this page.
2. `routing` and the route table.
3. `inflight` and `ttlmap`.
4. `metrics` and `httpx`.
5. `oauth` store records (from `store_gateway.go` and `session.go` aliases).
6. `oauth/entra`, then `teamtoken`.
7. `proxy`.
8. `oauth/server` handlers; `main` keeps wiring only.

A separate PR cleans comments under the AGENTS.md comment rule. Correctness or
security findings go to their own issues, not into the moves.
