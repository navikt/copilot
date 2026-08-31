# Modellvalg i Nav Copilot

Levende referansedokument for hvilke modeller vi bruker, hvorfor, og hvordan vi vurderer oppdateringer.

## Gjeldende modellpinning

De fleste agenter og prompts har et eksplisitt `model:`-felt i YAML-frontmatter. `nav-pilot` og `local-worker` har det ikke: orkestratoren kjører på klientens egen standardmodell, og `local-worker` bindes til den lokale modellen ved oppstart og skal ikke pinnes. Valget følger oppgavetype, kostnad og ytelse, ikke leverandørpreferanse. Priser og kategori står i modelltabellen under.

### Agenter

| Agent | Modell | Begrunnelse |
|-------|--------|-------------|
| `@nav-pilot` | Claude Sonnet 4.6 | Sterk på norsk, god på planlegging og arkitektur |
| `@nav-pilot-opus` | Claude Opus 4.6 | Dypest resonnering for høy-risiko beslutninger |
| `@security-champion` | Claude Opus 4.6 | Sikkerhetskritiske vurderinger krever høyeste presisjon |
| `@code-review` | GPT-5.3-Codex | Sterkest på kodeforståelse og terminal-oppgaver |
| `@kafka` | GPT-5.3-Codex | Teknisk presis på hendelsesdrevne mønstre |
| `@nais` | GPT-5.3-Codex | God på infrastruktur og YAML-konfigurasjon |
| `@research` | GPT-5.3-Codex | Effektiv på bred kodebase-søk og oppsummering |
| `@rust` | GPT-5.3-Codex | Terminal-Bench-leder for kompilert kode |
| `@auth` | Claude Sonnet 4.6 | Nyansert på sikkerhetsmønstre og token-flyt |
| `@aksel` | Claude Sonnet 4.6 | Sterk på komponentstruktur og designsystem-konvensjoner |
| `@accessibility` | Claude Sonnet 4.6 | God på WCAG-tolkning og semantisk HTML |
| `@observability` | Claude Sonnet 4.6 | Presis på metrikk-mønstre og PromQL |
| `@forfatter` | Claude Sonnet 4.6 | Anthropic-modellene er best på norsk klarspråk |

### Prompts

| Prompt | Modell | Begrunnelse |
|--------|--------|-------------|
| `kafka-topic` | GPT-5.3-Codex | Konsistent med kafka-agenten |
| `nais-manifest` | GPT-5.3-Codex | Konsistent med nais-agenten |
| `aksel-component` | Gemini 3.6 Flash | Rask og billig for scaffolding av Aksel-komponenter |
| `ktor-endpoint` | Claude Haiku 4.5 | Enkel strukturert mal, trenger ikke tung modell |
| `nextjs-api-route` | Claude Haiku 4.5 | Enkel strukturert mal |
| `spring-boot-endpoint` | Claude Haiku 4.5 | Enkel strukturert mal |
| `golang-service` | Claude Haiku 4.5 | Enkel strukturert mal |

## Tilgjengelige modeller og bruksområder

Hele modellflåten, ikke bare de som er pinnet i agenter.

| Modell | Kategori | Input | Output | Best for |
|--------|----------|-------|--------|----------|
| Claude Opus 5 | Powerful | $5.00 | $25.00 | Dyp resonnering, risikovurdering og sikkerhetskritisk kode med justerbar effort (low/medium/high). Lansert 24. juli 2026 |
| Claude Opus 4.6 / 4.8 | Powerful | $5.00 | $25.00 | Dyp risikovurdering, sikkerhetskritisk kode, kompleks arkitektur |
| Claude Sonnet 4.6 | Versatile | $3.00 | $15.00 | Daglig koding, norsk tekst, planlegging |
| Claude Sonnet 5 | Versatile | $2.00 | $10.00 | Samme som Sonnet 4.6, lavere pris (kampanje t.o.m. 31. aug 2026) |
| Claude Haiku 4.5 | Versatile | $1.00 | $5.00 | Sjekklister, maler, scaffold-prompts |
| GPT-5.3-Codex | Powerful | $1.75 | $14.00 | Kodeforståelse, terminal, infrastruktur |
| GPT-5.6 Luna | Lightweight | $0.20 | $1.20 | Raske rutineoppgaver, enkel autofullfør |
| GPT-5.6 Terra | Versatile | $2.00 | $12.00 | Allround daglig koding i GPT-familien |
| GPT-5.6 Sol | Powerful | $2.00 | $10.00 | Tung reasoning over store kodebaser (krever Pro+) |
| Gemini 2.5 Pro | Powerful | (utgått) | (utgått) | 🚫 Utfaset 31. juli 2026 og borte fra GitHubs prisliste. Bruk Gemini 3.1 Pro for research og lange kontekstvinduer |
| Gemini 3.5 Flash | Lightweight | $1.50 | $9.00 | Rask og billig for enkle oppgaver |
| Gemini 3.6 Flash | Versatile | $0.75 | $3.75 | Agentiske workflows med parallell verktøybruk |
| Kimi K2.7 Code | Versatile | $0.95 | $4.00 | Rimeligste alternativ for kode-agent-løkker (open-weight) |

