# Copilot instructions for navikt/copilot

Monorepo for Nav's Copilot ecosystem and AI developer tooling:

- `apps/my-copilot`: Next.js 16 and Aksel web portal (Zone 1 BFF)
- `apps/copilot-api`: Go backend API for GitHub Copilot and BigQuery data (Zone 2)
- `apps/copilot-cli`: Go proxy that verifies GitHub App user tokens for `nav-pilot`
- `apps/copilot-survey`: Go service and PostgreSQL store for anonymous surveys (Zone 3)
- `apps/copilot-metrics`: Go batch job for GitHub Copilot usage and billing metrics
- `apps/copilot-adoption`: Go batch job scanning `navikt` repositories for AI adoption
- `apps/mcp-registry`: Go service serving the Nav MCP allowlist
- `apps/mcp-onboarding`: Go reference MCP server with OAuth 2.1 and PKCE
- `cli/nav-pilot`: Go CLI that installs customizations and launches Copilot or OpenCode
- `agents/`, `skills/`, `instructions/`, `prompts/`, `hooks/`: Distributable customizations synced to Nav repositories

Read `AGENTS.md` for workflow rules and `SECURITY.md` for the security model and trust boundaries.

## Security and trust boundaries

- **Three-zone architecture**:
  - Zone 1 (`my-copilot`) is internet-facing behind Wonderwall (`src/proxy.ts` enforces authentication on private paths and exchanges tokens via Texas OBO). It holds no GitHub App keys or BigQuery credentials.
  - Zone 2 (`copilot-api`) is internal-only, validates Entra ID JWTs (`azp` must match an allowed caller from `NAIS_TOKEN_INTROSPECTION` / JWKS), and holds the GitHub App private key and BigQuery access.
  - Zone 3 (`copilot-survey`) stores anonymous survey responses in PostgreSQL (`survey_participation` holds HMAC-hashed participant keys and `survey_answers` holds unlinked answers with no user ID or timestamp). It must never log caller identities or receive Entra ID or GitHub credentials.
- **`copilot-cli` authentication**: Verifies `ghu_` tokens against GitHub (`POST /applications/{client_id}/token`), checks active `navikt` organization membership fail-closed, validates `login` against GitHub's username format, and forwards the verified username to `copilot-api` via `X-On-Behalf-Of`.
- **`cli/nav-pilot` hostile repo boundary**: Treat target repositories and `.nav-pilot/` or `opencode.json` configs as untrusted. Reject symlinks in artifact directories and exports (`source.CheckSymlink`), never expand arbitrary host environment variables into OpenCode configs, and never run `nav-pilot` in tests without `testhome.Run` and `isolatedConfig(t)`.
- **No secrets or PII in code or logs**: Never commit credentials or tokens. Never log tokens, `Authorization` headers, SAML `nameId` values, emails, or raw SQL errors containing user input.

## Repository conventions

- **Issue-closing keywords**: PR bodies may be in Norwegian, but issue references must use GitHub's English keywords (`Closes #NNN` or `Fixes #NNN`). Never use `Lukker #NNN` or `Fikser #NNN`, which GitHub ignores.
- **Artifact language**: Body text in `agents/`, `skills/`, `instructions/`, and `prompts/` is English. Metadata in YAML frontmatter and `metadata.json` (`description`, `title`, `displayName`) stays Norwegian.
- **Generated files**: Do not hand-edit `apps/my-copilot/src/lib/copilot-manifest.json` or `docs/README.*.md`. Run `mise run generate` after changing customizations.
- **Focused diffs**: Keep changes minimal and scoped to the task. Run `mise check` and `mise test` in the affected directory before opening a PR.
- **Code review**: Focus on security boundaries, correctness, data races, and regressions. Skip nitpicks that linters (`golangci-lint`, `Biome`, `staticcheck`) already catch.
