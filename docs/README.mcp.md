# 🔌 MCP Servers

Nav-godkjente MCP-servere som utvider Copilot med eksterne verktøy.

📖 **Utforsk og installer:** [ki-utvikling.nav.no/verktoy](https://ki-utvikling.nav.no/verktoy)

## Tilgjengelige MCP-servere

| Server                    | Beskrivelse                                                         | URL                                        |
| ------------------------- | ------------------------------------------------------------------- | ------------------------------------------ |
| **GitHub MCP**            | GitHub repos, issues, PRs og kodesøk. Innebygd i VS Code.          | `https://api.githubcopilot.com/mcp/`       |
| **Nav Copilot Discovery** | Oppdag Nav-tilpasninger, vurder agent-readiness, generer AGENTS.md. | `https://mcp-onboarding.intern.nav.no/mcp` |
| **Figma MCP**             | Hent Figma-designkontekst for design-to-code.                      | `https://mcp.figma.com/mcp`                |

## Installer

### VS Code

Enkleste vei er MCP Registry. Åpne Extensions (`Cmd+Shift+X`), filtrer på MCP Registry, søk opp serveren og installer.

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

### Copilot CLI og OpenCode

```bash
nav-pilot mcp list
nav-pilot mcp enable figma-mcp --client copilot
```

Nye oppføringer får registerets headers og verktøyvalg. Eksisterende oppføringer beholdes.
Et manuelt `copilot mcp add` kopierer ikke disse begrensningene fra registeret.

Nais MCP (`nav-pilot mcp enable io.github.nais/nais-mcp`) og GitHub-serveren i OpenCode trenger innloggingen din. Under cplt kjører cplt dem på maskinen din, utenfor sandkassen, med bare lesende verktøy som standard. Figma logger du inn på i klienten. Se [MCP gjennom cplt](https://ki-utvikling.nav.no/nav-pilot/guider/mcp-gjennom-cplt).

Tilkoblet, men verktøyet mangler? Se [MCP-feilsøking](https://ki-utvikling.nav.no/nav-pilot/guider/feilsoking#mcp-verktoy).

> Se [GitHub MCP-dokumentasjonen](https://docs.github.com/en/copilot/customizing-copilot/extending-copilot-chat-with-mcp) for oppsett i andre editorer.

## MCP Registry API

Nav MCP Registry på `https://mcp-registry.nav.no` implementerer [MCP Registry v0.1](https://modelcontextprotocol.io).

| Endpoint                                   | Beskrivelse               |
| ------------------------------------------ | ------------------------- |
| `GET /v0.1/servers`                        | List alle godkjente servere |
| `GET /v0.1/servers/{name}/versions/latest` | Hent siste versjon        |
