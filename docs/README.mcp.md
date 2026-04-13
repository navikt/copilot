# 🔌 MCP Servers

Nav-godkjente MCP-servere som utvider Copilot med eksterne verktøy.

📖 **Utforsk og installer:** [min-copilot.ansatt.nav.no/verktoy](https://min-copilot.ansatt.nav.no/verktoy)

## Tilgjengelige MCP-servere

| Server                    | Beskrivelse                                                         | URL                                        |
| ------------------------- | ------------------------------------------------------------------- | ------------------------------------------ |
| **GitHub MCP**            | GitHub repos, issues, PRs og kodesøk. Innebygd i VS Code.          | `https://api.githubcopilot.com/mcp/`       |
| **Nav Copilot Discovery** | Oppdag Nav-tilpasninger, vurder agent-readiness, generer AGENTS.md. | `https://mcp-onboarding.intern.nav.no/mcp` |
| **Figma MCP**             | Hent Figma-designkontekst for design-to-code.                      | `https://mcp.figma.com/mcp`                |

## Installer

### VS Code (anbefalt: MCP Registry)

1. Åpne Extensions (`Cmd+Shift+X`) → filtrer **MCP Registry** → søk og installer

### Manuell konfigurasjon

Legg til i `.vscode/mcp.json` (delt med teamet) eller VS Code `settings.json` (personlig):

```json
{
  "servers": {
    "server-name": {
      "type": "http",
      "url": "https://server-url/mcp"
    }
  }
}
```

### Copilot CLI

```bash
gh copilot mcp add --type http server-name https://server-url/mcp
```

> Se [GitHub MCP-dokumentasjonen](https://docs.github.com/en/copilot/customizing-copilot/extending-copilot-chat-with-mcp) for oppsett i andre editorer.

## MCP Registry API

Nav MCP Registry på `https://mcp-registry.nav.no` implementerer [MCP Registry v0.1](https://modelcontextprotocol.io).

| Endpoint                                   | Beskrivelse               |
| ------------------------------------------ | ------------------------- |
| `GET /v0.1/servers`                        | List alle godkjente servere |
| `GET /v0.1/servers/{name}/versions/latest` | Hent siste versjon        |
