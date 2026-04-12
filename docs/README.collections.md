# 📦 Copilot Collections

Collections are curated bundles of agents, skills, instructions, and prompts organized by team archetype.

📖 **Full documentation:** [min-copilot.ansatt.nav.no/nav-pilot/docs](https://min-copilot.ansatt.nav.no/nav-pilot/docs)

## Available Collections

| Collection | Description | Agents | Skills | Best for |
| --- | --- | --- | --- | --- |
| **kotlin-backend** | Kotlin/Ktor and Spring Boot on Nais | 6 | 10 | Backend API and event consumers |
| **nextjs-frontend** | Next.js with Aksel Design System | 4 | 7 | Innbygger- og saksbehandler-frontends |
| **fullstack** | Complete stack (backend + frontend) | 10 | 13 | Teams that own the full stack |
| **platform** | Nais, observability, security | 4 | 7 | Platform and DevOps teams |

## Collection Structure

```
.github/collections/
├── kotlin-backend/
│   └── manifest.json       # Lists all agents, skills, instructions, prompts
├── nextjs-frontend/
│   └── manifest.json
├── fullstack/
│   └── manifest.json
└── platform/
    └── manifest.json
```

Each `manifest.json` references items by name. The CLI resolves these to actual files from the repository.

## Creating a New Collection

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

## Modifying a Collection

Edit the `manifest.json` in the collection directory. Items are referenced by name — ensure the referenced agents, skills, instructions, and prompts exist in the repository.

After modifying, test with:

```bash
nav-pilot install --dry-run <collection>
nav-pilot install --force <collection>
```
