# 📦 Copilot Collections

Collections are curated bundles of agents, skills, instructions, and prompts organized by team archetype.

📖 **Full documentation:** [min-copilot.ansatt.nav.no/nav-pilot/docs](https://min-copilot.ansatt.nav.no/nav-pilot/docs)

## Available collections

| Collection | Description | Best for |
| --- | --- | --- |
| **kotlin-backend** | Kotlin/Ktor and Spring Boot on Nais | Backend API and event consumers |
| **frontend** | Framework-agnostic frontend (Aksel, UU, testing) | Astro, Remix, Vite and other non-Next.js frontends |
| **nextjs-frontend** | Next.js with Aksel Design System | Innbygger- og saksbehandler-frontends |
| **fullstack** | Complete stack (backend + frontend) | Teams that own the full stack |
| **platform** | Nais, observability, security | Platform and DevOps teams |

Each collection is one `manifest.json` that references items by name. The CLI resolves those names to actual files from the repository. Read a collection's manifest to see exactly what it pulls in.

## Creating a new collection

1. Create a directory in `.github/collections/<name>/`
2. Add a `manifest.json` listing the items:

```json
{
  "name": "my-collection",
  "description": "Description of the collection",
  "agents": ["nav-pilot", "my-agent"],
  "skills": ["nav-plan", "nav-deep-interview"],
  "instructions": ["my-instruction"],
  "prompts": ["my-prompt"]
}
```

3. Test with `nav-pilot install --dry-run <name>`
4. Submit a PR

Changing an existing collection is the same job. Edit its `manifest.json`, make sure every agent, skill, instruction and prompt you name exists in the repository, then:

```bash
nav-pilot install --dry-run <collection>
nav-pilot install --force <collection>
```

## Exporting for other tools

If you use [OpenCode](https://github.com/anomalyco/opencode) or [oh-my-openagent](https://github.com/code-yeongyu/oh-my-openagent) instead of GitHub Copilot, you can export all Nav customizations to `.opencode/` format:

```bash
nav-pilot export opencode              # generates .opencode/ in current directory
nav-pilot export opencode --user       # exports to ~/.config/opencode/ (global)
nav-pilot export opencode --dry-run    # preview what would be exported
```

See [nav-pilot docs](README.nav-pilot.md#eksport-til-andre-verktøy) for transformation details.
