# AGENTS.md — navikt/copilot

Monorepo containing NAV's GitHub Copilot ecosystem tools:

- **my-copilot** — Self-service portal for managing Copilot subscriptions (Next.js 16, TypeScript)
- **mcp-onboarding** — Reference MCP server with GitHub OAuth (Go)
- **mcp-registry** — Public registry for NAV-approved MCP servers (Go)

All apps deployed on NAIS (Kubernetes on GCP).

## Build & Test Commands

From repo root:

```bash
mise check    # Lint + type check all apps
mise test     # Run all tests
mise build    # Build all apps
mise all      # All of the above
```

Per-app (run from `apps/<name>/`):

```bash
# Go apps (mcp-onboarding, mcp-registry)
mise check    # fmt, vet, staticcheck, golangci-lint, test
mise test     # go test -v ./...
mise build    # go build

# Next.js app (my-copilot)
mise check    # ESLint, TypeScript, Prettier, Knip, Jest
mise test     # pnpm test (Jest)
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
- Jest for testing

## Git Workflow

- Feature branches off `main`
- PRs require passing CI checks
- Squash merge to `main`

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
