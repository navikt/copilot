# Nav MCP Onboarding

A reference MCP (Model Context Protocol) server demonstrating GitHub OAuth authentication and NAV Copilot customization discovery for use with GitHub Copilot in VS Code.

## Overview

This server implements:

- **OAuth 2.1 with PKCE** - Secure authentication flow required by MCP spec
- **Dynamic Client Registration (RFC 7591)** - MCP clients register automatically, no manual client_id needed
- **GitHub OAuth proxy** - Acts as OAuth authorization server, proxying to GitHub
- **Organization access control** - Validates user membership in allowed GitHub organizations
- **MCP JSON-RPC** - Full protocol implementation with streamable HTTP transport
- **Customization Discovery** - Browse and install NAV Copilot agents, instructions, prompts, and skills

## Architecture

```text
┌─────────────────┐     ┌──────────────────────────────┐     ┌─────────────┐
│   VS Code       │────▶│  mcp-onboarding + Discovery │────▶│   GitHub    │
│   (MCP Client)  │◀────│  (OAuth + MCP + Discovery)   │◀────│   OAuth     │
└─────────────────┘     └──────────────────────────────┘     └─────────────┘
```

**Flow:**

1. VS Code discovers OAuth metadata via `/.well-known/oauth-authorization-server`
2. VS Code registers as a client via `POST /register` (Dynamic Client Registration)
3. User is redirected to GitHub for authentication
4. Server exchanges GitHub code for tokens and validates org membership
5. Server issues its own access token mapped to GitHub session
6. VS Code uses token to call MCP tools (both hello-world and discovery)

## Available Tools

### Hello World Tools

| Tool          | Description                                                  |
| ------------- | ------------------------------------------------------------ |
| `hello_world` | Returns a greeting with authenticated user's GitHub username |
| `greet`       | Returns a personalized greeting message                      |
| `whoami`      | Returns information about the authenticated GitHub user      |
| `echo`        | Echoes back a provided message                               |
| `get_time`    | Returns current server time in various formats               |

### Discovery Tools

| Tool                     | Description                                         | Parameters                             |
| ------------------------ | --------------------------------------------------- | -------------------------------------- |
| `search_customizations`  | Search NAV Copilot customizations                   | `query`, `type`, `tags` (all optional) |
| `list_agents`            | List all NAV Copilot agents                         | `category` (optional)                  |
| `list_instructions`      | List all NAV Copilot instructions                   | None                                   |
| `list_prompts`           | List all NAV Copilot prompts                        | None                                   |
| `list_skills`            | List all NAV Copilot skills                         | None                                   |
| `get_installation_guide` | Get installation guide for a specific customization | `type` (required), `name` (required)   |

### Discovery Tool Examples

```javascript
// Search for all agents related to "nais"
search_customizations({ query: "nais", type: "agent" })

// List all frontend-related customizations
search_customizations({ tags: ["frontend"] })

// List all agents in the platform category
list_agents({ category: "platform" })

// Get installation guide for nais-agent
get_installation_guide({ type: "agent", name: "nais-agent" })
```

## Configuration

| Environment Variable   | Description                         | Default                 |
| ---------------------- | ----------------------------------- | ----------------------- |
| `PORT`                 | Server port                         | `8080`                  |
| `BASE_URL`             | Public URL for OAuth redirects      | `http://localhost:8080` |
| `GITHUB_CLIENT_ID`     | GitHub OAuth App client ID          | (required)              |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth App client secret      | (required)              |
| `ALLOWED_ORGANIZATION` | GitHub org users must belong to     | `navikt`                |
| `LOG_LEVEL`            | Log level: DEBUG, INFO, WARN, ERROR | `INFO`                  |

## Setup

### 1. Create GitHub OAuth App

1. Go to GitHub → Settings → Developer settings → OAuth Apps
2. Create new OAuth App with:
   - **Homepage URL**: Your server URL
   - **Authorization callback URL**: `{BASE_URL}/oauth/callback`
3. Note the Client ID and generate a Client Secret

### 2. Run Locally

```bash
export GITHUB_CLIENT_ID=your_client_id
export GITHUB_CLIENT_SECRET=your_client_secret
export BASE_URL=http://localhost:8080

mise run dev
```

### 3. Test Endpoints

```bash
curl http://localhost:8080/.well-known/oauth-authorization-server | jq
curl http://localhost:8080/.well-known/oauth-protected-resource | jq

curl http://localhost:8080/mcp
```

## Development

```bash
mise run version    # Generate version string (YYYYMMDD-gitsha)
mise run install    # Download dependencies
mise run generate   # Generate copilot-manifest.json from .github files
mise run check      # Run all checks (fmt, vet, lint, test, generate:check)
mise run test       # Run tests
mise run build      # Build binary to bin/mcp-onboarding
mise run dev        # Run with DEBUG logging
mise run lint       # Run golangci-lint
```

### Generating Customizations Manifest

The customizations manifest is **embedded** into the binary at compile time using Go's `embed` directive. This ensures the manifest is always available and cannot get out of sync with the binary.

**Generating the manifest:**
```bash
mise generate    # or: go run ./cmd/generate-manifest
```

This creates `internal/discovery/copilot-manifest.json` which is embedded into the binary.

**CI Check**: The `mise check` command includes `generate:check` which fails if the manifest is out of date. This ensures the embedded manifest stays synchronized with the `.github` files.

Always run `mise run generate` after adding or modifying agent, instruction, prompt, or skill files.

## Deployment

Automatic deployment via GitHub Actions on merge to main and pull requests.

- **Production**: `https://mcp-onboarding.nav.no`
- **Development**: `https://mcp-onboarding.intern.dev.nav.no`

Deployed to Nais using the reusable `mise-build-deploy-nais` workflow.

## API Endpoints

| Endpoint                                  | Method | Description                            |
| ----------------------------------------- | ------ | -------------------------------------- |
| `/.well-known/oauth-authorization-server` | GET    | OAuth server metadata                  |
| `/.well-known/oauth-protected-resource`   | GET    | Protected resource metadata            |
| `/register`                               | POST   | Dynamic Client Registration (RFC 7591) |
| `/oauth/authorize`                        | GET    | Start OAuth flow                       |
| `/oauth/callback`                         | GET    | GitHub OAuth callback                  |
| `/oauth/token`                            | POST   | Token exchange                         |
| `/mcp`                                    | POST   | MCP JSON-RPC endpoint                  |
| `/health`                                 | GET    | Health check                           |
| `/ready`                                  | GET    | Readiness check                        |

## MCP Registry

This server is registered in Nav's MCP registry:

- **Server Name**: `io.github.navikt/mcp-onboarding`
- **Version**: 2.0.0
- **Capabilities**: OAuth 2.1, Hello World tools, NAV Copilot customization discovery

## Security

- Uses OAuth 2.1 with PKCE (Proof Key for Code Exchange)
- Dynamic Client Registration for seamless MCP client onboarding
- Redirect URIs restricted to `http://127.0.0.1`, `http://localhost`, or `https://`
- Client registrations rate limited (max 1000) and expire after 30 days
- Validates GitHub organization membership before issuing tokens
- Tokens expire after 1 hour (refresh tokens: 30 days)
- All tokens and client registrations stored in memory (lost on restart)

## License

MIT
