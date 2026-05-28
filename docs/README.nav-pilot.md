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

# Personlig installasjon (fungerer på tvers av alle repoer)
nav-pilot install --user --all  # installerer agenter, skills og instruksjoner til ~/.copilot/

# Oppdater nav-pilot
nav-pilot upgrade

# Rapporter en feil eller foreslå en forbedring
nav-pilot feedback             # åpner bug-rapport i nettleser
nav-pilot feedback --feature   # åpner feature request
```

## Oppgradering

```bash
# Enkleste metode — nav-pilot oppdaterer seg selv
nav-pilot upgrade

# Alternativt via Homebrew
brew update && brew upgrade nav-pilot
```

### Feilsøking: «already installed»

Hvis `brew upgrade` sier at nav-pilot allerede er oppdatert men versjonen er gammel, skyldes det at den lokale tap-cachen ikke er oppdatert. Kjør `brew update` først for å hente nyeste formler:

```bash
brew update && brew upgrade nav-pilot
```

Dersom `brew update` feiler med tilgangsfeil:

```bash
sudo chown -R $(whoami) /opt/homebrew
brew update && brew upgrade nav-pilot
```

## Installasjonsscopes

nav-pilot støtter to installasjonsscopes:

| Scope | Plassering | Innhold | Bruk |
|-------|-----------|---------|------|
| **Repo** (standard) | `.github/` | Agenter, skills, instruksjoner, prompts | Delt med teamet via git |
| **Personlig** (`--user`) | `~/.copilot/` | Agenter, skills og instruksjoner | Personlig, fungerer i alle repoer |

- **Repo-scope** er standard — filene sjekkes inn i git og deles med hele teamet.
- **Personlig scope** installerer til `~/.copilot/`. Agenter og skills leses automatisk av alle Copilot-klienter. Instruksjoner krever `COPILOT_CUSTOM_INSTRUCTIONS_DIRS` og fungerer kun med Copilot CLI.
- Prompts støttes kun i repo-scope.

#### Instruksjoner i personlig scope

Instruksjoner installeres til `~/.copilot/.github/instructions/`. nav-pilot setter `COPILOT_CUSTOM_INSTRUCTIONS_DIRS` automatisk når du starter cplt via interaktiv modus. For direkte bruk av cplt, legg til i shell-profilen:

```bash
eval "$(nav-pilot env)"
```

## Eksport til andre verktøy

nav-pilot kan eksportere Navs tilpasninger til andre AI-kodeverktøy:

```bash
# Eksporter til OpenCode / oh-my-openagent
nav-pilot export opencode              # → .opencode/ i nåværende mappe
nav-pilot export opencode --user       # → ~/.config/opencode/ (globalt)
nav-pilot export opencode --dry-run    # forhåndsvis
nav-pilot export opencode --force      # overskriv eksisterende
```

Eksport transformerer `.github/`-artefakter til `.opencode/`-format:

| Nav-artefakt | OpenCode-mål | Transformasjon |
|---|---|---|
| Skills (`SKILL.md`) | `.opencode/skills/` | 1:1-kopi (kompatibelt format) |
| Prompts (`.prompt.md`) | `.opencode/commands/` | Fjerner `name` fra frontmatter |
| Agenter (`.agent.md`) | `.opencode/agents/` | Erstatter frontmatter med `mode: subagent` |
| Instruksjoner (`.instructions.md`) | `AGENTS.md` | Slår sammen alle til én fil |

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

### Tips: Bruk skills for spesialisert hjelp

Skills er on-demand kunnskapspakker du aktiverer med `$skill-navn`:

| Skill | Hva den gjør |
|-------|-------------|
| `$terse-mode` | Kompakt output — sparer ~65 % output-tokens |
| `$security-owasp` | OWASP 2025-sjekk for Go, Kotlin, Java, Node.js |
| `$nav-deep-interview` | Strukturert intervju før implementering |
| `$api-design` | REST API-design med Nav-konvensjoner |
| `$nais` | Deployment og plattformhjelp |

Se alle tilgjengelige skills med `$help` eller på [ki-utvikling.nav.no/nav-pilot/docs](https://ki-utvikling.nav.no/nav-pilot/docs).

## For bidragsytere

### Endre agenten

Agenten ligger i `.github/agents/nav-pilot.agent.md`. Den inneholder ruterlogikken — hvilke skills som brukes i hvilken rekkefølge. Faseadferd styres av `<operating_loop>` øverst i filen, som sikrer at agenten holder seg i riktig fase gjennom hele samtalen.

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

## Tilpasse synkronisering

Hvis teamet ditt ikke trenger alle filene fra en samling (f.eks. Next.js-filer i et Astro-prosjekt), kan du markere dem som *overrides* i `.github/copilot-sync.json`. Da hoppes de over ved sync — ingen hashsjekk, ingen PR-diff.

Se [Sync → Overrides](README.sync.md#overrides) for eksempler.

## Relatert

- [Testing →](README.testing.md) — Strukturelle og E2E-tester for nav-pilot
- [Collections →](README.collections.md) — Samlinger og install-script
- [Agents →](README.agents.md) — Alle tilgjengelige agenter
- [Skills →](README.skills.md) — Alle tilgjengelige skills
- [Sync →](README.sync.md) — Automatisk oppdatering
