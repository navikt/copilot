# 🧭 nav-pilot — Navs AI-utviklerverktøy

nav-pilot gjør GitHub Copilot til en Nav-ekspert. Én agent, fire skills og fire collections koder inn Navs institusjonelle kunnskap som kjørbare arbeidsflyter.

📖 **Full dokumentasjon:** [min-copilot.ansatt.nav.no/nav-pilot/docs](https://min-copilot.ansatt.nav.no/nav-pilot/docs)

---

## Arkitektur

nav-pilot er bygget på tre lag:

```
┌─────────────────────────────────────────────────────────┐
│  Lag 1: Instruksjoner (alltid aktive)                   │
│  Nav-mønstre, kodestandarder, anti-patterns             │
│  → Hver Copilot-sesjon er Nav-bevisst automatisk        │
├─────────────────────────────────────────────────────────┤
│  Lag 2: @nav-pilot agent (én inngangsport)              │
│  Ruter til riktig fase og skill                         │
│  Delegerer til @auth, @nais, @kafka, @security-champion │
├─────────────────────────────────────────────────────────┤
│  Lag 3: Skills (byggeklosser)                           │
│  Intervju, plan, review, feilsøking                     │
│  Brukes av @nav-pilot eller alene                       │
└─────────────────────────────────────────────────────────┘
```

### Design-prinsipper

1. **Kunnskap, ikke orkestrering** — Verdien vår er institusjonell kunnskap, ikke orkestrering som snart blir standardvare.
2. **Tynn ruter, tykke skills** — `@nav-pilot` delegerer til skills med beslutningstrær, maler og sjekklister.
3. **Eksplisitte stopp** — nav-pilot foreslår, du godkjenner, nav-pilot fortsetter.
4. **Arketype først** — Første spørsmål er alltid «hva slags ting bygger du?»
5. **Minimalt CLI** — CLI-et er et rent installasjonsverktøy (Go, null avhengigheter). All AI-funksjonalitet er markdown kjørt av GitHub Copilot.

---

## For bidragsytere

### Endre agenten

Agenten ligger i `.github/agents/nav-pilot.agent.md`. Den inneholder ruterlogikken — hvilke skills som brukes i hvilken rekkefølge.

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

- [Collections →](README.collections.md) — Samlinger og install-script
- [Agents →](README.agents.md) — Alle tilgjengelige agenter
- [Skills →](README.skills.md) — Alle tilgjengelige skills
- [Sync →](README.sync.md) — Automatisk oppdatering
