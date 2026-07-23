# Modellvalg i Nav Copilot

Levende referansedokument for hvilke modeller vi bruker, hvorfor, og hvordan vi evaluerer oppdateringer.

---

## Gjeldende modellpinning

Alle agenter og prompts har et eksplisitt `model:`-felt i YAML-frontmatter. Valget er gjort ut fra oppgavetype, kostnad og ytelse — ikke leverandørpreferanse.

### Agenter

| Agent | Modell | Kategori | Input | Output | Begrunnelse |
|-------|--------|----------|-------|--------|-------------|
| `@nav-pilot` | Claude Sonnet 4.6 | Versatile | $3.00 | $15.00 | Sterk på norsk, god på planlegging og arkitektur |
| `@nav-pilot-opus` | Claude Opus 4.6 | Powerful | $5.00 | $25.00 | Dypest resonnering for høy-risiko beslutninger |
| `@security-champion` | Claude Opus 4.6 | Powerful | $5.00 | $25.00 | Sikkerhetskritiske vurderinger krever høyeste presisjon |
| `@code-review` | GPT-5.3-Codex | Powerful | $1.75 | $14.00 | Sterkest på kodeforståelse og terminal-oppgaver |
| `@kafka` | GPT-5.3-Codex | Powerful | $1.75 | $14.00 | Teknisk presis på hendelsesdrevne mønstre |
| `@nais` | GPT-5.3-Codex | Powerful | $1.75 | $14.00 | God på infrastruktur og YAML-konfigurasjon |
| `@research` | GPT-5.3-Codex | Powerful | $1.75 | $14.00 | Effektiv på bred kodebase-søk og oppsummering |
| `@rust` | GPT-5.3-Codex | Powerful | $1.75 | $14.00 | Terminal-Bench-leder for kompilert kode |
| `@auth` | Claude Sonnet 4.6 | Versatile | $3.00 | $15.00 | Nyansert på sikkerhetsmønstre og token-flyt |
| `@aksel` | Claude Sonnet 4.6 | Versatile | $3.00 | $15.00 | Sterk på komponentstruktur og designsystem-konvensjoner |
| `@accessibility` | Claude Sonnet 4.6 | Versatile | $3.00 | $15.00 | God på WCAG-tolkning og semantisk HTML |
| `@observability` | Claude Sonnet 4.6 | Versatile | $3.00 | $15.00 | Presis på metrikk-mønstre og PromQL |
| `@forfatter` | Claude Sonnet 4.6 | Versatile | $3.00 | $15.00 | Anthropic-modellene er best på norsk klarspråk |

### Prompts

| Prompt | Modell | Kategori | Input | Output | Begrunnelse |
|--------|--------|----------|-------|--------|-------------|
| `kafka-topic` | GPT-5.3-Codex | Powerful | $1.75 | $14.00 | Konsistent med kafka-agenten |
| `nais-manifest` | GPT-5.3-Codex | Powerful | $1.75 | $14.00 | Konsistent med nais-agenten |
| `aksel-component` | Gemini 3.6 Flash | Versatile | $1.50 | $7.50 | Rask og billig for scaffolding av Aksel-komponenter |
| `ktor-endpoint` | Claude Haiku 4.5 | Versatile | $1.00 | $5.00 | Enkel strukturert mal, trenger ikke tung modell |
| `nextjs-api-route` | Claude Haiku 4.5 | Versatile | $1.00 | $5.00 | Enkel strukturert mal |
| `spring-boot-endpoint` | Claude Haiku 4.5 | Versatile | $1.00 | $5.00 | Enkel strukturert mal |
| `golang-service` | Claude Haiku 4.5 | Versatile | $1.00 | $5.00 | Enkel strukturert mal |

---

## Tilgjengelige modeller og bruksområder

Oversikt over hele modellflåten — ikke bare de som er pinnet i agenter.

