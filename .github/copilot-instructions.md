# Copilot Instructions for navikt/copilot

## Repository Overview

Monorepo containing NAV's GitHub Copilot ecosystem tools:

- **my-copilot** - Self-service portal for managing Copilot subscriptions (Next.js 16)
- **mcp-onboarding** - Reference MCP server with GitHub OAuth (Go)
- **mcp-registry** - Public registry for NAV-approved MCP servers (Go)

All applications deployed on NAIS platform with environment-specific configurations.

---

# Nav Development Standards

These standards apply across Nav projects. Project-specific guidelines follow below.

## Nav Principles

- **Team First**: Autonomous teams with circles of autonomy, supported by Architecture Advice Process
- **Product Development**: Continuous development and product-organized reuse over ad hoc approaches
- **Essential Complexity**: Focus on essential complexity, avoid accidental complexity
- **DORA Metrics**: Measure and improve team performance using DevOps Research and Assessment metrics

## Nav Tech Stack

- **Backend**: Kotlin with Ktor, PostgreSQL, Apache Kafka
- **Frontend**: Next.js 16+, React, TypeScript, Aksel Design System
- **Platform**: Nais (Kubernetes on Google Cloud Platform)
- **Auth**: Azure AD, TokenX, ID-porten, Maskinporten
- **Observability**: Prometheus, Grafana Loki, Tempo (OpenTelemetry)

## Nav Code Standards

### Kotlin/Ktor Patterns

- ApplicationBuilder pattern for bootstrapping
- Sealed classes for environment configuration (Dev/Prod/Local)
- Kotliquery with HikariCP for database access
- Rapids & Rivers pattern for Kafka event handling

### Next.js/Aksel Requirements

- **CRITICAL**: Always use Aksel spacing tokens, never Tailwind padding/margin
- Mobile-first with responsive props: `xs`, `sm`, `md`, `lg`, `xl`
- Norwegian number formatting with space separators

### Nais Deployment

- Manifests in `.nais/` directory
- Required endpoints: `/isalive`, `/isready`, `/metrics`
- OpenTelemetry auto-instrumentation for observability

### Writing Effective Agents

