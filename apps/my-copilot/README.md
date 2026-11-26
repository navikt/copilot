# My Copilot

My Copilot is a self-service tool for managing your GitHub Copilot subscription. It allows users to activate or deactivate their Copilot subscription and view their subscription details.

## What It Does

- **Subscription Management**: Users can activate or deactivate their GitHub Copilot subscription.
- **Subscription Details**: Users can view details about their current subscription, including plan type, status, last activity, and more.
- **User Information**: Displays user information such as name, email, and groups.

## Video HUD System

The homepage features a production-ready **video HUD system** for playing short educational videos in a dedicated player interface. This system includes:

- **Playback Control**: Play, pause, skip forward/backward, fullscreen, and repeat functionality
- **Progress Tracking**: Persistent watch state tracking via localStorage with debounced saves
- **KPI Telemetry**: Event emission for analytics (play started, errors, rebuffers)
- **URL Synchronization**: Video state synced with URL query parameters for deep linking
- **Accessibility**: WCAG 2.1 AA compliant with keyboard navigation and screen reader support
- **Overlay System**: Metadata overlays (episode badges, progress indicators, content metadata)

### Video sorting & watch state

Videos are sorted by watch status: unwatched videos appear first, and watched videos (progress ≥ 80%) are deprioritised to the back of the feed.

The order is **frozen while the viewer is open** so the active video never jumps when it crosses the watched threshold mid-playback. The list re-syncs to the latest watch order only when playback returns to idle (i.e. when you close the viewer).

