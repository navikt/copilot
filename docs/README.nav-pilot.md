# 🧭 nav-pilot — Navs AI-utviklerverktøy

nav-pilot gjør GitHub Copilot til en Nav-ekspert. Én agent med en 4-fase modell (Intervju → Plan → Review → Lever) som koder inn Navs institusjonelle kunnskap.

📖 **Full dokumentasjon:** [min-copilot.ansatt.nav.no/nav-pilot/docs](https://min-copilot.ansatt.nav.no/nav-pilot/docs)

---

## Kom i gang

```bash
# Installer nav-pilot CLI (macOS)
brew install navikt/tap/nav-pilot

# Alternativt (Linux / CI)
# curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh | bash

# Installer en samling i repoet ditt
cd /path/to/your/repo
nav-pilot                        # interaktiv velger
nav-pilot install kotlin-backend # eller direkte

# Installer til brukerens hjemmappe (fungerer på tvers av alle repoer)
nav-pilot install --user fullstack  # installerer agenter og skills til ~/.copilot/

# Oppdater nav-pilot
nav-pilot update

# Rapporter en feil eller foreslå en forbedring
nav-pilot feedback             # åpner bug-rapport i nettleser
nav-pilot feedback --feature   # åpner feature request
```

## Installasjonsscopes

nav-pilot støtter to installasjonsscopes:

| Scope | Plassering | Innhold | Bruk |
|-------|-----------|---------|------|
| **Repo** (standard) | `.github/` | Agenter, skills, instruksjoner, prompts | Delt med teamet via git |
| **Bruker** (`--user`) | `~/.copilot/` | Kun agenter og skills | Personlig, fungerer i alle repoer |

- **Repo-scope** er standard — filene sjekkes inn i git og deles med hele teamet.
- **Bruker-scope** installerer til `~/.copilot/`, som GitHub Copilot leser automatisk. Agenter og skills er tilgjengelige i alle repoer uten å modifisere hvert enkelt repo.
- Instruksjoner og prompts støttes kun i repo-scope (GitHub Copilot leser ikke disse fra `~/.copilot/`).

## Bruk

Det finnes tre måter å bruke nav-pilot på:

### Terminal (GitHub Copilot CLI)

```bash
copilot --agent nav-pilot --prompt "Jeg trenger en ny tjeneste som behandler dagpengesøknader"
```

### VS Code / JetBrains (Copilot Chat)

```
@nav-pilot Jeg trenger en ny tjeneste som behandler dagpengesøknader
```

### nav-pilot CLI (interaktiv)

```bash
nav-pilot
```

Starter interaktiv modus — sjekker oppdateringer og tilbyr å starte Copilot med valgt agent.

## For bidragsytere

### Endre agenten

Agenten ligger i `.github/agents/nav-pilot.agent.md`. Den inneholder ruterlogikken — hvilke skills som brukes i hvilken rekkefølge. Fase-headers styres av `<response_format>` XML-tag øverst i filen.

### Endre skills

Hver skill ligger i `.github/skills/<name>/`:
- `SKILL.md` — Prompt-instruksjoner
- `metadata.json` — Metadata (navn, beskrivelse)
- `references/` — Referansedata (beslutningstrær, maler, sjekklister)

### Legge til ny kunnskap

1. Identifiser et beslutningstre, anti-pattern eller mal
2. Legg det i riktig skill sin `references/`-mappe
3. Oppdater `SKILL.md` til å referere til den nye filen
4. Test med `@nav-pilot` i en ekte kontekst

---

## Relatert

- [Testing →](README.testing.md) — Strukturelle og E2E-tester for nav-pilot
- [Collections →](README.collections.md) — Samlinger og install-script
- [Agents →](README.agents.md) — Alle tilgjengelige agenter
- [Skills →](README.skills.md) — Alle tilgjengelige skills
- [Sync →](README.sync.md) — Automatisk oppdatering
