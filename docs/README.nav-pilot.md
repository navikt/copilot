# 🧭 nav-pilot — Navs AI-utviklerverktøy

nav-pilot gjør GitHub Copilot til en Nav-ekspert. I stedet for å huske alle mønstrene, beslutningstrærne og fellene selv — spør `@nav-pilot`.

```
@nav-pilot Jeg trenger en ny tjeneste som behandler dagpengesøknader
```

nav-pilot stiller de riktige spørsmålene, velger riktig auth, genererer Nais-manifest, og hjelper deg unngå de vanligste feilene — før du skriver en linje kode.

---

## Hva er nav-pilot?

nav-pilot er en samling av **én agent, fire skills og fire samlinger** som koder inn Navs institusjonelle kunnskap som kjørbare arbeidsflyter.

| Komponent | Hva det er |
| --- | --- |
| `@nav-pilot` | Planleggingsagent — din inngangsport |
| `$nav-deep-interview` | Avdekker blinde flekker (personvern, auth, avhengigheter) |
| `$nav-plan` | Beslutningstrær → Nais-manifest, CI/CD, prosjektstruktur |
| `$nav-architecture-review` | Flerperspektiv-review → ADR |
| `$nav-troubleshoot` | Diagnostikk for pod-krasj, 401-er, Kafka-lag, DB-feil |

nav-pilot er **ikke** et separat CLI-verktøy. Det er markdown-filer som fungerer direkte i VS Code, JetBrains, Copilot CLI og GitHub.com.

---

## Hvorfor nav-pilot og ikke oh-my-codex?