Se [prissiden](/priser) for fullstendig og oppdatert pristabell.

## Kriterier for å bytte modell

Vi bytter **ikke** modell automatisk når noe nytt lanseres. Et bytte krever at alle tre er oppfylt:

1. **Bekreftet ID.** Modellnavnet i `model:`-feltet er verifisert mot faktisk model picker-oppførsel, ikke bare dokumentasjon.
2. **Kostnad er lik eller lavere.** Eller: ytelsesgevinsten er dokumentert og rettferdiggjør økt kostnad.
3. **Testet på reell oppgave.** Minst én oppgave av typen agenten brukes til, ikke benchmark-tall fra leverandøren.

### Eksempel: GPT-5.3-Codex → GPT-5.6 Terra

| Kriterium | Status |
|-----------|--------|
| Bekreftet ID | ❌ Ikke verifisert i model picker |
| Kostnad | ⚖️ Jevnt, se regnestykket under |
| Testet | ❌ Ikke testet |

Terra koster $2.00 mot Codex $1.75 på input, men $12.00 mot $14.00 på output, så hvilken som er billigst avhenger av blandingen. Terra er billigere ved alt under åtte input-tokens per output-token, og 1,6 % dyrere ved 10:1 ($2,91 mot $2,86 per million tokens). **Forholdet 10:1 er et anslag, ikke noe vi har målt.** Konklusjonen tåler hele spennet uansett: forskjellen er noen få prosent i begge retninger, og kostnad er ikke lenger et argument mot Terra.

Regnestykket ser bort fra cachet input, der Codex ligger på $0.175 mot Terras $0.20. Cachet input dominerer agentiske løkker, så det trekker i motsatt retning av output-prisen. Skal noen bytte på kostnad alene, er det den blandingen som må måles først.

**Konklusjon:** ikke byttet. GPT-5.3-Codex beholdes inntil videre.

## Sjekkliste for nye modeller

> **Notat (24. juli 2026):** Claude Opus 5 (`claude-opus-5`) er lansert av Anthropic og er kandidat til å erstatte Opus 4.6-pinningene på `@nav-pilot-opus` og `@security-champion`. Listeprisen er identisk med Opus 4.8 ($5.00/$25.00), og Anthropic oppgir vesentlig sterkere resonnering (mer enn dobling av Opus 4.8 på Frontier-Bench v0.1). Et bytte skal gjennom sjekklisten under før pinningene endres. Foreløpig beholdes Opus 4.6. Utrullingen i Copilot er gradvis (GA for Pro+/Max/Business/Enterprise 24. juli), så modellen kan mangle i model picker en periode.

Når nye modeller slås på (som nå med Claude Opus 5, GPT-5.6-familien, Kimi K2.7 og Gemini 3.6 Flash):

- [ ] Bekreft eksakt modell-ID i model picker (ikke bare dokumentasjonsnavn)
- [ ] Sammenlign pris mot eksisterende pinnet modell for samme agent
- [ ] Sjekk om modellen er tilgjengelig på riktig Copilot-plan (Pro vs Pro+/Business)
- [ ] Test på en reell oppgave av typen agenten brukes til
- [ ] Oppdater tabell over pinning og begrunnelse i dette dokumentet
- [ ] Oppdater `model:`-feltet i agent/prompt-filen

## Modell-ID-format

Slik ser navnekonvensjonene i `model:`-feltet ut i dag:

| Modell | Format | Merk |
|--------|--------|------|
| `GPT-5.3-Codex` | Bindestrek mellom versjon og variant | Fungerer |
| `Claude Sonnet 4.6` | Mellomrom | Fungerer |
| `Claude Opus 4.6` | Mellomrom | Fungerer |
| `Claude Opus 5` | Mellomrom | Verifisert (GitHub changelog / Anthropic, 24. juli 2026). API-ID `claude-opus-5` |
| `Gemini 3.5 Flash` | Mellomrom | Fungerer |
| `Gemini 3.6 Flash` | Mellomrom | Antatt, ikke verifisert i praksis |
| `GPT-5.6 Terra` | Mellomrom | Antatt, ikke verifisert i praksis |

Frem til en modell er verifisert i praksis, merkes den som «Antatt» og bør ikke brukes i produksjonspinning.
