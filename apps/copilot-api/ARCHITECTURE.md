# copilot-api Architecture

## Identity Resolution

The identity resolution layer answers: **"What GitHub username does this authenticated caller represent?"** — decoupled from _how_ the caller authenticated.

### Problem Solved

Before this architecture, every per-user handler contained inline branching logic:

```go
// ❌ OLD: auth mechanism leaked into every handler
if isCopilotCLI(user.AZP) {
    username = r.Header.Get("X-On-Behalf-Of")
} else {
    username, _ = github.GetUsernameBySamlIdentity(user.Email)
}
```

Adding a 3rd auth mechanism (GitHub Actions, service accounts, new CLI) required changing every handler. Handlers mixed business logic with authentication concerns.

### Design: Strategy + Chain + Middleware

```
┌─────────────────────────────────────────────────────────────────────────┐
│  Request enters /api/v1/                                                │
│                                                                         │
│  1. Auth Middleware  → validates JWT, extracts *User into context        │
│  2. Identity Middleware (required=false globally)                        │
│     → runs IdentityResolverChain.Resolve(user, request)                 │
│     → first-match-wins: on-behalf-of → SAML → ...                       │
│     → stores *ResolvedIdentity in context (or skips on failure)         │
│  3. Per-user routes: auth.RequireResolvedIdentity(chain, handler)       │
│     → wraps handler with IdentityMiddleware(chain, required=true)       │
│     → structurally blocks request if no identity resolved               │
│  4. Handler calls auth.RequireOwnership(w, r, username)                 │
│     → compares resolved identity with requested username                │
│     → rejects mismatches with 403                                       │
└─────────────────────────────────────────────────────────────────────────┘
```

### Core Interfaces

```go
// internal/auth/identity.go — one impl per auth mechanism
type IdentityResolver interface {
    CanResolve(user *User, r *http.Request) bool   // cheap, no I/O
    Resolve(ctx context.Context, user *User, r *http.Request) (*ResolvedIdentity, error)
}

// Result stored in request context
type ResolvedIdentity struct {
    GitHubUsername string  // verified GitHub login
    Source         string  // "saml", "on-behalf-of", etc. (audit trail)
}
```

### Concrete Resolvers

| File | Resolver | Condition | Resolution |
|------|----------|-----------|------------|
| `internal/auth/identity_onbehalfof.go` | `OnBehalfOfIdentityResolver` | App token (`idtyp=app` or role `access_as_application`, no NAVident or e-mail), `azp` ∈ trusted set, `GET` on a route in `onBehalfOfRoutes` | `X-On-Behalf-Of` header (format-validated) |
| `internal/auth/identity_saml.go` | `SAMLIdentityResolver` | User has non-empty email | GitHub SCIM API lookup |

**Chain order matters** — on-behalf-of is registered before SAML because a trusted M2M token typically has no email claim for SAML to resolve.

`IdentityMiddleware` answers 401 when a user token carries `X-On-Behalf-Of`, so a user token never asserts someone else. `onBehalfOfRoutes` is keyed on the ServeMux pattern. The global pass sees only the outer pattern, `/api/v1/`, so the header is honoured in the per-route `requireResolvedIdentity` alone.

### Sentinel Errors

| Error | Meaning | HTTP mapping |
|-------|---------|--------------|
| `ErrNoApplicableResolver` | No resolver matched the request | 401 |
| `ErrNoGitHubAccount` | Resolver matched but no GitHub account found (e.g. new employee) | 403 |
| `ErrIdentityHeaderMissing` | Trusted intermediary expected header but it's absent | 401 |
| `ErrInvalidIdentityHeader` | Header present but not a valid GitHub username | 401 |

Mapping lives in `writeIdentityResolutionError()` in `internal/auth/identity_middleware.go`.

### Handler Pattern

After the refactor, every per-user handler follows this 4-line pattern:

```go
func (h *Handlers) HandleUserMetrics(w http.ResponseWriter, r *http.Request) {
    username := r.PathValue("username")
    if !auth.RequireOwnership(w, r, username) { return }  // ← mechanism-agnostic

    // Pure business logic from here on...
}
```

Handlers have **zero knowledge** of SAML, X-On-Behalf-Of, or any future mechanism.

### Defense-in-Depth (Three Layers)

1. **Global `IdentityMiddleware(chain, required=false)`** — runs on all `/api/v1/` routes, resolves identity best-effort so aggregate endpoints (team dashboards, billing) aren't blocked when identity can't be determined.

