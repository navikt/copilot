# Modellvalg i Nav Copilot

Levende referansedokument for hvilke modeller vi bruker, hvorfor, og hvordan vi vurderer oppdateringer.

## Gjeldende modellpinning

De fleste agenter og prompts har et eksplisitt `model:`-felt i YAML-frontmatter. `nav-pilot` har det ikke: orkestratoren kjører på klientens egen standardmodell. Valget følger oppgavetype, kostnad og ytelse, ikke leverandørpreferanse. Priser og kategori står i modelltabellen under.

### Agenter

| Agent | Modell | Begrunnelse |
|-------|--------|-------------|
| `@nav-pilot` | Klientens standardmodell | Orkestratoren pinnes ikke; den arver modellen brukeren allerede kjører i klienten |
| `@nav-pilot-opus` | Claude Opus 4.6 | Dypest resonnering for høy-risiko beslutninger |
| `@security-champion` | Claude Opus 4.6 | Sikkerhetskritiske vurderinger krever høyeste presisjon |
| `@code-review` | GPT-5.3-Codex | Sterkest på kodeforståelse og terminal-oppgaver |
| `@kafka` | GPT-5.3-Codex | Teknisk presis på hendelsesdrevne mønstre |
| `@research` | GPT-5.6 Luna | Leser og søker uten å skrive kode, og Luna koster omtrent en tiendedel av Codex |
| `@rust` | GPT-5.3-Codex | Terminal-Bench-leder for kompilert kode |
| `@aksel` | Claude Sonnet 4.6 | Sterk på komponentstruktur og designsystem-konvensjoner |
| `@accessibility` | Claude Sonnet 4.6 | God på WCAG-tolkning og semantisk HTML |
| `@forfatter` | Claude Sonnet 4.6 | Anthropic-modellene er best på norsk klarspråk |

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

## Grunnlaget for Luna-byttene (august 2026)

Golden-prompt-harnessen kjørte nav-pilot-personaen mot Claude Sonnet 4.6,
GPT-5.6 Sol, GPT-5.6 Luna og GPT-5.6 Terra på samme oppgave. Tall, metode og
forbehold står i
[benchmarken og beslutningene fra august 2026](nav-pilot-benchmark-og-beslutninger-2026-08.md).
Kortversjonen: ingen av modellene skilte seg signifikant fra Claude Sonnet 4.6
på den ene påkrevde påstanden som ble målt. Målingen viser altså ikke at Luna
er tryggere, den viser at kandidatene ikke lot seg skille med dette utvalget.
Når sikkerhet ikke skiller dem, avgjør kostnad.

Blandet pris under forutsetter **10 input-tokens per output-token. Det er et
anslag, ikke noe vi har målt**, og forholdet varierer med oppgaven.

| Modell | Input | Output | Blandet $/1M ved 10:1 (anslag) |
|--------|-------|--------|-------------------------------|
| GPT-5.6 Luna | $0.20 | $1.20 | 0,29 |
| Claude Haiku 4.5 | $1.00 | $5.00 | 1,36 |
| GPT-5.3-Codex | $1.75 | $14.00 | 2,86 |
| Claude Sonnet 4.6 | $3.00 | $15.00 | 4,09 |

### Hva som flyttes

- `@research` går fra GPT-5.3-Codex til GPT-5.6 Luna. Agenten er lesetilgang
  alene: verktøylista er `read`, `search`, `web` og lesende GitHub-MCP-kall,
  uten `execute`, `edit` eller `runSubagent`. Den samler inn og oppsummerer,
  den skriver ikke kode. Luna ligger omtrent 90 prosent under Codex blandet.
- De fire malpromptene som sto på Claude Haiku 4.5, går til Luna:
  `ktor-endpoint`, `nextjs-api-route`, `spring-boot-endpoint` og
  `golang-service`. Alle fyller ut en fast mal. Luna er omtrent en femtedel av
  Haiku 4.5 blandet, og omtrent en fjortendedel av Sonnet 4.6.

### Hva som ikke flyttes hit

- `@code-review` og `@accessibility` står igjen på henholdsvis GPT-5.3-Codex og
  Claude Sonnet 4.6. De ble opprinnelig foreslått til Luna som lesende
  mønsteranvendere, men det stemmer ikke: `@code-review` har `execute`, og
  `@accessibility` har `execute`, `edit` og `runSubagent`. De kjører altså
  kommandoer, skriver filer og starter underagenter. GitHub plasserer Luna i
  Lightweight-klassen, og målingen dekket bare nav-pilot-personaen, aldri en
  verktøytung agent. Byttet er derfor ubelagt og måles separat.
- `@forfatter` beholder Claude Sonnet 4.6. Jobben er å skille bokmål fra
  nynorsk og luke ut norske AI-markører. Målingen sier ingenting om det, og
  gevinsten er nær null mot en kjent nedside.
- Resten av GPT-5.3-Codex-pinningene står urørt. Å flytte dem er en egen
  beslutning som denne målingen ikke gir grunnlag for.

## Tilgjengelige modeller og bruksområder

Hele modellflåten, ikke bare de som er pinnet i agenter. Prisene under er
GitHubs listepriser slik de sto **30. august 2026**, og speiler
`apps/my-copilot/src/lib/model-pricing.ts`. De endrer seg uten varsel, så
tallene her har et tidsstempel og ikke evig gyldighet.

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
| Testet | ⚠️ Testet på nav-pilot-personaen (45 kjøringer), ikke på en kodegjennomgangsoppgave |

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
| `GPT-5.6 Luna` | Mellomrom | Verifisert gjennom golden-prompt-kjøringene (august 2026) |
| `GPT-5.6 Terra` | Mellomrom | Verifisert gjennom golden-prompt-kjøringene, men ikke pinnet |

Frem til en modell er verifisert i praksis, merkes den som «Antatt» og bør ikke brukes i produksjonspinning.
