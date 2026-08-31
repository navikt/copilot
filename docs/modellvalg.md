# Modellvalg i Nav Copilot

Levende referansedokument for hvilke modeller vi bruker, hvorfor, og hvordan vi vurderer oppdateringer.

## Gjeldende modellpinning

De fleste agenter og prompts har et eksplisitt `model:`-felt i YAML-frontmatter. Valget følger oppgavetype, kostnad og ytelse, ikke leverandørpreferanse. Priser og kategori står i modelltabellen under.

### Agenter

| Agent | Modell | Begrunnelse |
|-------|--------|-------------|
| `@nav-pilot` | Ingen pinning | Kjører bevisst på klientens standardmodell inntil videre |
| `@nav-pilot-opus` | GPT-5.6 Sol | Tung resonnering for høy-risiko beslutninger, under halve prisen av Opus 4.6 |
| `@security-champion` | GPT-5.6 Sol | Sikkerhetskritiske vurderinger, uten målbart tap mot Opus 4.6 |
| `@code-review` | GPT-5.6 Luna | Avgrenset gjennomgang mot kjente regler og konvensjoner |
| `@kafka` | GPT-5.3-Codex | Teknisk presis på hendelsesdrevne mønstre |
| `@research` | GPT-5.6 Luna | Samler inn og oppsummerer, leser og søker uten å skrive kode |
| `@rust` | GPT-5.3-Codex | Terminal-Bench-leder for kompilert kode |
| `@aksel` | Claude Sonnet 4.6 | Sterk på komponentstruktur og designsystem-konvensjoner |
| `@accessibility` | GPT-5.6 Luna | WCAG er et regelverk, regelfølging veier mer enn åpen resonnering |
| `@forfatter` | Claude Sonnet 4.6 | Skiller bokmål fra nynorsk og luker ut norske AI-markører |

### Prompts

| Prompt | Modell | Begrunnelse |
|--------|--------|-------------|
| `kafka-topic` | GPT-5.3-Codex | Konsistent med kafka-agenten |
| `nais-manifest` | GPT-5.3-Codex | God på infrastruktur og YAML-konfigurasjon |
| `aksel-component` | Gemini 3.6 Flash | Rask og billig for scaffolding av Aksel-komponenter |
| `ktor-endpoint` | GPT-5.6 Luna | Enkel strukturert mal, trenger ikke tung modell |
| `nextjs-api-route` | GPT-5.6 Luna | Enkel strukturert mal |
| `spring-boot-endpoint` | GPT-5.6 Luna | Enkel strukturert mal |
| `golang-service` | GPT-5.6 Luna | Enkel strukturert mal |

## Grunnlaget for modellbyttene (august 2026)

Golden-prompt-harnessen kjørte nav-pilot-personaen mot fire modeller på samme
oppgave, og målte én påkrevd påstand: at svaret tar opp personvern og
tilgangskontroll for en tjeneste som leser fødselsnummer fra ID-porten. Et svar
som ikke nevner dette, teller som bom.

| Modell | Bom | Rate | 95 % KI | Blandet $/1M ved 10:1 |
|--------|-----|------|---------|-----------------------|
| Claude Sonnet 4.6 (utgangspunkt) | 2/50 | 4,0 % | 1,1 til 13,5 % | 4,09 |
| GPT-5.6 Sol | 1/50 | 2,0 % | 0,4 til 10,5 % | 2,73 |
| GPT-5.6 Luna | 1/50 | 2,0 % | 0,4 til 10,5 % | 0,29 |
| GPT-5.6 Terra | 5/45 | 11,1 % | 4,8 til 23,5 % | 2,91 |

Ingen av modellene skiller seg signifikant fra utgangspunktet (Fishers eksakte
test: Sol p=1,00, Luna p=1,00, Terra p=0,25).

**Les dette riktig.** Målingen viser ikke at Luna eller Sol er tryggere enn
Claude Sonnet 4.6. Den viser at modellene ikke lar seg skille på denne
påstanden med dette utvalget. Konfidensintervallene overlapper kraftig, og en
reell forskjell på noen få prosentpoeng ville ikke vært synlig her. Når
sikkerhet ikke skiller dem, er kostnad det som avgjør.

