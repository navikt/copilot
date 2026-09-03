# Modellvalg i Nav Copilot

Levende referansedokument for hvilke modeller vi bruker, hvorfor, og hvordan vi vurderer oppdateringer.

## Gjeldende modellpinning

De fleste agenter og prompts har et eksplisitt `model:`-felt i YAML-frontmatter. `nav-pilot` har det ikke: orkestratoren kjører på klientens egen standardmodell. Valget følger oppgavetype, kostnad og ytelse, ikke leverandørpreferanse. Priser og kategori står i modelltabellen under.

### Agenter

| Agent | Modell | Begrunnelse |
|-------|--------|-------------|
| `@nav-pilot` | Klientens standardmodell | Orkestratoren pinnes ikke; den arver modellen brukeren allerede kjører i klienten |
| `@nav-pilot-opus` | GPT-5.6 Sol | Tung resonnering for høy-risiko beslutninger. Billigere enn Opus 4.6 på begge akser under 272K kontekst; over 272K også billigere til kampanjepris, men dyrere til antatt standardpris etter 3. sep 2026. Agenten er ikke målt mot Opus 4.6 |
| `@security-champion` | GPT-5.6 Sol | Sikkerhetskritiske vurderinger. Agenten er ikke målt mot Opus 4.6 eller mot noen annen modell. Byttet hviler på pris |
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
GitHubs listepriser slik de sto **3. september 2026**, og speiler
`apps/my-copilot/src/lib/model-pricing.ts`. De endrer seg uten varsel, så
tallene her har et tidsstempel og ikke evig gyldighet.

| Modell | Kategori | Input | Output | Best for |
|--------|----------|-------|--------|----------|
| Claude Opus 5 | Powerful | $5.00 | $25.00 | Dyp resonnering, risikovurdering og sikkerhetskritisk kode med justerbar effort (low/medium/high). Lansert 24. juli 2026 |
| Claude Opus 4.6 / 4.8 | Powerful | $5.00 | $25.00 | Dyp risikovurdering, sikkerhetskritisk kode, kompleks arkitektur |
| Claude Sonnet 4.6 | Versatile | $3.00 | $15.00 | Daglig koding, norsk tekst, planlegging |
| Claude Sonnet 5 | Versatile | $2.00 | $10.00 | Samme som Sonnet 4.6. ⚠️ Kampanjen vi noterte gikk ut 31. aug 2026, og standardprisen er ukjent. Se noten under tabellen |
| Claude Haiku 4.5 | Versatile | $1.00 | $5.00 | Sjekklister, maler, scaffold-prompts |
| GPT-5.3-Codex | Powerful | $1.75 | $14.00 | Kodeforståelse, terminal, infrastruktur |
| GPT-5.6 Luna | Lightweight | $0.20 | $1.20 | Raske rutineoppgaver, enkel autofullfør. OpenAI plasserer den i nano-sjiktet fra tidligere GPT-5-familier, men med høy reasoning-rating og justerbar effort |
| GPT-5.6 Terra | Versatile | $2.00 | $12.00 | Allround daglig koding i GPT-familien |
| GPT-5.6 Sol | Powerful | $2.00 | $10.00 | Tung reasoning over store kodebaser (krever Pro+). ⚠️ Kampanjepris t.o.m. 3. sep 2026, antatt standardpris $4.00 / $20.00. Se noten under tabellen |
| Gemini 2.5 Pro | Powerful | (utgått) | (utgått) | 🚫 Utfaset 31. juli 2026 og borte fra GitHubs prisliste. Bruk Gemini 3.1 Pro for research og lange kontekstvinduer |
| Gemini 3.5 Flash | Lightweight | $1.50 | $9.00 | Rask og billig for enkle oppgaver |
| Gemini 3.6 Flash | Versatile | $0.75 | $3.75 | Agentiske workflows med parallell verktøybruk. Kampanjepris t.o.m. 31. des 2026 |
| Kimi K2.7 Code | Versatile | $0.95 | $4.00 | Rimeligste alternativ for kode-agent-løkker (open-weight) |

