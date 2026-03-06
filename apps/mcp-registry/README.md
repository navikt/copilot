# MCP Registry

Public registry service for Nav-approved MCP servers, implementing the [MCP Registry v0.1 specification](https://github.com/modelcontextprotocol/registry).

**Public URL:** `https://mcp-registry.nav.no`

## Endpoints

- `GET /` - Service information and available endpoints
- `GET /v0.1/servers` - List all registered MCP servers
- `GET /v0.1/servers/{name}/versions/{version}` - Get specific server version
- `GET /v0.1/servers/{name}/versions/latest` - Get latest version of a server
- `GET /health` - Health check endpoint
- `GET /ready` - Readiness check endpoint
- `GET /metrics` - Prometheus metrics endpoint

**Server names must be URL-encoded** - the `/` in names like `io.github.navikt/github-mcp` becomes `%2F`.

## Configuration

**Environment Variables:**

- `PORT` (default: `8080`) - Server port
- `LOG_LEVEL` (default: `INFO`) - `DEBUG` | `INFO` | `WARN` | `ERROR`
- `LOGGED_ENDPOINTS` (default: `/,/v0.1/servers`) - Comma-separated endpoint paths to log
- `DOMAIN_INTERNAL` (default: `intern.dev.nav.no`) - Internal domain for template substitution
- `DOMAIN_EXTERNAL` (default: `ekstern.dev.nav.no`) - External domain for template substitution

### Domain Template Variables

URLs in `allowlist.json` support template variables for environment-specific domains:

- `{{domain_internal}}` → replaced with `DOMAIN_INTERNAL` value
- `{{domain_external}}` → replaced with `DOMAIN_EXTERNAL` value

Example:

```json
{
  "remotes": [{ "type": "streamable-http", "url": "https://my-server.{{domain_internal}}/mcp" }]
}
```

- In dev: `https://my-server.intern.dev.nav.no/mcp`
- In prod: `https://my-server.intern.nav.no/mcp`

## Development

```bash
mise run version  # Generate version string
mise run install  # Download dependencies
mise run dev      # Run with DEBUG logging
mise run test     # Run tests with verbose output
mise run check    # Run all checks (fmt, vet, staticcheck, lint, test)
mise run build    # Build binary to bin/mcp-registry
mise run validate # Validate allowlist.json by starting server
```

**Available tasks:** Run `mise tasks` to see all available commands.

## Deployment

Automatic via GitHub Actions on merge to main.

- Production: `https://mcp-registry.nav.no` (public)
- Development: `https://mcp-registry.ekstern.dev.nav.no` (public)

## Setup

For user-facing setup instructions (enterprise admin configuration, IDE setup, Copilot CLI), see the [main README](../../README.md#mcp-registry--server-discovery).

## Registry Format (v0.1)

`allowlist.json` structure follows the [MCP Server JSON Schema](https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json):

```json
{
  "servers": [
    {
      "$schema": "https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json",
      "name": "io.github.navikt/github-mcp",
      "description": "Nav's GitHub MCP Server",
      "version": "1.0.0",
      "status": "active",
      "publishedAt": "2025-01-15T00:00:00Z",
      "remotes": [{ "type": "streamable-http", "url": "https://api.githubcopilot.com/mcp/" }]
    }
  ]
}
```

### Server Name Format

Names follow reverse-DNS with exactly one `/`:

- Format: `{namespace}/{name}` (e.g., `io.github.navikt/github-mcp`)
- Pattern: `^[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9]/[a-zA-Z0-9][a-zA-Z0-9._-]*[a-zA-Z0-9]$`

### Response Format

**List servers:** `GET /v0.1/servers`

```json
{
  "servers": [{ "server": {...}, "_meta": {...} }],
  "metadata": { "count": 1 }
}
```

**Get server:** `GET /v0.1/servers/io.github.navikt%2Fgithub-mcp/versions/latest`

```json
{
  "server": {
    "name": "io.github.navikt/github-mcp",
    "description": "...",
    "version": "1.0.0",
    "remotes": [...]
  },
  "_meta": {
    "io.modelcontextprotocol.registry/official": {
      "status": "active",
      "publishedAt": "2025-01-15T00:00:00Z",
      "updatedAt": "2025-01-15T00:00:00Z"
    }
  }
}
```

## Adding Servers

1. Edit `allowlist.json`
2. Run `mise run check` to validate
3. Submit PR (requires security review)

**Required fields**: `name`, `description`, `version`

**Optional fields**: `status` (default: `active`), `publishedAt`, `remotes`, `packages`

### Remote Servers (HTTP)

For MCP servers accessible over the network via Streamable HTTP or SSE:

```json
{
  "name": "io.github.navikt/my-server",
  "description": "My remote MCP server.",
  "version": "1.0.0",
  "remotes": [{ "type": "streamable-http", "url": "https://my-server.{{domain_internal}}/mcp" }]
}
```

### Local Servers (stdio via packages)

For MCP servers that run locally as processes (e.g. via `npx`), use the `packages` field instead of `remotes`. This follows the [MCP Registry Package Types](https://modelcontextprotocol.io/specification/draft/basic/transports#stdio) specification.

```json
{
  "name": "com.example/my-local-mcp",
  "description": "Local MCP server for development tooling.",
  "version": "1.0.0",
  "packages": [
    {
      "registryType": "npm",
      "identifier": "@example/my-mcp-server",
      "transport": { "type": "stdio" }
    }
  ]
}
```

**Package fields:**

| Field                  | Required | Description                                                               |
| ---------------------- | -------- | ------------------------------------------------------------------------- |
| `registryType`         | Yes      | Package registry: `npm`, `pypi`, `oci`, `nuget`, or `mcpb`                |
| `identifier`           | Yes      | Package name (e.g. `@playwright/mcp`, `mcp-server-fetch`)                 |
| `version`              | No       | Specific version to pin                                                   |
| `runtimeHint`          | No       | Runtime command hint (e.g. `npx`, `uvx`)                                  |
| `transport.type`       | Yes      | Transport type: `stdio` for local, `streamable-http` or `sse` for network |
| `environmentVariables` | No       | Array of env vars the server needs (API keys, config)                     |

**Environment variable entry:**

```json
{
  "name": "API_KEY",
  "description": "API key for the service",
  "isRequired": true,
  "isSecret": true
}
```

#### Examples from Official MCP Registry

**npm with stdio** ([Chrome DevTools MCP](https://github.com/ChromeDevTools/chrome-devtools-mcp)):

```json
{
  "name": "io.github.ChromeDevTools/chrome-devtools-mcp",
  "description": "Chrome DevTools MCP for browser debugging and performance analysis.",
  "version": "0.1.0",
  "packages": [
    {
      "registryType": "npm",
      "identifier": "chrome-devtools-mcp",
      "transport": { "type": "stdio" }
    }
  ]
}
```

**npm with stdio** ([Playwright MCP](https://github.com/microsoft/playwright-mcp)):

```json
{
  "name": "com.microsoft/playwright-mcp",
  "description": "Browser automation and testing using Playwright.",
  "version": "0.1.0",
  "packages": [
    {
      "registryType": "npm",
      "identifier": "@playwright/mcp",
      "transport": { "type": "stdio" }
    }
  ]
}
```

**npm with stdio** ([Next.js DevTools MCP](https://github.com/vercel/next-devtools-mcp)):

```json
{
  "name": "com.vercel/next-devtools-mcp",
  "description": "Next.js dev server diagnostics for coding agents.",
  "version": "0.1.0",
  "packages": [
    {
      "registryType": "npm",
      "identifier": "next-devtools-mcp",
      "transport": { "type": "stdio" }
    }
  ]
}
```

**Both remotes and packages** — a server can offer both remote and local installation options:

```json
{
  "name": "io.github.navikt/dual-mcp",
  "description": "MCP server available both remotely and locally.",
  "version": "1.0.0",
  "remotes": [{ "type": "streamable-http", "url": "https://dual-mcp.{{domain_internal}}/mcp" }],
  "packages": [
    {
      "registryType": "npm",
      "identifier": "@navikt/dual-mcp",
      "transport": { "type": "stdio" }
    }
  ]
}

## Metrics

Exposed via `GET /metrics` in Prometheus format.

| Metric                          | Type      | Labels                          | Description                                              |
| ------------------------------- | --------- | ------------------------------- | -------------------------------------------------------- |
| `http_requests_total`           | Counter   | `method`, `path`, `status_code` | HTTP requests by method, path, and status                |
| `http_request_duration_seconds` | Histogram | `method`, `path`                | HTTP request latency                                     |
| `registry_server_lookups_total` | Counter   | `server`, `result`              | Server lookups by name and result (`found`, `not_found`) |

A shared Grafana dashboard is available at [`dashboards/copilot-ecosystem.json`](../../dashboards/copilot-ecosystem.json).

## References

- [MCP Registry v0.1 Specification](https://github.com/modelcontextprotocol/registry)
- [MCP Server JSON Schema](https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json)
- [MCP Registry Supported Package Types](https://modelcontextprotocol.io/specification/draft/basic/transports#stdio) — npm, PyPI, Docker, OCI, MCPB
- [MCP Registry Quickstart: Publish a Server](https://modelcontextprotocol.io/quickstart/publish)
- [MCP Transports: stdio](https://modelcontextprotocol.io/specification/draft/basic/transports#stdio) — how stdio transport works
- [GitHub MCP Registry Configuration](https://docs.github.com/en/copilot/how-tos/administer-copilot/manage-mcp-usage/configure-mcp-registry)
- [MCP Specification](https://modelcontextprotocol.io)