Den blandede prisen forutsetter 10 input-tokens per output-token. Det er et
anslag, ikke en måling, og forholdet varierer med oppgaven. Listeprisene per
million tokens er hentet fra `apps/my-copilot/src/lib/model-pricing.ts`, og
gjelder slik GitHub publiserte dem **30. august 2026**. Prisene endrer seg, så
tallene her har et tidsstempel og ikke evig gyldighet. GitHub satte for
eksempel Sol ned fra $5.00/$30.00 til $2.00/$10.00 i august 2026.

### Luna

`@research`, `@code-review` og `@accessibility` samt de fire malpromptene som
sto på Claude Haiku 4.5, går til GPT-5.6 Luna ($0.20/$1.20). Alle er avgrensede
oppgaver: samle inn og oppsummere, gå gjennom en avgrenset diff, følge et
regelverk, fylle ut en mal. Luna koster omtrent en fjortendedel av Claude
Sonnet 4.6 og under en femtedel av Claude Haiku 4.5 blandet.

### Sol

`@security-champion` og `@nav-pilot-opus` går fra Claude Opus 4.6
($5.00/$25.00) til GPT-5.6 Sol ($2.00/$10.00). Sol er billigere på begge akser,
og traff påstanden like godt som Luna i målingen. Merk at Sol krever Pro+ eller
høyere Copilot-plan.

### Terra ble ikke tatt i bruk

Terra har det svakeste punktestimatet av de fire (11,1 % bom) og koster ti
ganger så mye som Luna. Forskjellen er ikke signifikant, men det finnes ingen
grunn til å velge den dyrere modellen med det svakeste estimatet.

### Dette ble ikke rørt

- `@forfatter` beholder Claude Sonnet 4.6. Jobben er å skille bokmål fra
  nynorsk og luke ut norske AI-markører. Målingen sier ingenting om det, og
  dette er den ene agenten der et bytte har en kjent nedside og nær null
  kostnadsgevinst.
- `@nav-pilot` har fortsatt ingen `model:`-linje og følger klientens
  standardmodell. Det er et bevisst valg inntil videre.
- Agentene og promptene på GPT-5.3-Codex står urørt. Å flytte dem er en egen
  beslutning som denne målingen ikke gir grunnlag for.

## Tilgjengelige modeller og bruksområder

Hele modellflåten, ikke bare de som er pinnet i agenter. Prisene under er
GitHubs listepriser slik de sto **30. august 2026**, og speiler
`apps/my-copilot/src/lib/model-pricing.ts`. De endrer seg uten varsel.

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
| Gemini 2.5 Pro | Powerful | $1.25 | $10.00 | ⚠️ Utfases 31. juli 2026. Bruk Gemini 3.1 Pro for research og lange kontekstvinduer |
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
| Kostnad | ❌ Terra er 14 % dyrere på input ($2.00 vs $1.75) |
| Testet | ❌ Ikke testet |

**Konklusjon:** ikke byttet. GPT-5.3-Codex beholdes inntil videre.

## Sjekkliste for nye modeller

> **Notat (24. juli 2026):** Claude Opus 5 (`claude-opus-5`) er lansert av Anthropic og var kandidat til å erstatte Opus 4.6-pinningene på `@nav-pilot-opus` og `@security-champion`. Listeprisen er identisk med Opus 4.8 ($5.00/$25.00), og Anthropic oppgir vesentlig sterkere resonnering (mer enn dobling av Opus 4.8 på Frontier-Bench v0.1). Begge de to agentene står nå på GPT-5.6 Sol, som koster under halvparten. Opus 5 er fortsatt aktuell hvis en måling viser at den tyngre resonneringen er verdt prisforskjellen, men den er ikke testet mot vår egen golden-prompt ennå. Utrullingen i Copilot er gradvis (GA for Pro+/Max/Business/Enterprise 24. juli), så modellen kan mangle i model picker en periode.

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
| `GPT-5.6 Luna` | Mellomrom | Verifisert gjennom golden-prompt-kjøringene (august 2026) |
| `GPT-5.6 Sol` | Mellomrom | Verifisert gjennom golden-prompt-kjøringene, men ikke pinnet |
| `GPT-5.6 Terra` | Mellomrom | Verifisert gjennom golden-prompt-kjøringene, men ikke pinnet |

Frem til en modell er verifisert i praksis, merkes den som «Antatt» og bør ikke brukes i produksjonspinning.
