---
title: "Nav-pilot lander på bakken"
date: 2026-08-30
featured: true
author: starefossen
category: praksis
excerpt: "En lokal modell kan gjøre mekaniske endringer over flere filer. Selve modellkjøringen bruker ingen AI-credits, men hovedagenten i skyen gjør det fortsatt."
tags:
  - nav-pilot
  - local-models
  - mlx
  - cost
  - alpha
---

Med `nav-pilot alpha local` kan du kjøre en modell fra Qwen-familien på din egen maskin. Vi prøver lokal delegering for å redusere credit-forbruket, men labmålingene viser at det ikke lønner seg på alle oppgaver.

Vi kaller den bakkemodellen. Hovedagenten blir i skya og bestemmer, bakkemodellen utfører. I logger og konfigurasjon heter den `local-worker`.

Den arbeidsdelingen krever opencode som klient. Bare der blir bakkemodellen en underagent hovedagenten kan sende avgrensede oppgaver til. Under Copilot CLI, som er standardklienten i nav-pilot, finnes ingen slik underagent: klienten setter modelleverandøren som en miljøvariabel for hele prosessen, så én leverandør betjener hele økten. Valget der er hele økten på den lokale modellen eller ingenting lokalt. Vi har verifisert det mot Copilot CLI 1.0.83-3. Vil du ha utsending, bytt med `nav-pilot config set client opencode`. Runtimen under Copilot CLI kan ha flere leverandører i én økt, men Copilot CLI lar ikke en agent velge sin egen ennå, og det er ikke dokumentert. Vi tester om det kan tas i bruk ([github/copilot-cli#4703](https://github.com/github/copilot-cli/issues/4703)).

```
nav-pilot alpha local init
```

Det er hele oppsettet. Krever Mac med Apple Silicon og 48 GB minne. Modellen tar 26 GB på disk og 21 GB i minnet mens den kjører. `init` spør om passordet ditt for å heve en minnegrense i macOS.

## Hva det er

Bakkemodellen får oppgaver der beslutningen allerede er tatt: rename av et symbol på tvers av filer, eller et nytt felt som skal gjennom en mapper og alle call sites.

Vi har også kjørt den alene, uten hovedagent over seg. Den klarte en rename av 46 references i 10 filer i begge forsøkene, og prosjektet kompilerte alle gangene.

## Hva det ikke er

Bakkemodellen skal gjennomføre beslutninger hovedagenten allerede har tatt. For standardmodellen er bare `edit-multi-mechanical` merket `trusted` for delegering. Spørsmål, enkel filredigering, nye filer og debugging skal bli i skyen. Qwen 3.8-profilene har ingen delegeringsklasser merket `trusted`.

Dette styrer instruksjonene til hovedagenten, ikke hva den lokale modellen teknisk kan gjøre.

Ikke raskere. Som regel tregere.

Og ingen har brukt den en hel arbeidsdag ennå.

## Hva vi vet

Først prøvde vi åtte modeller og bygg på det samme oppgavesettet: elleve oppgaver med oppslag, redigering og endringer over flere filer.

- **Qwen3.6-35B-A3B, OptiQ 4-bit.** Den vi valgte. Løste flest oppgaver i den første testen. Lengre kjøringer avdekket senere tool-looper, se oppdateringen under.
- **Qwen3.6-35B-A3B, vanlig 4-bit.** Samme modell, annet bygg. Gikk i tool-loop og brukte 220 kall på én oppgave.
- **KAT-Coder V2.5.** Løste like mange, men trengte flere kall på å komme dit.
- **Qwen3.6-27B.** For treg. Median 113 sekunder mot 18.
- **Qwen3.8-27B i 4-, 6- og 8-bit.** Ikke standard, men 4-bit og 8-bit kan nå velges. Se under.
- **Granite 4.1 8B.** For liten. Løste 1 av 8.

### Oppdatering 2. september: forspranget var vårt, ikke modellens

Her sto det til 2. september at Qwen3.8-27B 4-bit løste **5, 5, 6 og 7 av 8** mot
standardmodellens **3, 3, 3, 4 og 4**, og at 3.8 dermed løser mer. Det tallet holder ikke, og
grunnen er verdt å skrive ned.

Sandkassen vi kjører oppgavene i hadde aldri gitt modellene tilgang til byggverktøyene. Ingen av
dem kunne kompilere eller kjøre en test. Vi målte altså ikke hvem som løser oppgaver, vi målte
hvem som skriver best Kotlin i blinde. Da tilgangen kom på plass, og oppgavesettet i tillegg ble
pinnet til én commit i stedet for å følge master, forsvant forspranget: **3,75 mot 3,25 løste
oppgaver, p = 0,71.** Vi fant ingen statistisk signifikant forskjell i dette utvalget.

Qwen3.8 4-bit kan fortsatt velges, og den er fortsatt omtrent sju ganger tregere, median 65
sekunder mot 9. Men vi har ikke lenger noe belegg for at den løser mer, og de gamle tallene over
skal ikke brukes til å velge modell. Nye kjøringer på det reparerte oppsettet pågår.

Vil du prøve den likevel:

```bash
nav-pilot alpha local models
nav-pilot alpha local use qwen3.8-27b-optiq-4bit
nav-pilot alpha local init
```

Vi krever nå minst fem kjøringer før et tall får styre en anbefaling, og at harnesset blir gjennomgått før tallene brukes.

Deretter 200 kjøringer på én maskin med modellen vi valgte, fordelt på to klienter, seks oppgavetyper, tre refactor-strategier og tre kodebaser: en Ktor-app, en Spring-app og en frontend.

I Ktor-repoet kostet oppgaven 13 AI-credits med lokal utsending, mot 34 uten. Testene ga samme resultat. Til gjengjeld tok det 156 sekunder mot 100.

I Spring-repoet snudde det: 16 credits mot 9. Der ble det dyrere å kjøre lokalt, med samme modell og samme oppsett.

I disse laboppgavene hang besparelsen sammen med hvor mange steg skymodellen brukte alene. Målingene viser ikke at kodebasen er uten betydning.

![Jo flere steg skymodellen trenger alene, jo mer sparer du på å sende arbeidet til bakkemodellen. 19 steg sparer 61 prosent, 13 steg sparer 47 prosent, 5 steg sparer 20 prosent, og på 2 steg koster utsendingen 79 prosent mer enn den sparer.](/images/nav-pilot-step-count.svg)

| Skymodellen alene | Med bakkemodellen   |
| ----------------- | ------------------- |
| 19 steg           | sparer 61 %         |
| 13 steg           | sparer 47 %         |
| 5 steg            | sparer 20 %         |
| 2 steg            | **koster 79 % mer** |

I oppgaven der skymodellen brukte to steg, økte lokal delegering credit-forbruket. I de tre oppgavene med flere steg gikk forbruket ned.

Frontend-tallet, det på 13 steg, er det eneste vi målte etter at vi hadde skrevet ned hva vi trodde ville skje. Vi bommet ikke.

Alt dette er målt i lab: enkeltoppgaver, rent repo, ingen avbrytelser, ingen som venter på deg.

### Oppdatering 24. september: lengre kjøringer krevde flere vern

Senere kjøringer avdekket tool-looper på 203 og 220 kall. Den lokale loop-guarden avslutter som standard turen etter fire identiske kall på rad med samme resultat, eller åtte identiske kall selv om resultatet endrer seg.

Serveren avslutter nå hvis genereringstråden krasjer, for eksempel ved Metal OOM. Neste session får en konkret restart-melding i stedet for å koble seg til en prosess som svarer på porten, men aldri leverer et resultat.

Manifestet kan også styre sampling og `--prefill-step-size`. Dette gjør det mulig å rette modellspesifikke problemer uten en ny nav-pilot-release. Verdiene settes først etter målinger. Se oppdateringen under for dem som nå er satt.

Nav-pilot leser nå delegeringsklasser fra modellmanifestet. For standardmodellen er bare `edit-multi-mechanical` godkjent for delegering: mekaniske endringer som følger ett mønster gjennom flere filer. Spørsmål, enkel filredigering, nye filer og debugging blir i skyen.

Qwen 3.8-profilene har foreløpig ingen oppgaveklasse godkjent for delegering. Vurderingen kommer fra minst fem kjøringer og to ulike oppgaver per klasse, med en kvalitetsgrense mot skyarmen.

### Oppdatering 24. september (kveld): verdiene er satt, og Qwen 3.8 kan velges

Standardmodellen kjører nå med temperatur 0,6 og top_p 0,95, de samme verdiene vi målte den med. For Qwen 3.8 setter manifestet ingen temperatur ennå, fordi den målingen ikke er ferdig.

Begge Qwen 3.8-modellene kan velges med `nav-pilot alpha local use`. Ingen av dem blir standard:

- **Qwen3.8-27B 4-bit** har 64k kontekst og svar på inntil 8k tokens. Den er mye tregere enn standard og langt mindre forutsigbar: to kjøringer av de samme oppgavene ga median 88 og 906 sekunder.
- **Qwen3.8-27B 8-bit** har 48k kontekst og svar på inntil 4k tokens. Den leser prompten i steg på 512 tokens for å bruke mindre minne på lange prompter. Før gikk den tom for minne rundt 51k tokens. Den løste 31 av 40 oppgaver mot standardens 28, men bruker omtrent ti ganger så lang tid.

8-bit krever nav-pilot 2026.09.24-110317 eller nyere. Eldre versjoner kjenner ikke steglengden og kan gå tom for minne nær 48k. Manifestet sier nå hvilken nav-pilot hver modell krever, og en eldre nav-pilot skjuler modellen. Peker `local_model` på den, faller nav-pilot tilbake til standardmodellen og sier hvilken versjon du trenger.

Tabellen over modellene på [ki-utvikling.nav.no/nav-pilot/docs](https://ki-utvikling.nav.no/nav-pilot/docs#lokal-modeller) er nå generert fra manifestet, så kontekst, minnekrav og minste versjon følger det nav-pilot selv leser.

Svarer ikke GitHub når du starter en agentpakke som `nais/pilot`, bruker nav-pilot nå manifestet fra forrige vellykkede oppstart og skriver en advarsel. Før feilet hver oppstart, også økter med lokal modell der ingenting annet trenger nett.

## Hva vi trenger fra deg

- Kjør `nav-pilot alpha local status` med en gang noe henger.
- Si fra hvis en endring kompilerer, men er feil på en måte selv en slurvete kollega ikke ville levert.
- Si fra om ventetiden er verdt det midt i arbeidsdagen. Det er det bare du som vet.

«Ikke verdt bryet» er et like nyttig svar som det motsatte, og bedre å få nå enn om et år.

Så lenge dette er alfa, måler vi tettere enn i resten av nav-pilot: hvor mange oppgaver hver økt sender til bakkemodellen, hvilken modell som kjører, oppstartstid og når serveren henger. Vi samler aldri inn koden din eller det du skriver. `DO_NOT_TRACK=1` skrur alt av.

## Bli med

Meld deg i #nav-pilot. Vi tar inn én og én i starten.

Hele rapporten, med metode og alle tallene: [local-inference-findings.md](https://github.com/navikt/mlx-workspace/blob/main/reports/local-inference-findings.md). Hvorfor akkurat denne modellen, og hva vi forkastet: [alpha-model-decision.md](https://github.com/navikt/mlx-workspace/blob/main/reports/alpha-model-decision.md). Rådataene ligger i [navikt/mlx-workspace](https://github.com/navikt/mlx-workspace), også kjøringene som gikk galt.

> **Rettelse 24. september (kveld):** Kommandoen for å bytte til Qwen 3.8 satte `model`, som er modellen økten kjører på. Riktig nøkkel er `local_model`. Saken sa også at valg av leverandør per agent bare lå som en feature request hos GitHub. Runtimen under Copilot CLI kan allerede ha flere leverandører i én økt, og vi tester om det kan tas i bruk.

> **Rettelse 31. august:** Saken oppga først at 8-bit gikk i timeout på 8 av 11 oppgaver, og forklarte deretter tallet med en chat-mal vi skrev selv, men verken tallet eller den forklaringen kan vi stå inne for.

**Kilder:**

- [Exit the server when its generation thread dies](https://github.com/navikt/copilot/pull/931) (navikt/copilot, 24. september 2026)
- [Count a tool call as looping only when its result repeats](https://github.com/navikt/copilot/pull/933) (navikt/copilot, 24. september 2026)
- [Let the manifest set sampling for local models](https://github.com/navikt/copilot/pull/934) (navikt/copilot, 24. september 2026)
- [Let the manifest set mlx-lm's prefill step size](https://github.com/navikt/copilot/pull/936) (navikt/copilot, 24. september 2026)
- [Generate the dispatch policy from manifest capabilities](https://github.com/navikt/copilot/pull/941) (navikt/copilot, 24. september 2026)
- [Score task classes against the local-vs-cloud bar](https://github.com/navikt/mlx-workspace/pull/27) (navikt/mlx-workspace, 24. september 2026)
- [Fall back to the cached agent source when it cannot be fetched](https://github.com/navikt/copilot/pull/942) (navikt/copilot, 24. september 2026)
- [Gate manifest entries on min_nav_pilot](https://github.com/navikt/copilot/pull/943) (navikt/copilot, 24. september 2026)
- [Withheld defaults, non-string minimums and status from cache](https://github.com/navikt/copilot/pull/945) (navikt/copilot, 24. september 2026)
- [8-bit Qwen3.8 at 48k with prefill step 512, and data-based model texts](https://github.com/navikt/mlx-workspace/pull/28) (navikt/mlx-workspace, 24. september 2026)
- [Set min_nav_pilot from the params an entry uses](https://github.com/navikt/mlx-workspace/pull/30) (navikt/mlx-workspace, 24. september 2026)
- [Run the default model at temperature 0.6, as benchmarked](https://github.com/navikt/mlx-workspace/pull/31) (navikt/mlx-workspace, 24. september 2026)
- [Generer tabellen over lokale modeller fra manifestet](https://github.com/navikt/copilot/pull/947) (navikt/copilot, 24. september 2026)
