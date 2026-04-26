# Template: Monorepo Root

README template for a monorepo root containing multiple apps, libraries, or packages. Replace `{placeholders}` with actual values.

---

# {Monorepo Name}

{One-sentence description of what this monorepo contains — under 120 characters.}

## Innhold

- [{App/lib name}]({path}/README.md) — {one-line description}
- [{App/lib name}]({path}/README.md) — {one-line description}
- [{App/lib name}]({path}/README.md) — {one-line description}

## Struktur

```
{root}/
├── apps/
│   ├── {app-1}/          # {short description}
│   └── {app-2}/          # {short description}
├── libs/
│   └── {lib-1}/          # {short description}
├── docs/                  # Dokumentasjon
└── .nais/                 # Nais-manifester (om felles)
```

## Kom i gang

**Forutsetninger:** {e.g., mise, Node.js ≥ 22, Go ≥ 1.24}

```bash
git clone https://github.com/navikt/{monorepo-name}
cd {monorepo-name}
{install-command}        # Installer avhengigheter for alle apps
{dev-command}            # Start alt lokalt
{test-command}           # Kjør alle tester
{check-command}          # Lint + typesjekk + test
```

### Per app

```bash
cd apps/{app-name}
{app-dev-command}
```

## Tech stack

| App | Språk | Rammeverk | Database |
|-----|-------|-----------|----------|
| {app-1} | {e.g., TypeScript} | {e.g., Next.js 16} | {e.g., —} |
| {app-2} | {e.g., Go} | {e.g., stdlib} | {e.g., PostgreSQL} |

## Deploy

Hver app deployes uavhengig via GitHub Actions til Nais.

| App | Dev | Prod | Manifest |
|-----|-----|------|----------|
| {app-1} | Push til `main` | {Manuell/Tag} | `apps/{app-1}/.nais/` |
| {app-2} | Push til `main` | {Manuell/Tag} | `apps/{app-2}/.nais/` |

## Bidra

1. Lag branch fra `main`
2. Gjør endringer i relevant `apps/` eller `libs/` mappe
3. Kjør `{check-command}` fra rotmappen
4. Opprett PR

Se [CONTRIBUTING.md](CONTRIBUTING.md) for detaljer.

## Team

- **Team:** {#team-name}
- **Slack:** [{#team-channel}]({slack-url})

## Lisens

[{License}](LICENSE)
