# AGENTS.md — navikt/copilot

Monorepo containing Nav's GitHub Copilot ecosystem tools:

- **my-copilot** — Self-service portal for managing Copilot subscriptions (Next.js 16, TypeScript)
- **copilot-api** — Backend API for Copilot data and seat management (Go)
- **copilot-metrics** — Naisjob that populates BigQuery with daily Copilot usage metrics (Go)
- **mcp-onboarding** — Reference MCP server with GitHub OAuth (Go)
- **mcp-registry** — Public registry for Nav-approved MCP servers (Go)

All apps deployed on NAIS (Kubernetes on GCP).

**Security architecture documented in [SECURITY.md](SECURITY.md)** — read before modifying auth, network policies, or secret management.

## Build & Test Commands

From repo root:

```bash
mise check    # Lint + type check + test all apps
mise test     # Run all tests (verbose)
mise build    # Build all apps
mise all      # Full pipeline: generate → check → build
```

Per-app (run from `apps/<name>/`):

```bash
# Go apps (copilot-api, mcp-onboarding, mcp-registry)
mise check    # fmt, vet, staticcheck, golangci-lint, test
mise test     # go test -v ./...
mise build    # go build

# Next.js app (my-copilot)
mise check    # ESLint, TypeScript, Prettier, Knip, Vitest
mise test     # pnpm test (Vitest)
mise build    # next build
```

## Project Structure

```text
apps/
  copilot-api/        # Go backend API — GitHub API, BigQuery, seat management
  mcp-onboarding/     # Go MCP server — OAuth, 16 tools, readiness assessment
  mcp-registry/       # Go registry API — allowlist.json, MCP Registry v0.1 spec
  my-copilot/         # Next.js 16 — App Router, Aksel Design System (BFF)
    src/
      app/            # Routes and API handlers
      components/     # React components
      lib/            # Utilities, auth, backend API client
.github/
  instructions/       # Scoped Copilot instructions (*.instructions.md)
  agents/             # Custom Copilot agents (*.agent.md)
  prompts/            # Reusable prompt templates (*.prompt.md)
  skills/             # Domain knowledge packages (SKILL.md)
  copilot-instructions.md  # Global Copilot instructions
docs/                 # Documentation
SECURITY.md           # Security architecture, trust zones, auth flow
```

### Customization Language

- **YAML descriptions**: Norwegian (shown in VS Code agent/skill picker and on the my-copilot website)
- **Machine instructions** (operating loops, rules, boundaries, checklists): English — maximizes LLM instruction adherence, especially in multi-turn conversations
- **User-visible output templates** (phase headers, progress indicators, checkpoint summaries): Norwegian — UX matters for Nav developers
- **Domain content** (decision trees, reference tables, code examples): English
- **Exceptions**: `@forfatter` and `@accessibility-agent` use Norwegian body content per their domain (UI/UX, accessibility, Norwegian language)

## Code Style

### Minimal Editing

When fixing a bug or implementing a feature, change only what is necessary. Do not rename variables, restructure working code, or refactor beyond the task at hand. Keep diffs small and focused so they are easy to review.

### Go (copilot-api, mcp-onboarding, mcp-registry)

- Standard library preferred — minimal dependencies
- `go vet` + `staticcheck` + `golangci-lint` for linting
- Table-driven tests
- `slog` for structured logging
- Error wrapping with `fmt.Errorf("context: %w", err)`

### TypeScript/Next.js (my-copilot)

- TypeScript strict mode
- Nav Aksel Design System (`@navikt/ds-react`) for UI components
- **Always use Aksel spacing tokens (Box, VStack, HStack), never Tailwind p-/m- utilities**
- ESLint + Prettier + Knip for code quality
- Vitest for testing
- **Norwegian UI text**: Follow `apps/my-copilot/ORDBOK.md` for terminology — keep English tech terms where there's no good Norwegian alternative, use simple words, avoid unnecessary anglicisms

## Git Workflow

- Feature branches off `main`
- PRs require passing CI checks
- Squash merge to `main`
- **Semantic commit messages** using [Conventional Commits](https://www.conventionalcommits.org/):
  - `feat:` new features
  - `fix:` bug fixes
  - `style:` visual/UI changes (no logic change)
  - `refactor:` code restructuring
  - `docs:` documentation changes
  - `chore:` maintenance, config, dependencies
  - `test:` adding or updating tests
  - Scopes in parentheses when helpful: `feat(docs):`, `style(my-copilot):`
- **No `Co-authored-by` trailers** in commit messages
- **Do not push** unless explicitly asked
- **Do not amend commits on main** unless explicitly asked — use a new commit instead

## NAIS Deployment

- Manifests in `apps/<name>/.nais/`
- Required endpoints: `/isalive`, `/isready`, `/metrics`
- Environment configs: dev + prod

## Boundaries

See [SECURITY.md](SECURITY.md) for the full security architecture, trust zones, and auth flow.

### Always

- Run `mise check` after changes
- Follow existing code patterns in the project
- Use parameterized queries for any database access
- Validate all external input

### Ask First

- Changing authentication mechanisms
- Modifying NAIS production configurations
- Adding new dependencies

### Never

- Commit secrets or credentials
- Use Tailwind p-/m- utilities instead of Aksel spacing tokens
- Skip input validation on external boundaries
