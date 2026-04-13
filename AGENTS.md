# AGENTS.md — navikt/copilot

Monorepo containing Nav's GitHub Copilot ecosystem tools:

- **my-copilot** — Self-service portal for managing Copilot subscriptions (Next.js 16, TypeScript)
- **copilot-metrics** — Naisjob that populates BigQuery with daily Copilot usage metrics (Go)
- **mcp-onboarding** — Reference MCP server with GitHub OAuth (Go)
- **mcp-registry** — Public registry for Nav-approved MCP servers (Go)

All apps deployed on NAIS (Kubernetes on GCP).

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
# Go apps (mcp-onboarding, mcp-registry)
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
  mcp-onboarding/     # Go MCP server — OAuth, 16 tools, readiness assessment
  mcp-registry/       # Go registry API — allowlist.json, MCP Registry v0.1 spec
  my-copilot/         # Next.js 16 — App Router, Aksel Design System
    src/
      app/            # Routes and API handlers
      components/     # React components
      lib/            # Utilities, auth, GitHub API client
.github/
  instructions/       # Scoped Copilot instructions (*.instructions.md)
  agents/             # Custom Copilot agents (*.agent.md)
  prompts/            # Reusable prompt templates (*.prompt.md)
  skills/             # Domain knowledge packages (SKILL.md)
  copilot-instructions.md  # Global Copilot instructions
docs/                 # Documentation
```

### Customization Language

- **YAML descriptions**: Norwegian (shown on the my-copilot website)
- **Body content**: English for backend, infra, security, and database topics; Norwegian for UI/UX, Aksel, and accessibility topics

## Code Style

### Go (mcp-onboarding, mcp-registry)

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

## NAIS Deployment

- Manifests in `apps/<name>/.nais/`
- Required endpoints: `/isalive`, `/isready`, `/metrics`
- Environment configs: dev + prod

## Boundaries

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