2. **Route-level `auth.RequireResolvedIdentity(chain, handler)`** — wraps specific per-user routes with `required=true`, making it _structurally impossible_ for a per-user route to serve requests without a resolved identity. Applied in `router.API()` (`internal/api/router/router.go`):

   ```go
   perUser := func(h http.HandlerFunc) http.HandlerFunc {
       return auth.RequireResolvedIdentity(identityChain, h)
   }
   mux.HandleFunc("GET /api/v1/copilot/usage/user/{username}", bq(perUser(...)))
   mux.HandleFunc("GET /api/v1/copilot/seats/{username}", gh(perUser(...)))
   ```

3. **Handler-level `auth.RequireOwnership(w, r, username)`** — verifies the resolved identity matches the _specific_ requested username. Even if layer 2 ensures some identity exists, this prevents user A from accessing user B's data.

### Adding a New Auth Mechanism

1. Create a new file (e.g. `identity_githubactions.go`)
2. Implement `IdentityResolver` interface (CanResolve + Resolve)
3. Register in the chain in `main.go` (order = priority)
4. **No handler changes needed** — the chain handles dispatch automatically

```go
// Example: GitHub Actions OIDC token → extract actor from JWT claims
type GitHubActionsIdentityResolver struct { ... }
func (r *GitHubActionsIdentityResolver) CanResolve(user *User, _ *http.Request) bool { ... }
func (r *GitHubActionsIdentityResolver) Resolve(ctx context.Context, user *User, _ *http.Request) (*ResolvedIdentity, error) { ... }
```

### Input Validation

The `OnBehalfOfIdentityResolver` validates the `X-On-Behalf-Of` header against GitHub's username format (`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,37}[a-zA-Z0-9])?$`) before accepting it. This prevents a compromised or buggy trusted intermediary from injecting malformed identifiers (path separators, control characters, SQL fragments) into downstream systems.

### Testing

- **Resolver isolation**: Each resolver tested independently with mocked dependencies (`internal/auth/identity_test.go`)
- **Handler isolation**: Tests inject `ResolvedIdentity` directly into context via `identityContext()` helper — no need to mock the full SAML/GitHub API chain just to test business logic (`internal/api/copilot/github_handlers_test.go`, `internal/budget/budget_handlers_test.go`)
- **Integration**: Chain + middleware tested end-to-end with multiple resolvers wired together (`internal/auth/identity_test.go:TestIdentityMiddleware`)

---

## Router Structure

```
/health, /ready, /metrics  ← public (no auth)
/public/v1/*               ← public (no auth): videos, fact
/internal/v1/saml/name-id  ← app token from copilot-survey only; no identity
                             middleware, request log or trace
/internal/v1/github/org-membership ← app token from copilot-cli only; same
/api/v1/*                  ← protected:
  ├── Auth middleware (JWT validation)
  ├── Identity middleware (global, required=false)
  │
  ├── Aggregate routes (no ownership check needed):
  │   ├── GET /copilot/usage/metrics
  │   ├── GET /copilot/adoption/*
  │   ├── GET /copilot/billing
  │   └── ...
  │
  ├── Admin routes (auth.RequireAdmin):
  │   └── GET /admin/billing/*
  │
  └── Per-user routes (perUser wrapper + auth.RequireOwnership):
      ├── GET /copilot/usage/my-teams
      ├── GET /copilot/usage/user/{username}
      ├── GET /copilot/usage/user/{username}/weekly
      ├── GET /copilot/usage/user/{username}/daily-credits
      ├── GET /copilot/seats/{username}
      ├── POST /copilot/seats
      ├── DELETE /copilot/seats/{username}
      ├── GET /copilot/budget
      └── /copilot/budget/{options,profile,raises} (budget.RegisterRaiseRoutes)
```

## Key Files

| File | Responsibility |
|------|---------------|
| `internal/auth/identity.go` | Interface, chain, sentinel errors |
| `internal/auth/identity_onbehalfof.go` | X-On-Behalf-Of resolver (copilot-cli) |
| `internal/auth/identity_saml.go` | SAML → GitHub username resolver (my-copilot) |
| `internal/auth/identity_middleware.go` | Middleware, `RequireOwnership`, `RequireResolvedIdentity` |
| `internal/auth/identity_test.go` | Comprehensive tests for all identity components |
| `internal/api/router/router.go` | /api/v1/ router with perUser wiring |
| `main.go` | Chain construction and middleware composition |