| Modell | Kategori | Input | Output | Best for |
|--------|----------|-------|--------|----------|
| Claude Opus 4.6 / 4.8 | Powerful | $5.00 | $25.00 | Dyp risikovurdering, sikkerhetskritisk kode, kompleks arkitektur |
| Claude Sonnet 4.6 | Versatile | $3.00 | $15.00 | Daglig koding, norsk tekst, planlegging |
| Claude Sonnet 5 | Versatile | $2.00 | $10.00 | Samme som Sonnet 4.6, lavere pris (kampanje t.o.m. 31. aug 2026) |
| Claude Haiku 4.5 | Versatile | $1.00 | $5.00 | Sjekklister, maler, scaffold-prompts |
| GPT-5.3-Codex | Powerful | $1.75 | $14.00 | Kodeforståelse, terminal, infrastruktur |
| GPT-5.6 Luna | Lightweight | $1.00 | $6.00 | Raske rutineoppgaver, enkel autofullfør |
| GPT-5.6 Terra | Versatile | $2.50 | $15.00 | Allround daglig koding i GPT-familien |
| GPT-5.6 Sol | Powerful | $5.00 | $30.00 | Tung reasoning over store kodebaser (krever Pro+) |
| Gemini 2.5 Pro | Powerful | $1.25 | $10.00 | Beste pris/ytelse for research og lange kontekstvinduer |
| Gemini 3.5 Flash | Lightweight | $1.50 | $9.00 | Rask og billig for enkle oppgaver |
| Gemini 3.6 Flash | Versatile | $1.50 | $7.50 | Agentiske workflows med parallell verktøybruk |
| Kimi K2.7 Code | Versatile | $0.95 | $4.00 | Rimeligste alternativ for kode-agent-løkker (open-weight) |

Se [prissiden](/priser) for fullstendig og oppdatert pristabell.

---

## Kriterier for å bytte modell

Vi bytter **ikke** modell automatisk når noe nytt lanseres. Et bytte krever at alle tre er oppfylt:

1. **Bekreftet ID** — modellnavnet i `model:`-feltet er verifisert mot faktisk model picker-oppførsel, ikke bare dokumentasjon
2. **Kostnad er lik eller lavere** — eller ytelsesgevinsten er dokumentert og rettferdiggjør økt kostnad
3. **Testet på reell oppgave** — minst én oppgave av typen agenten brukes til, ikke benchmark-tall fra leverandøren

### Eksempel: GPT-5.3-Codex → GPT-5.6 Terra

| Kriterium | Status |
|-----------|--------|
| Bekreftet ID | ❌ Ikke verifisert i model picker |
| Kostnad | ❌ Terra er 43 % dyrere på input ($2.50 vs $1.75) |
| Testet | ❌ Ikke testet |

**Konklusjon:** Ikke byttet. GPT-5.3-Codex beholdes inntil videre.

---

## Sjekkliste for nye modeller

Når nye modeller slås på (som nå med GPT-5.6-familien, Kimi K2.7 og Gemini 3.6 Flash):

- [ ] Bekreft eksakt modell-ID i model picker (ikke bare dokumentasjonsnavn)
- [ ] Sammenlign pris mot eksisterende pinnet modell for samme agent
- [ ] Sjekk om modellen er tilgjengelig på riktig Copilot-plan (Pro vs Pro+/Business)
- [ ] Test på en reell oppgave av typen agenten brukes til
- [ ] Oppdater tabell over pinning og begrunnelse i dette dokumentet
- [ ] Oppdater `model:`-feltet i agent/prompt-filen

---

## Modell-ID-format

Eksisterende observasjon av navnekonvensjoner i `model:`-feltet:

| Modell | Format | Merk |
|--------|--------|------|
| `GPT-5.3-Codex` | Bindestrek mellom versjon og variant | Fungerer |
| `Claude Sonnet 4.6` | Mellomrom | Fungerer |
| `Claude Opus 4.6` | Mellomrom | Fungerer |
| `Gemini 3.5 Flash` | Mellomrom | Fungerer |
| `Gemini 3.6 Flash` | Mellomrom | Antatt — ikke verifisert i praksis |
| `GPT-5.6 Terra` | Mellomrom | Antatt — ikke verifisert i praksis |

Frem til en modell er verifisert i praksis, merkes den som «Antatt» og bør ikke brukes i produksjonspinning.