[oh-my-codex](https://github.com/cline/oh-my-codex), [oh-my-claudecode](https://github.com/anthropics/oh-my-claudecode) og [oh-my-openagent](https://github.com/nicepkg/oh-my-openagent) er populære «harness»-verktøy som legger planlegging, team-orkestrering og lifecycle hooks oppå CLI-agenter. De er imponerende — men de løser feil problem for Nav.

| | oh-my-codex | nav-pilot |
| --- | --- | --- |
| **Installasjon** | `npm install -g` | Én kommando eller ett klikk |
| **Inngangspunkt** | `omx plan` (terminal) | `@nav-pilot` (VS Code, JetBrains, CLI, GitHub.com) |
| **Kunnskap** | Generisk koding | Navs institusjonelle spillebok |
| **Auth** | Vet ikke hva TokenX er | Velger riktig auth-mekanisme basert på caller-type |
| **Plattform** | Vet ikke hva Nais er | Genererer Nais-manifest med riktig accessPolicy |
| **Oppdateringer** | `npm update` | Auto-sync workflow (ukentlig PR) |
| **Vedlikehold** | Hold tritt med CLI-endringer | Bare markdown — GitHub vedlikeholder runtime |

**Kort sagt:** oh-my-\* bygger bedre orkestrering. nav-pilot bygger bedre *kunnskap*. Orkestrering blir commoditized. Kunnskap er vanskelig å kopiere.

### Hva nav-pilot vet som Copilot ikke vet

Copilot er god på kode, men vet ingenting om:

- At innbyggere bruker ID-porten men saksbehandlere bruker Azure AD
- At du trenger `accessPolicy.inbound` i Nais-manifestet, ellers kan ingen kalle tjenesten din
- At HikariCP default pool (10) er for stort for containere — start med 3
- At du aldri skal sette CPU-limits i Nais (bare requests)
- At PII aldri skal logges — logg sakId, ikke fnr
- At Azure client_credentials med brukerkontext mister audit trail
- At Rapids & Rivers-meldinger trenger `@event_name` og `demandValue`
- At Chainguard-images er standard i Nav, ikke distroless

Denne kunnskapen er kodet inn i nav-pilot sine beslutningstrær, blinde-flekker-sjekklister og diagnostiske trær.

---

## Hvordan det fungerer

nav-pilot jobber i fire faser med eksplisitte stopp mellom hver:

```
┌──────────────┐     ┌──────────┐     ┌──────────┐     ┌───────────┐
│  1. Intervju │ ──→ │  2. Plan │ ──→ │ 3. Review│ ──→ │ 4. Lever  │
│  Hva bygger  │     │ Arkitek- │     │ Sikkerhet│     │ Kode,     │
│  vi?         │     │ tur +    │     │ Plattform│     │ tester,   │
│              │     │ test     │     │ Endring  │     │ docs      │
└──────────────┘     └──────────┘     └──────────┘     └───────────┘
      ↑ stopp              ↑ stopp         ↑ stopp          ↑ stopp
```

Du bestemmer når du går videre. Nav-pilot foreslår — du godkjenner.

### Fase 1: Intervju

nav-pilot stiller målrettede spørsmål for å avdekke blinde flekker. De fleste Nav-utviklere glemmer å stille seg selv:

- **Personvern:** Behandler dere PII? Hvilke kategorier?
- **Auth:** Hvem kaller tjenesten — bruker, tjeneste, ekstern partner?
- **Avhengigheter:** Hva skjer når en avhengighet er nede?
- **Endringspåvirkning:** Hvem konsumerer dine API-er/hendelser? Hvem påvirkes?
- **Teststatus:** Hva er testdekningen i koden som endres?
- **Observerbarhet:** Hvilke forretningsmetrikker viser at tjenesten fungerer?

### Fase 2: Plan

Basert på svarene genererer nav-pilot en konkret plan:

- **Auth-beslutning** — ID-porten, Azure AD, TokenX eller Maskinporten
- **Nais-manifest** — ferdig YAML med riktige ressurser og accessPolicy
- **Prosjektstruktur** — mappestruktur for valgt arketype
- **CI/CD** — GitHub Actions workflow med build, test, deploy
- **Teststrategi** — riktig testnivå per komponent, karakteriseringstester ved endring
- **Database** — Flyway-migrasjoner, HikariCP-konfig
- **Leveransedokumenter** — endringsdokument, utrullingsplan, observerbarhetsplan

### Fase 3: Review

Planen gjennomgås fra fire perspektiver:

1. **Sikkerhet** — Er auth riktig? Er PII beskyttet?
2. **Plattform** — Passer ressursene? Fungerer observerbarhet?
3. **Arkitektur** — Er dette den enkleste løsningen?
4. **Endringssikkerhet** — Er teststrategi definert? Er rollback-plan realistisk?

### Fase 4: Lever

Basert på godkjent plan genereres:
- Kode, config og tester
- Endringsdokument med rollback-plan
- Observerbarhetsplan med suksesskriterier
- Post-deploy-verifiseringssjekkliste
- API-endringsdokument (ved breaking changes)
- Runbook-oppdatering (ved ny tjeneste)

---

## Kom i gang

### Alternativ 1: Installer en samling (anbefalt)

```bash
# Installer nav-pilot CLI
curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh | bash

# Installer en samling i repoet ditt
cd /path/to/your/repo
nav-pilot install kotlin-backend
```

Fire samlinger er tilgjengelige:

| Samling | Best for |
| --- | --- |
| `kotlin-backend` | Backend API og hendelsekonsumenter |
| `nextjs-frontend` | Innbygger- og saksbehandler-frontends |
| `fullstack` | Team som eier hele stacken |
| `platform` | Plattform- og DevOps-team |

Se [Collections →](README.collections.md) for full oversikt.

### Alternativ 2: Installer bare nav-pilot

```bash
mkdir -p .github/agents .github/skills
cd /tmp && git clone --depth 1 https://github.com/navikt/copilot.git nav-copilot

# Agent
cp nav-copilot/.github/agents/nav-pilot.agent.md .github/agents/
cp nav-copilot/.github/agents/nav-pilot.metadata.json .github/agents/

# Skills
for skill in nav-deep-interview nav-plan nav-architecture-review nav-troubleshoot; do
  cp -r nav-copilot/.github/skills/$skill .github/skills/
done

rm -rf /tmp/nav-copilot
```

### Alternativ 3: Fra verktøysiden

Gå til [min-copilot.ansatt.nav.no/verktoy](https://min-copilot.ansatt.nav.no/verktoy), finn nav-pilot-agent og klikk «Installer».

---

## Brukseksempler

### Planlegg ny tjeneste

```
@nav-pilot Jeg trenger en ny tjeneste som behandler dagpengesøknader.
Den mottar hendelser fra dp-soknad via Kafka og lagrer vedtak i PostgreSQL.
Saksbehandlere skal kunne se vedtak via et API.
```

nav-pilot vil:
1. Spørre om dataklassifisering (fnr = fortrolig)
2. Velge TokenX for saksbehandler-API, Kafka for hendelser
3. Generere Nais-manifest med riktig auth og accessPolicy
4. Lage prosjektstruktur med Rapids & Rivers-konsument

### Planlegg ny frontend

```
@nav-pilot Vi skal lage et saksbehandlerverktøy for tiltakspenger med Next.js.
Saksbehandlere logger inn med Azure AD og skal kunne se og behandle søknader.
```

### Feilsøk et problem

```
@nav-pilot Poden til dp-behandling krasjer med CrashLoopBackOff i dev-gcp.
Loggen sier "Connection refused: localhost:5432".
```

### Generer ADR

```
@nav-pilot Vi vurderer å bytte fra REST til Kafka for vedtakshendelser mellom
dp-behandling og dp-utbetaling. Generer en ADR.
```

---

## Skillene i detalj

### `$nav-deep-interview`

Kjør et strukturert intervju som avdekker blinde flekker. Stiller spørsmål fra fire domener:

- **Personvern og data** — PII-kategorier, dataklassifisering, sletteregler
- **Plattform og auth** — Caller-type, avhengigheter, feilhåndtering
- **Observerbarhet** — Forretningsmetrikker, varsling, on-call
- **Team og prosess** — Avhengigheter, deadlines, erfaring

Inkluderer referansedata:
- `data-classification.md` — Navs fire klassifiseringsnivåer
- `blind-spots.md` — 25+ vanlige oversikter fra ekte Nav-repoer

### `$nav-plan`

Arkitekturplanlegging med beslutningstrær. Dekker:

- **Auth-beslutningstre** — Fra caller-type til Nais-konfigurasjon
- **Kommunikasjonstre** — REST, Kafka, SSE
- **Database-tre** — PostgreSQL, BigQuery, Redis, stateless
- **accessPolicy-tre** — Inbound og outbound regler

Genererer konkrete artefakter:
- Nais-manifest (ferdig YAML)
- Prosjektstruktur (Kotlin/Ktor, Spring Boot, Next.js)
- CI/CD-workflow (GitHub Actions)
- Database-migrasjoner (Flyway)

Inkluderer referansedata:
- `decision-trees.md` — Alle beslutningstrær med kodeeksempler
- `nais-templates.md` — Komplette Nais-maler for 5 arketyper

### `$nav-architecture-review`

Generer Architecture Decision Records (ADR) med flerperspektiv-review:

1. **Arkitektur** — Passer dette i Navs arkitektur? Enklere alternativer?
2. **Sikkerhet** — Data, auth, tilgang, PII
3. **Plattform** — Nais, ressurser, observerbarhet, CI/CD

Inkluderer referansedata:
- `adr-template.md` — Komplett ADR-mal med Nav-spesifikke seksjoner
- `nav-principles.md` — Navs arkitekturprinsipper (Team First, essensiell kompleksitet, DORA)

### `$nav-troubleshoot`

Diagnostiske trær for vanlige Nav-plattformproblemer:

| Symptom | Diagnostikk |
| --- | --- |
| Pod krasjer (CrashLoopBackOff) | Status → logs → events → ressurser |
| 401/403 | Token → issuer → audience → expiry → JWKS → accessPolicy |
| Kafka consumer lag | Konsument oppe? → Feil i log? → Poison pill? → Schema-mismatch? |
| DB-tilkobling feiler | Cloud SQL oppe? → Env-vars? → Flyway? → Pool exhaustion? |
| Treg responstid | Prometheus → Tempo trace → DB EXPLAIN → ekstern avhengighet |
| Deploy feiler | Actions-feil? → Nais deploy-feil? → Pod starter ikke? |

---

## Arkitektur

nav-pilot er bygget på tre lag:

```
┌─────────────────────────────────────────────────────────┐
│  Lag 1: Instruksjoner (alltid aktive)                   │
│  Nav-mønstre, kodestandarer, anti-patterns              │
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

1. **Kunnskap, ikke orkestrering** — Vår moat er institusjonell kunnskap (auth-trær, Nais-maler, anti-patterns), ikke fancy orkestrering som commoditiseres.

2. **Tynn ruter, tykke skills** — `@nav-pilot` er en lett ruter som delegerer til skills for tung innhold. Skills har referansefiler med beslutningstrær, maler og sjekklister.

3. **Eksplisitte stopp** — Ikke en magisk pipeline som gjør alt automatisk. nav-pilot foreslår, du godkjenner, nav-pilot fortsetter.

4. **Arketype først** — Første spørsmål er alltid «hva slags ting bygger du?» Dette bestemmer stack, auth, og Nais-konfigurasjon.

5. **Bare markdown** — Ingen CLI-binary, ingen npm-pakke, ingen runtime-avhengigheter. Fungerer overalt GitHub Copilot fungerer.

### Hvorfor ikke et eget CLI?

oh-my-codex og oh-my-claudecode er CLI-wrappere. Det gir dem kontroll, men også:

- Avhengighet av underliggende CLI-versjon
- Vedlikeholdsbyrde når API-er endres
- Begrenset til terminalen
- Egen installasjon og oppdateringssyklus

nav-pilot unngår alt dette ved å bruke GitHub Copilots egne primitiver (agents, skills, instructions). GitHub vedlikeholder runtime — vi vedlikeholder kunnskap.

---

## Holde oppdatert

### Automatisk sync (anbefalt)

Sett opp [copilot-customization-sync](https://github.com/navikt/copilot-customization-sync) workflow for ukentlige PRs med oppdateringer.

### Manuelt

Kjør install-scriptet på nytt — det overskriver eksisterende filer:

```bash
nav-pilot install --force kotlin-backend
```

---

## Relatert

- [Collections →](README.collections.md) — Samlinger og install-script
- [Agents →](README.agents.md) — Alle tilgjengelige agenter
- [Skills →](README.skills.md) — Alle tilgjengelige skills
- [Sync →](README.sync.md) — Automatisk oppdatering
- [RFC →](nav-planning-skills-rfc.md) — Bakgrunn og designbeslutninger