See [`docs/VIDEO_HUD_ARCHITECTURE.md`](./docs/VIDEO_HUD_ARCHITECTURE.md#4-video-sorting--watch-state-freeze) for the algorithm and the freeze mechanism.

**For complete documentation**, see [`docs/VIDEO_HUD_ARCHITECTURE.md`](./docs/VIDEO_HUD_ARCHITECTURE.md). This includes:

- Architecture overview with adapter pattern (media, storage, URL-sync, telemetry)
- Event guard pattern preventing background video corruption
- State machine for legal playback transitions
- KPI deduplication strategies
- Component library reference (episode pills, badges, content panels, etc.)
- Testing patterns and performance tips

## Integrations

- **copilot-api**: my-copilot is a BFF with no BigQuery or GitHub credentials. Every data call goes to [copilot-api](../copilot-api/) (`COPILOT_API_URL`, default `http://copilot-api`) with an OBO token from Texas (`src/lib/backend-api.ts`).
  - copilot-api reads usage analytics from BigQuery (`copilot_metrics.usage_metrics`), populated by the [copilot-metrics](../copilot-metrics/) naisjob. This replaced the deprecated GitHub Copilot Metrics API.
  - copilot-api calls the GitHub Copilot User Management API for seat assignments and billing, with the `X-GitHub-Api-Version: 2022-11-28` header.
- **Azure AD**: Uses Azure AD for authentication and authorization, ensuring that only authorized users can access the application.
- **Next.js**: Built with Next.js 16 for server-side rendering and optimized performance.
- **Aksel Design System**: Uses Nav's design system (`@navikt/ds-react`) with Tailwind CSS for styling.

## Development

### Prerequisites

- Node.js (version 22 or higher)
- pnpm (version 7 or higher)
- Azure AD application for authentication

### Getting Started

First, clone the repository:

```bash
git clone https://github.com/navikt/copilot.git
cd copilot/apps/my-copilot
```

Install the dependencies:

```bash
pnpm install --frozen-lockfile
```

`@nais/apm` is served from GitHub Packages, which requires authentication even for public packages. The project `.npmrc` sets only the registry mapping. Keep credentials in your user-level `~/.npmrc`; an existing valid literal credential works unchanged. For environment-based credentials, put this line in `~/.npmrc` and set `GITHUB_PACKAGES_TOKEN` in your environment:

```ini
//npm.pkg.github.com/:_authToken=${GITHUB_PACKAGES_TOKEN}
```

Both pnpm 11.5.1 and 11.28.9 support this user-level configuration. Never commit credentials. CI uses a runner-temporary user config with the workflow's `GITHUB_TOKEN` and `packages: read`; Docker mounts the user config as a BuildKit secret.

Create a `.env.local` file in the root directory and add the required environment variables:

```env
AZURE_APP_CLIENT_ID=your_azure_app_client_id
AZURE_OPENID_CONFIG_JWKS_URI=your_azure_openid_config_jwks_uri
AZURE_OPENID_CONFIG_ISSUER=your_azure_openid_config_issuer
```

#### Data from copilot-api

The stats and usage pages get their data from copilot-api, not from BigQuery directly. Run copilot-api locally (see [its README](../copilot-api/README.md)) and point `COPILOT_API_URL` at it in `.env.local`. With neither `NAIS_CLUSTER_NAME` nor `NAIS_TOKEN_EXCHANGE_ENDPOINT` set, `src/lib/backend-api.ts` treats the run as local development and calls copilot-api without an OBO token. The BigQuery variables (`GCP_TEAM_PROJECT_ID`, `COPILOT_METRICS_DATASET`, `COPILOT_ADOPTION_DATASET` and so on) belong to copilot-api.

Run the development server:

```bash
pnpm dev
```

Open [http://localhost:3000](http://localhost:3000) with your browser to see the result.

### Storybook

Storybook is configured for component-level development and visual testing.

Start Storybook locally:

```bash
pnpm storybook
# or
mise run storybook
```

Build static Storybook:

```bash
pnpm build-storybook
# or
mise run storybook:build
```

Storybook runs on [http://localhost:6006](http://localhost:6006).

`mise run dev` starts only the Next.js app. Storybook is started separately with the commands above.

Video stories use real media URLs from the dev bucket (`copilot-videos-public-dev`) via fixtures in `src/components/video/stories/storybook-video-fixtures.ts`.
Story structure and documentation conventions are documented in Storybook under `Video/Dokumentasjon/Storybook Guidelines`.

### Building and Testing

To build the project:

```bash
pnpm build
```

To run the tests:

```bash
pnpm test
```

The survey form also has a Playwright test against two production builds
(`e2e/survey.spec.ts`): it loads the page on one build, swaps in the other on
the same port, and sends. It also checks an expired login. CI runs it when
`apps/my-copilot` changes.

```bash
pnpm exec playwright install chromium   # once
mise run e2e
```

### No server actions

Forms post to route handlers (`src/app/api/**/route.ts`), not server actions.
An action id changes with every build, so a page loaded before a deploy cannot
call the new one, and the request fails before it reaches the backend. This
lost survey answers in October 2026 (#1804). ESLint refuses `"use server"`.

### Links must keep working

`src/lib/link-inventory.json` lists every path and anchor on the site that has been published or linked. `src/link-inventory.test.ts` checks that each one still resolves, and fails when the repo links to something that is not listed. After adding a page, a heading or a link, run:

```bash
pnpm link-inventory:update
```

The command only adds entries. When a page moves, add a permanent redirect in `next.config.ts`. When an anchor moves, add it to `src/lib/legacy-anchors.ts`. Never delete an inventory entry.

### The /cplt page

`/cplt` is the landing page for the cplt open-source project, for readers outside Nav. It stays in English and keeps its own dark design. It sits under the English `(en)` route group (`src/app/(en)/cplt/`), so it gets the English site header and footer and no nav-pilot section menu; the menu still links to it as «Sandkassen (cplt)». Don't translate it or give it the Aksel look of the documentation pages; fix bugs only. `/cplt/windows` is its WSL2 guide for Windows users, under the same rules: a short version of the WSL2 section in the cplt README. `src/cplt-page.test.ts` fails if either page loses `lang="en"`.

### Deployment

This project uses GitHub Actions for CI/CD. The workflow is `.github/workflows/my-copilot.yaml`, which calls the shared `.github/workflows/mise-build-deploy-nais.yaml`. The application is deployed to the Nais platform.

### Environment Variables

All environment variables are documented below, organized by integration:

| Variable                                | Required | Default                       | Description                                           |
| --------------------------------------- | -------- | ----------------------------- | ----------------------------------------------------- |
| **Azure AD** (provided by NAIS)         |          |                               |                                                       |
| `AZURE_APP_CLIENT_ID`                   | Yes      | —                             | Azure AD application client ID                        |
| `AZURE_OPENID_CONFIG_JWKS_URI`          | Yes      | —                             | Azure AD JWKS endpoint for JWT validation             |
| `AZURE_OPENID_CONFIG_ISSUER`            | Yes      | —                             | Azure AD issuer for JWT validation                    |
| **copilot-api**                         |          |                               |                                                       |
| `COPILOT_API_URL`                       | No       | `http://copilot-api`          | copilot-api base URL (BigQuery and GitHub data)       |
| **Telemetry** (@nais/apm)               |          |                               |                                                       |
| `NAIS_FRONTEND_TELEMETRY_COLLECTOR_URL` | No       | set by nais                   | Collector URL; unset on localhost, so nothing is sent |
| **MCP Registry**                        |          |                               |                                                       |
| `MCP_REGISTRY_URL`                      | No       | `https://mcp-registry.nav.no` | MCP server registry URL                               |

### Group Access

This project uses group access to control who can use GitHub Copilot. The groups are defined in the `app.yaml` file under the `azure.application.claims.groups` section. To give more groups access, you need to add their IDs to this section.

Example:

```yaml
azure:
  application:
    enabled: true
    tenant: nav.no
    allowAllUsers: true
    claims:
      groups:
        - id: 48120347-8582-4329-8673-7beb3ed6ca06
        - id: 76e9ee7e-2cd1-4814-b199-6c0be007d7b4
        - id: eb5c5556-6c9a-4e54-83fc-f70cae25358d
        # Add more group IDs here
```

## Metrics

my-copilot exposes `GET /metrics` for Prometheus (`prometheus.path` in `.nais/app.yaml`) with its own HTTP and runtime metrics. The `copilot_seats_*` gauges come from copilot-api, see [copilot-api's README](../copilot-api/README.md#monitoring).

Grafana dashboards: [`dashboards/tenant/copilot-home.json`](../../dashboards/tenant/copilot-home.json) for a status overview with seats (users and cost live in my-copilot under `/admin/kostnad`) and [`dashboards/tenant/my-copilot-ops.json`](../../dashboards/tenant/my-copilot-ops.json) for health. See [`dashboards/README.md`](../../dashboards/README.md).

## Install Instructions References

### Client Support Matrix

Based on the [official Copilot customization cheat sheet](https://docs.github.com/en/copilot/reference/customization-cheat-sheet#ide-and-surface-support):

| Feature      | VS Code | IntelliJ | Copilot CLI | GitHub.com |
| ------------ | ------- | -------- | ----------- | ---------- |
| Instructions | ✓       | ✓        | ✓           | ✓          |
| Agents       | ✓       | ✗        | ✓           | ✓          |
| Prompts      | ✓       | ✓        | ✗           | ✓          |
| Skills       | ✓       | ✗        | ✓           | ✓          |
| MCP servers  | ✓       | ✓        | ✓           | ✓          |

This matrix is implemented in `CLIENT_SUPPORT` in `src/lib/install-commands.ts` and determines which editor tabs appear in the install accordion.

### References

The customization detail drawer (`src/components/detail-drawer/detail-drawer.tsx`) provides per-editor install instructions for all customization types. The following official references were used to build these instructions:

### VS Code

- **MCP server management**: <https://code.visualstudio.com/docs/copilot/customization/mcp-servers>
  - `--add-mcp` CLI option for adding servers to user profile
  - `.vscode/mcp.json` workspace configuration
  - MCP registry browsing via VS Code settings
- **MCP deep link URI scheme**: `vscode:mcp/{registry-host}/v0.1/servers/{url-encoded-name}/versions/latest`
  - Discovered via <https://github.com/microsoft/vscode/issues/276579>
  - Server names with `/` must be URL-encoded (`%2F`)
  - Also supports `vscode-insiders:mcp/...` for Insiders builds
- **Customization install URI schemes** (agents, instructions, prompts):
  - `vscode:chat-agent/install?url={raw-github-url}`
  - `vscode:chat-instructions/install?url={raw-github-url}`
  - `vscode:chat-prompt/install?url={raw-github-url}`
  - Source: <https://code.visualstudio.com/docs/copilot/customization>

### IntelliJ (JetBrains IDEs)

- **MCP registry browsing**: Open Copilot Chat → click MCP registry icon → browse/install/uninstall
  - Source: <https://github.blog/changelog/2025-10-29-mcp-registry-and-allowlist-controls-for-copilot-in-jetbrains-eclipse-and-xcode-now-in-public-preview/>
  - Status: Public preview (nightly builds as of March 2026)
- **MCP manual config path** (GitHub Copilot plugin): `~/.config/github-copilot/intellij/mcp.json`
  - Same `"servers"` format as VS Code's `.vscode/mcp.json`
  - Source: <https://docs.github.com/en/copilot/how-tos/context/model-context-protocol/extending-copilot-chat-with-mcp?tool=jetbrains>
- **No deep link support** for MCP registry install — browse-only via in-IDE UI
- **OAuth support**: Official docs now state JetBrains supports OAuth and PAT for remote MCP servers
  - Previously tracked as unsupported (JetBrains LLM-25012)
  - Our install instructions still show a warning — verify and remove once confirmed working

### Copilot CLI

- **`/mcp add` interactive form**: Name, Type (http/stdio), URL/Command, Environment Variables
- **MCP config file**: `~/.copilot/mcp-config.json` under `"mcpServers"`
- **Loopback redirect URI**: Uses ephemeral ports per RFC 8252 Section 7.3
  - Our OAuth server (`mcp-onboarding`) ignores port on loopback URIs

### Nav MCP Registry

- **Registry URL**: `https://mcp-registry.nav.no` (configurable via `MCP_REGISTRY_URL` env var)
- **Deep link construction**: `buildMcpInstallUrl()` in `src/lib/mcp-registry.ts`
  - Format: `vscode:mcp/mcp-registry.nav.no/v0.1/servers/{encoded-name}/versions/latest`

## Customization Metadata

Each customization in the repo-root `agents/`, `instructions/`, `prompts/` and `skills/` directories has a sibling `metadata.json` file that provides catalog metadata (domain, tags, usage examples). These files are **never loaded by Copilot** — they're only consumed by the manifest generator.

### File placement

| Type         | Pattern                              | Example                       |
| ------------ | ------------------------------------ | ----------------------------- |
| Agents       | `agents/<name>.metadata.json`        | `aksel.metadata.json`         |
| Instructions | `instructions/<name>.metadata.json`  | `nextjs-aksel.metadata.json`  |
| Prompts      | `prompts/<name>.metadata.json`       | `nais-manifest.metadata.json` |
| Skills       | `skills/<name>/metadata.json`        | `aksel-builder/metadata.json` |
| MCP servers  | `examples` field in `allowlist.json` | See mcp-registry README       |

### Schema

```json
{
  "domain": "frontend",
  "tags": ["aksel", "design-system", "react"],
  "examples": [
    {
      "scenario": "Konverter Tailwind til Aksel",
      "prompt": "@aksel-agent Konverter denne Tailwind-layouten til Aksel spacing tokens"
    }
  ]
}
```

Valid domains: `platform`, `frontend`, `backend`, `auth`, `observability`, `general`, `testing`, `design`.

### Regenerating the manifest

After adding or editing metadata files, regenerate the manifest:

```bash
cd apps/my-copilot
pnpm run generate
```

This produces `src/lib/copilot-manifest.json` which is committed to the repo.

## Learn More

To learn more about the technologies used in this project, take a look at the following resources:

- [Next.js Documentation](https://nextjs.org/docs) - learn about Next.js features and API.
- [GitHub API Documentation](https://docs.github.com/en/rest) - learn about the GitHub API.
- [Azure AD Documentation](https://docs.microsoft.com/en-us/azure/active-directory/) - learn about Azure AD.
- [Tailwind CSS Documentation](https://tailwindcss.com/docs) - learn about Tailwind CSS.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request with your changes.

## License

This project is licensed under the MIT License.