**Kampanjepriser.** GitHub merker enkelte rader med kampanjepris i fotnoter, og
fotnotene følger ikke med når vi synkroniserer pristabellen
([#503](https://github.com/navikt/copilot/issues/503)). Per 31. august 2026
gjelder det:

- **GPT-5.6 Sol:** 50 % av standardpris t.o.m. 3. september 2026. Fotnoten
  oppgir ikke standardprisen. Doblet kampanjepris gir $4.00 input og $20.00
  output ($8.00 / $30.00 for lang kontekst over 272K), og det er utregning fra
  «50 % off», ikke en pris vi har sett publisert. Sol lå på $5.00 / $30.00 i
  pristabellen vår fram til synkroniseringen 30. august, så listeprisen ser ut
  til å ha blitt satt ned og deretter fått en kampanje på toppen. Verifiser
  mot kilden 4. september.
- **Gemini 3.6 Flash og Gemini 3.7 Flash:** $0.75 input og $3.75 output t.o.m.
  31. desember 2026. Standardprisen står ikke i fotnoten. Gemini 3.7 Flash er
  ikke pinnet noe sted hos oss og står derfor ikke i tabellen over.
- **Claude Sonnet 5:** notatet vårt sa kampanje t.o.m. 31. august 2026. GitHubs
  pristabell viser fortsatt $2.00 / $10.00 og har ingen fotnote for Sonnet 5, så
  vi kan hverken bekrefte kampanjen eller finne standardprisen. Tallet skal
  verifiseres mot kilden før det brukes i et regnestykke.
- **GPT-5.6 Luna: ikke kampanjepris.** Sjekket særskilt fordi $0.20 / $1.20 er
  80 % under de $1.00 / $6.00 som stod i juli-artiklene våre, og fordi sju
  pinninger hviler på tallet. OpenAIs egen modellside for `gpt-5.6-luna` oppgir
  $0.20 input, $0.02 cachet input og $1.20 output som listepris, uten
  kampanjeformuleringer. GitHubs pristabell, som er kilden vi synkroniserer fra,
  har ingen fotnote på Luna-raden. Ingen av Luna-pinningene har altså en
  utløpsdato.

Se [prissiden](/priser) for fullstendig og oppdatert pristabell.

## Grunnlaget for Sol-byttet (august 2026)

`@security-champion` og `@nav-pilot-opus` går fra Claude Opus 4.6 til GPT-5.6
Sol. **Byttet hviler på pris. Disse to agentene er ikke målt mot noen modell,
heller ikke mot den de flytter fra.**

Golden-prompt-harnessen kjørte nav-pilot-personaen mot Claude Sonnet 4.6,
GPT-5.6 Sol, GPT-5.6 Luna og GPT-5.6 Terra. Opus 4.6, modellen disse to
agentene faktisk kjører på i dag, var ikke med i målingen, og harnessen tester
personaen til `nav-pilot`, ikke `@security-champion` og ikke `@nav-pilot-opus`.
Det som ble målt er én regex-påstand på én prompt, og der skilte Sol seg ikke
fra Claude Sonnet 4.6 (Fisher p = 1,00). Det betyr umulig å skille, ikke
likeverdig. Tall, metode og forbehold står i
[benchmarken og beslutningene fra august 2026](nav-pilot-benchmark-og-beslutninger-2026-08.md).

### Prisen er en kampanjepris

GitHub oppgir i en fotnote på prissiden (anker
`#user-content-fn-gpt-56-sol-promo`) at GPT-5.6 Sol ligger på **50 prosent
avslag til og med 3. september 2026**. Kampanjeprisen for standardvinduet er
$2.00 input og $10.00 output, som er tallene i tabellen over. Full pris er
dermed $4.00 og $20.00. **Det tallet er regnet ut fra fotnoten, ikke en pris
GitHub har oppgitt direkte, og ikke en pris dette repoet har hatt liggende.**

Sammenlikningen under er blandet pris per million tokens ved **10 input-tokens
per output-token. Forholdet er et anslag, ikke noe vi har målt**, og varierer
med oppgaven.

| Modell | Input | Output | Blandet $/1M ved 10:1 (anslag) |
|--------|-------|--------|-------------------------------|
| GPT-5.6 Sol, kampanje t.o.m. 3. sep 2026 | $2.00 | $10.00 | 2,73 |
| Claude Sonnet 4.6 | $3.00 | $15.00 | 4,09 |
| GPT-5.6 Sol, full pris | $4.00 | $20.00 | 5,45 |
| Claude Opus 4.6 | $5.00 | $25.00 | 6,82 |

### Hva byttet faktisk sparer

Den riktige sammenlikningen er mot Claude Opus 4.6, som er modellen disse to
agentene kjører på i dag. **Under 272K kontekst** er Sol billigere enn Opus 4.6
på begge akser både med og uten kampanje: $2.00 mot $5.00 og $10.00 mot $25.00
nå, og $4.00 mot $5.00 og $20.00 mot $25.00 etter 3. september. Det er omtrent
20 prosent billigere på begge akser når kampanjen er over, og den delen av
gevinsten overlever kampanjeslutt.

**Over 272K snur det.** Sol har et eget prisnivå for lang kontekst, Opus 4.6 har
ikke det og koster $5.00 / $25.00 uansett kontekstlengde. Til kampanjepris
ligger Sol fortsatt under ($4.00 mot $5.00 og $15.00 mot $25.00), men til antatt
standardpris, $8.00 / $30.00 (se noten om kampanjepriser over), er Sol dyrere
enn Opus 4.6 på begge akser. Begge disse to agentene er pitchet mot tung
resonnering over store kodebaser, altså nettopp arbeidslasten som oftest krysser
272K. Prisgevinsten gjelder korte kontekster, ikke lange.

Mot Claude Sonnet 4.6 er bildet et annet, og det skal ikke brukes som
begrunnelse. Til kampanjepris ligger Sol 33 prosent under Sonnet 4.6 blandet,
men til full pris ligger Sol 33 prosent **over**. Sonnet 4.6 er heller ikke
modellen disse agentene erstatter.

### Kostnadsregelen peker ikke på Sol

Regelen «når sikkerhet ikke skiller dem, avgjør kostnad» velger ikke Sol. Til
antatt standardpris ligger Sol på 5,45 blandet, mens GPT-5.3-Codex ligger på
2,86 og Gemini 3.1 Pro på 2,91. Begge er Powerful-modeller, og begge er
billigere enn Sol. Fulgt bokstavelig peker regelen på en av dem, ikke på Sol.

Sol er valgt fordi den er nærmeste erstatter for Opus 4.6 i resonneringssjiktet.
**Det er en vurdering, ikke en måling.** Vi har ingen tall som viser at Sol
resonnerer bedre enn GPT-5.3-Codex eller Gemini 3.1 Pro på oppgavene disse to
agentene gjør, og ingen som viser at den holder Opus 4.6-nivået. Argumentet er
ubelagt, og skal leses som det.

Merk at Sol krever Copilot Pro+ eller høyere plan.

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

> **Notat (24. juli 2026, oppdatert 31. august 2026):** Claude Opus 5 (`claude-opus-5`) er lansert av Anthropic og var kandidat til å erstatte Opus 4.6-pinningene på `@nav-pilot-opus` og `@security-champion`. Listeprisen er identisk med Opus 4.8 ($5.00/$25.00), og Anthropic oppgir vesentlig sterkere resonnering (mer enn dobling av Opus 4.8 på Frontier-Bench v0.1). Begge agentene står nå på GPT-5.6 Sol, som er billigere enn Opus 4.6 på begge akser under 272K kontekst også etter at kampanjeprisen løper ut 3. september 2026; over 272K er Sol til antatt standardpris dyrere enn Opus 4.6 på begge akser. Opus 5 er fortsatt aktuell hvis en måling viser at den tyngre resonneringen er verdt prisforskjellen, men den er ikke testet mot vår egen golden-prompt. Utrullingen i Copilot er gradvis (GA for Pro+/Max/Business/Enterprise 24. juli), så modellen kan mangle i model picker en periode.

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
| `GPT-5.6 Sol` | Mellomrom | Verifisert gjennom golden-prompt-kjøringene (august 2026) |
| `Claude Opus 5` | Mellomrom | Verifisert (GitHub changelog / Anthropic, 24. juli 2026). API-ID `claude-opus-5` |
| `Gemini 3.5 Flash` | Mellomrom | Fungerer |
| `Gemini 3.6 Flash` | Mellomrom | Antatt, ikke verifisert i praksis |
| `GPT-5.6 Luna` | Mellomrom | Verifisert gjennom golden-prompt-kjøringene (august 2026) |
| `GPT-5.6 Terra` | Mellomrom | Verifisert gjennom golden-prompt-kjøringene, men ikke pinnet |

Frem til en modell er verifisert i praksis, merkes den som «Antatt» og bør ikke brukes i produksjonspinning.
