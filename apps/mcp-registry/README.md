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

**Optional fields**: `status` (default: `active`), `publishedAt`, `remotes`

## References

- [MCP Registry v0.1 Specification](https://github.com/modelcontextprotocol/registry)
- [MCP Server JSON Schema](https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json)
- [GitHub MCP Registry Configuration](https://docs.github.com/en/copilot/how-tos/administer-copilot/manage-mcp-usage/configure-mcp-registry)
- [MCP Specification](https://modelcontextprotocol.io)
