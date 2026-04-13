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

## 4-fase modell

nav-pilot jobber i fire faser med eksplisitte stopp mellom hver. Fasene er synlige i output med emoji-prefiks:

```
🔍 Fase 1: Intervju — kartlegger behov og blinde flekker
   Stiller spørsmål om domene, personvern, avhengigheter, auth, drift
   ─────────────────────────────────────────
   ⏳ Venter på svar før Fase 2: Plan

📐 Fase 2: Plan — arkitektur og beslutninger
   Velger arketype, foreslår mønstre, lager Nais-konfig
   ─────────────────────────────────────────
   ⏳ Bekreft planen før Fase 3: Review

🔎 Fase 3: Review — kvalitetssikring
   Delegerer til @auth, @security-champion, @nais, @observability
   ─────────────────────────────────────────
   ⏳ Bekreft funn før Fase 4: Lever

🚀 Fase 4: Lever — genererer kode og dokumentasjon
   Implementerer basert på godkjent plan og review
```

Modellen hopper over faser når konteksten tilsier det — en direkte implementeringsoppgave kan gå rett til Fase 4.

### Spesialist-delegering

I Fase 3 delegerer nav-pilot til spesialistagenter som viser fremdrift med egne emoji-prefiks:

| Agent | Prefiks | Domene |
|-------|---------|--------|
| `@auth-agent` | 🔐 | Azure AD, TokenX, M2M `azp`-validering |
| `@security-champion-agent` | 🛡️ | Trusselmodellering, OWASP, compliance |
| `@nais-agent` | ⚙️ | Nais-konfig, GCP-ressurser, deploy |
| `@observability-agent` | 📊 | Prometheus, OpenTelemetry, dashboards |
| `@code-review-agent` | 📝 | Kodefeil, Nav-konvensjoner |

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

- [Testing →](README.testing.md) — Strukturelle og E2E-tester for nav-pilot
- [Collections →](README.collections.md) — Samlinger og install-script
- [Agents →](README.agents.md) — Alle tilgjengelige agenter
- [Skills →](README.skills.md) — Alle tilgjengelige skills
- [Sync →](README.sync.md) — Automatisk oppdatering