Based on [GitHub's analysis of 2,500+ repositories](https://github.blog/ai-and-ml/github-copilot/how-to-write-a-great-agents-md-lessons-from-over-2500-repositories/), follow these patterns when creating or updating agents in `.github/agents/`:

**Structure (in order):**

1. **Frontmatter** - Name and description in YAML
2. **Persona** - One sentence: who you are and what you specialize in
3. **Commands** - Executable commands early, with flags and expected output
4. **Related Agents** - Table of agents to delegate to
5. **Core Content** - Code examples over explanations (show, don't tell)
6. **Boundaries** - Three-tier system at the end

**Six Core Areas to Cover:**

- Commands (with flags and options)
- Testing patterns
- Project structure
- Code style (✅ Good / ❌ Bad examples)
- Git workflow
- Boundaries

**Three-Tier Boundaries:**

```markdown
## Boundaries

### ✅ Always
- Run `mise check` after changes
- Use parameterized queries

### ⚠️ Ask First
- Modifying production configs
- Changing auth mechanisms

### 🚫 Never
- Commit secrets to git
- Skip input validation
```

**Key Principles:**

- **Commands early**: Put executable commands near the top, not buried at the bottom
- **Code over prose**: Show real code examples, not descriptions of what code should do
- **Specific stack**: Include versions (`Next.js 16`, `Go 1.26`, `Kotlin 2.0`)
- **Actionable boundaries**: "Never commit secrets" not "I cannot access secrets"

---

# Application-Specific Guidelines

## apps/my-copilot (Next.js + TypeScript)

Self-service portal for managing GitHub Copilot subscriptions. Next.js 16 app with Azure AD authentication.

### Commands

Working directory: `apps/my-copilot`

**Run after all changes:** `mise check`

**Available tasks:**

- `mise check` - Run all checks (ESLint, TypeScript, Prettier, Knip, Jest)
- `mise test` - Run Jest tests
- `mise dev` - Start Next.js dev server (http://localhost:3000)

### Tech Stack

- Next.js 16 with App Router
- TypeScript strict mode
- Nav Design System (@navikt/ds-react)
- Tailwind CSS v4.1
- Octokit for GitHub API
- Jest for testing

### File Structure

```
apps/my-copilot/src/
├── app/              # Next.js App Router pages
│   ├── api/         # API routes for Copilot data
│   ├── usage/       # Analytics dashboard
│   └── overview/    # License management
├── components/       # Reusable React components
└── lib/             # Utilities and business logic
    ├── auth.ts      # Azure AD JWT validation
    ├── github.ts    # GitHub API client (Octokit)
    └── data-utils.ts # Data aggregation
```

### Key Patterns

**Authentication:**

```typescript
const user = await getUser(); // redirects if not auth
const user = await getUser(false); // returns null if not auth
```

**API Routes:**

```typescript
// ✅ Explicit error handling
export async function GET() {
  const { usage, error } = await getCopilotUsage("navikt");
  if (error) return NextResponse.json({ error }, { status: 500 });
  return NextResponse.json(usage);
}
```

**Number Formatting:**

```typescript
import { formatNumber } from "@/lib/format";
formatNumber(151354); // "151 354" (Norwegian locale)
```

### Spacing Rules - Mobile-First Design

**CRITICAL**: Always use Nav DS spacing tokens, never Tailwind padding/margin utilities.

```typescript
// ✅ Good - Nav DS Box with responsive padding
<main className="max-w-7xl mx-auto">
  <Box
    paddingBlock={{ xs: "space-16", md: "space-24" }}
    paddingInline={{ xs: "space-16", md: "space-40" }}
  >
    {children}
  </Box>
</main>

// ❌ Bad - Tailwind utilities
<main className="p-4 mx-4">
```

**Responsive breakpoints**: `xs` (0px), `sm` (480px), `md` (768px), `lg` (1024px), `xl` (1280px)

**Common spacing tokens**: `space-8`, `space-16`, `space-24`, `space-32`, `space-40`

### Testing

- Framework: Jest with TypeScript
- Location: `*.test.ts` files next to source
- Run: `pnpm test` in `apps/my-copilot`
- Focus on `lib/` utilities

### Boundaries

✅ Always: Use Nav DS spacing tokens, run `mise check` after all changes, TypeScript strict mode
⚠️ Ask first: Auth changes, GitHub API modifications, data aggregation changes
🚫 Never: Tailwind p-/m- utilities, commit secrets, skip type checking

---

## apps/mcp-onboarding (Go + MCP)

Reference MCP server demonstrating GitHub OAuth authentication for VS Code integration.

### Commands

Working directory: `apps/mcp-onboarding`

**Run after all changes:** `mise check`

**Available tasks:**

- `mise check` - Run all checks (fmt, vet, staticcheck, lint, test)
- `mise test` - Run tests with verbose output
- `mise dev` - Run with DEBUG logging (http://localhost:8080)

### Tech Stack

- Go with standard library
- OAuth 2.1 with PKCE
- MCP JSON-RPC protocol
- Streamable HTTP transport

### Architecture

```
VS Code (MCP Client) ←→ mcp-onboarding (OAuth + MCP Server) ←→ GitHub OAuth
```

**Flow:**
1. VS Code discovers OAuth metadata via `/.well-known/oauth-authorization-server`
2. User authenticates via GitHub
3. Server validates org membership and issues access token
4. VS Code uses token to call MCP tools

### Available MCP Tools

- `hello_world` - Returns greeting with authenticated GitHub username
- `greet` - Personalized greeting message
- `whoami` - Authenticated user information
- `echo` - Echo back message
- `get_time` - Current server time

### Configuration

| Variable               | Description                    | Default                 |
| ---------------------- | ------------------------------ | ----------------------- |
| `PORT`                 | Server port                    | `8080`                  |
| `BASE_URL`             | Public URL for OAuth redirects | `http://localhost:8080` |
| `GITHUB_CLIENT_ID`     | GitHub OAuth App client ID     | (required)              |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth App client secret | (required)              |
| `ALLOWED_ORGANIZATION` | Required GitHub org membership | `navikt`                |
| `LOG_LEVEL`            | Log level                      | `INFO`                  |

### Key Files

- `main.go` - Application entry point
- `oauth.go` - OAuth 2.1 + PKCE implementation
- `mcp.go` - MCP JSON-RPC protocol handlers
- `github.go` - GitHub API integration

### Boundaries

✅ Always: Run `mise check` after all changes, validate OAuth flow, test org membership
⚠️ Ask first: OAuth implementation changes, MCP protocol updates
🚫 Never: Commit secrets, skip security validation, bypass org checks

---

## apps/mcp-registry (Go + Public API)

Public registry service for NAV-approved MCP servers, implementing MCP Registry v0.1 specification.

### Commands

Working directory: `apps/mcp-registry`

**Run after all changes:** `mise check`

**Available tasks:**

- `mise check` - Run all checks (fmt, vet, staticcheck, lint, test)
- `mise test` - Run tests with verbose output
- `mise validate` - Validate allowlist.json by starting server
- `mise dev` - Run with DEBUG logging (http://localhost:8080)

### Tech Stack

- Go with standard library
- MCP Registry v0.1 specification
- JSON Schema validation
- Prometheus metrics

### Configuration

| Variable           | Description                       | Default                  |
| ------------------ | --------------------------------- | ------------------------ |
| `PORT`             | Server port                       | `8080`                   |
| `LOG_LEVEL`        | Log level                         | `INFO`                   |
| `LOGGED_ENDPOINTS` | Endpoints to log                  | `/,/v0.1/servers`        |
| `DOMAIN_INTERNAL`  | Internal domain for templates     | `intern.dev.nav.no`      |
| `DOMAIN_EXTERNAL`  | External domain for templates     | `ekstern.dev.nav.no`     |

### API Endpoints

| Endpoint                                          | Description                |
| ------------------------------------------------- | -------------------------- |
| `GET /`                                           | Service information        |
| `GET /v0.1/servers`                               | List all MCP servers       |
| `GET /v0.1/servers/{name}/versions/{version}`     | Get specific server        |
| `GET /v0.1/servers/{name}/versions/latest`        | Get latest server version  |
| `GET /health`                                     | Health check               |
| `GET /ready`                                      | Readiness check            |
| `GET /metrics`                                    | Prometheus metrics         |

**Note**: Server names with `/` must be URL-encoded (e.g., `io.github.navikt/github-mcp` → `io.github.navikt%2Fgithub-mcp`)

### Key Files

- `main.go` - Application entry point and routing
- `handlers.go` - HTTP request handlers
- `config.go` - Configuration and environment loading
- `validation.go` - allowlist.json validation
- `types.go` - Data structures
- `allowlist.json` - Registry data (MCP Server JSON Schema)

### Server Name Format

Reverse-DNS with exactly one `/`:

- Format: `{namespace}/{name}` (e.g., `io.github.navikt/github-mcp`)
- Pattern: `^[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9]/[a-zA-Z0-9][a-zA-Z0-9._-]*[a-zA-Z0-9]$`

### Domain Templates

Use `{{domain_internal}}` and `{{domain_external}}` in `allowlist.json` for environment-specific URLs:

```json
{
  "remotes": [{
    "type": "streamable-http",
    "url": "https://my-server.{{domain_internal}}/mcp"
  }]
}
```

### Boundaries

✅ Always: Validate allowlist.json with `mise validate`, run `mise check` after all changes, follow MCP spec
⚠️ Ask first: Registry format changes, validation rules, API endpoints
🚫 Never: Bypass security validation, skip schema validation, modify spec compliance

---

# Monorepo Navigation

When working across applications:

1. **Check context**: Determine which app the request relates to
2. **Change directory**: Use `cd apps/{app-name}` before running commands
3. **Use mise tasks**: All apps support `mise` commands (preferred)
4. **Verify changes**: Always run `mise check` after making changes
5. **Reference paths**: Use workspace-relative paths from repo root

**Example workflow:**

```bash
# Working on my-copilot
cd apps/my-copilot
mise check

# Working on mcp-registry
cd apps/mcp-registry
mise validate       # validate allowlist.json
mise check

# Back to root
cd ../..
```

---

# Build and Verify

**IMPORTANT**: After making any code or documentation changes, run `mise all` from the repository root to build and verify all applications:

```bash
mise all
```

This runs all checks across all applications to ensure nothing is broken.
