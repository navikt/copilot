# 🧭 nav-pilot — Navs AI-utviklerverktøy

nav-pilot gjør GitHub Copilot til en Nav-ekspert. Én agent med en 4-fase modell (Intervju → Plan → Review → Lever) som koder inn Navs institusjonelle kunnskap.

📖 **Full dokumentasjon:** [min-copilot.ansatt.nav.no/nav-pilot/docs](https://min-copilot.ansatt.nav.no/nav-pilot/docs)

---

## Kom i gang

```bash
# Installer nav-pilot CLI
curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh | bash

# Installer en samling i repoet ditt
cd /path/to/your/repo
nav-pilot install kotlin-backend

# Bruk i Copilot
@nav-pilot Jeg trenger en ny tjeneste som behandler dagpengesøknader
```

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
