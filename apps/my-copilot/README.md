# My Copilot

My Copilot is a self-service tool for managing your GitHub Copilot subscription. It allows users to activate or deactivate their Copilot subscription and view their subscription details.

## What It Does

- **Subscription Management**: Users can activate or deactivate their GitHub Copilot subscription.
- **Subscription Details**: Users can view details about their current subscription, including plan type, status, last activity, and more.
- **User Information**: Displays user information such as name, email, and groups.

## Integrations

- **BigQuery**: Usage analytics are read from BigQuery (`copilot_metrics.usage_metrics`), populated by the [copilot-metrics](../copilot-metrics/) naisjob. This replaced the deprecated GitHub Copilot Metrics API.
- **GitHub API**: Manages Copilot subscriptions and retrieves user details.
  - Uses GitHub Copilot User Management API for seat assignments and billing
  - All API requests include `X-GitHub-Api-Version: 2022-11-28` header for stability
- **Azure AD**: Uses Azure AD for authentication and authorization, ensuring that only authorized users can access the application.
- **Next.js**: Built with Next.js 16 for server-side rendering and optimized performance.
- **Aksel Design System**: Uses NAV's design system (`@navikt/ds-react`) with Tailwind CSS for styling.

## Development

### Prerequisites

- Node.js (version 22 or higher)
- pnpm (version 7 or higher)
- A GitHub App with the necessary permissions
- Azure AD application for authentication

### Getting Started

First, clone the repository:

```bash
git clone https://github.com/nais/my-copilot.git
cd my-copilot
```

Install the dependencies:

```bash
pnpm install --frozen-lockfile
```

Create a `.env.local` file in the root directory and add the required environment variables:

```env
GITHUB_APP_ID=your_github_app_id
GITHUB_APP_PRIVATE_KEY=your_github_app_private_key
GITHUB_APP_INSTALLATION_ID=your_github_app_installation_id
AZURE_APP_CLIENT_ID=your_azure_app_client_id
AZURE_OPENID_CONFIG_JWKS_URI=your_azure_openid_config_jwks_uri
AZURE_OPENID_CONFIG_ISSUER=your_azure_openid_config_issuer
```

#### BigQuery Access (for usage analytics and adoption data)

The stats/usage pages read from BigQuery. To access BigQuery locally:

1. Authenticate with GCP:

   ```bash
   gcloud auth application-default login
   ```

2. Add BigQuery env vars to `.env.local`:

   ```env
   GCP_TEAM_PROJECT_ID=<your-nais-team-project-id>

   # Copilot Metrics (usage data from copilot-metrics naisjob)
   COPILOT_METRICS_DATASET=copilot_metrics
   COPILOT_METRICS_TABLE=usage_metrics

   # Copilot Adoption (repo scanning data from copilot-adoption naisjob)
   COPILOT_ADOPTION_DATASET=copilot_adoption
   ```

   Find the project ID with `nais project list` or `gcloud projects list --filter="name:copilot"`.

Run the development server:

```bash
pnpm dev
```

Open [http://localhost:3000](http://localhost:3000) with your browser to see the result.

### Building and Testing

To build the project:

```bash
pnpm build
```

To run the tests:

```bash
pnpm test
```

### Deployment

This project uses GitHub Actions for CI/CD. The workflow is defined in `.github/workflows/build-deploy.yaml`. The application is deployed to the Nais platform.

### Environment Variables

All environment variables are documented below, organized by integration:

| Variable                        | Required | Default                                        | Description                                 |
| ------------------------------- | -------- | ---------------------------------------------- | ------------------------------------------- |
| **GitHub App**                  |          |                                                |                                             |
| `GITHUB_APP_ID`                 | Yes      | —                                              | GitHub App ID for API access                |
| `GITHUB_APP_PRIVATE_KEY`        | Yes      | —                                              | GitHub App private key (PEM format)         |
| `GITHUB_APP_INSTALLATION_ID`    | Yes      | —                                              | GitHub App installation ID for the org      |
| **Azure AD** (provided by NAIS) |          |                                                |                                             |
| `AZURE_APP_CLIENT_ID`           | Yes      | —                                              | Azure AD application client ID              |
| `AZURE_OPENID_CONFIG_JWKS_URI`  | Yes      | —                                              | Azure AD JWKS endpoint for JWT validation   |
| `AZURE_OPENID_CONFIG_ISSUER`    | Yes      | —                                              | Azure AD issuer for JWT validation          |
| **BigQuery**                    |          |                                                |                                             |
| `GCP_TEAM_PROJECT_ID`           | Yes      | —                                              | GCP project ID containing BigQuery datasets |
| `COPILOT_METRICS_DATASET`       | No       | `copilot_metrics`                              | Dataset for usage metrics                   |
| `COPILOT_METRICS_TABLE`         | No       | `usage_metrics`                                | Table for daily usage data                  |
| `COPILOT_ADOPTION_DATASET`      | No       | `copilot_adoption`                             | Dataset for repo adoption scan results      |
| **Telemetry** (Faro)            |          |                                                |                                             |
| `NEXT_PUBLIC_FARO_URL`          | No       | `https://telemetry.ekstern.dev.nav.no/collect` | Grafana Faro collector endpoint             |
| `NEXT_PUBLIC_FARO_APP_NAME`     | No       | `min-copilot`                                  | Application name for Faro                   |
| `NEXT_PUBLIC_FARO_NAMESPACE`    | No       | `nais`                                         | Namespace for Faro                          |
| **MCP Registry**                |          |                                                |                                             |
| `MCP_REGISTRY_URL`              | No       | `https://mcp-registry.nav.no`                  | MCP server registry URL                     |

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

Exposed via `GET /metrics` in Prometheus format.

| Metric                               | Type    | Labels | Description                             |
| ------------------------------------ | ------- | ------ | --------------------------------------- |
| `copilot_seats_total`                | Gauge   | —      | Total Copilot seats in the organization |
| `copilot_seats_active_this_cycle`    | Gauge   | —      | Seats active in current billing cycle   |
| `copilot_seats_inactive_this_cycle`  | Gauge   | —      | Seats inactive in current billing cycle |
| `copilot_seats_added_this_cycle`     | Gauge   | —      | Seats added in current billing cycle    |
| `copilot_seats_pending_invitation`   | Gauge   | —      | Seats with pending invitations          |
| `copilot_seats_pending_cancellation` | Gauge   | —      | Seats pending cancellation              |
| `mycopilot_page_views_total`         | Counter | `page` | Page views by section                   |

A shared Grafana dashboard is available at [`dashboards/copilot-ecosystem.json`](../../dashboards/copilot-ecosystem.json).

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

The customization detail drawer (`src/components/detail-drawer.tsx`) provides per-editor install instructions for all customization types. The following official references were used to build these instructions:

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

Each customization in `.github/` has a sibling `metadata.json` file that provides catalog metadata (domain, tags, usage examples). These files are **never loaded by Copilot** — they're only consumed by the manifest generator.

### File placement

| Type         | Pattern                                     | Example                       |
| ------------ | ------------------------------------------- | ----------------------------- |
| Agents       | `.github/agents/<name>.metadata.json`       | `aksel.metadata.json`         |
| Instructions | `.github/instructions/<name>.metadata.json` | `nextjs-aksel.metadata.json`  |
| Prompts      | `.github/prompts/<name>.metadata.json`      | `nais-manifest.metadata.json` |
| Skills       | `.github/skills/<name>/metadata.json`       | `aksel-spacing/metadata.json` |
| MCP servers  | `examples` field in `allowlist.json`        | See mcp-registry README       |

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
