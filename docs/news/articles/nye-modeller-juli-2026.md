---
title: "Fem nye modeller i Copilot — slik tester du dem uten å sprenge kvoten"
date: 2026-07-22
author: starefossen
category: nav
excerpt: "GPT-5.6 Luna, Sol og Terra, Kimi K2.7 Code og Gemini 3.6 Flash er nå tilgjengelig. Her er en rask oversikt over hva hver modell er god for, hva de koster — og hvordan du prøver dem ansvarlig."
url: "https://github.com/navikt/copilot/discussions"
tags:
  - models
  - cost-optimization
  - guide
---

Fem nye modeller er slått på i Copilot for Nav. Her er en ærlig oversikt over hva de koster og når du faktisk bør bruke dem — uten salgsprat fra leverandørene.

---

## Oversikt: Hva koster de, og hva er de gode for?

| Modell | Kategori | Input | Output | Styrke | Bruk til |
| --- | --- | --- | --- | --- | --- |
| **GPT-5.6 Luna** | Lightweight | $1.00 | $6.00 | Rask og billig | Enkle spørsmål, boilerplate, lette tester |
| **GPT-5.6 Terra** | Versatile | $2.50 | $15.00 | Balansert allrounder | Daglig koding, PR-beskrivelser, refaktorering |
| **GPT-5.6 Sol** | Powerful | $5.00 | $30.00 | Dypest resonnering | Store kodebaser, arkitektur, seige bugs |
| **Kimi K2.7 Code** | Versatile | $0.95 | $4.00 | Kodeforståelse og visuell koding | Tunge agent-looper og "mockup til kode" |
| **Gemini 3.6 Flash** | Versatile | $1.50 | $7.50 | Token-gjerrig agent | Raske agent-oppgaver med mange verktøy |

*Priser per 1M tokens i USD. Sammenlign med Claude Sonnet 4.6: $3.00 inn / $15.00 ut.*

> **Tips:** GPT-5.6 Sol er bare tilgjengelig for **Pro+, Max, Business og Enterprise** — ikke Pro.

---

## GPT-5.6-familien: Ikke bruk Sol til alt

OpenAI lanserer én modellfamilie delt inn i tre «gir». Tanken er at du skal bytte gir etter hvor vanskelig oppgaven er, slik at du ikke kaster bort penger på unødvendig prosessering. 

*   **Luna (1. gir):** Ekstremt rask og billig. **Men:** Den er ikke spesielt smart. Bruk den til rutinearbeid, enkel dokumentasjon og når du bare trenger rask autofullfør. Styr unna kompleks logikk.
*   **Terra (2. gir):** Standardvalget for hverdagskoding. Den klarer de fleste jobber godt nok, og erstatter typiske "velg og glem"-modeller. 
*   **Sol (3. gir):** Modellen med tyngst resonnering, men som koster mye. Fantastisk til å beholde fokus over lang tid og til å vurdere arkitektur. Den drar også stor fordel av *prompt caching*, som gjør den billigere hvis du sender inn samme, store kodebase gjentatte ganger. **Men:** Ikke bruk den til enkle oppgaver. Den er overpriset ("overkill") for hverdagskoding.

---

## Kimi K2.7 Code: Den tvungne tenkeren

Kimi K2.7 Code er den første open-weight-modellen i Copilots modellvelger. Den er spesielt trent for langvarige kodeprosjekter. 

*   **Det som fungerer:** Den er veldig god på å følge lange instruksjoner og holde kontekst over mange steg. Som open-weight-modell gir den dessuten en interessant pris/ytelse-ratio i agent-løkker der mange kall kjøres i sekvens.
*   **Haken:** Den må *alltid* «tenke». Du kan ikke skru av tenkemodusen for å få et umiddelbart, lynraskt svar. Den når heller ikke helt opp til de dyreste toppmodellene på komplekse kodingstester.

---

## Gemini 3.6 Flash: En token-gjerrig arbeidshest

Google fokuserer på effektivitet med Gemini 3.6 Flash. Dette er modellen du vil bruke til agent-arbeidsflyter (der AI-en jobber selvstendig i mange steg).

*   **Det som fungerer:** Den kaster bort færre tokens enn forgjengeren. Der andre modeller «kavner» (thrashing) med å prøve og feile mange ganger, treffer Gemini 3.6 Flash oftere på første eller andre forsøk. Google rapporterer høyere fullføringsgrad og bedre token-effektivitet enn 3.5 Flash på koding og agentoppgaver. Den er dessuten svært god på å bruke flere verktøy parallelt.
*   **Haken:** Dette er en arbeidshest, ikke et geni. Den vil slite hvis du ber den om å løse dype, arkitektoniske problemer som krever ekstrem resonnering. Den har også en tendens til å bli litt «pratsom» og overformatere svar hvis du ikke ber den være kort.

---

## Slik velger du modell i VS Code

1. Åpne Copilot Chat (`Ctrl+Alt+I` / `Cmd+Option+I`)
2. Klikk på modellnavnet øverst i chat-vinduet
3. Velg modell fra listen

Modellvelgeren er også tilgjengelig i Copilot CLI, JetBrains, Visual Studio og GitHub.com.

---

## Rask huskeregel

Ikke sikker på hvilken modell du skal velge? 

*   **Rask rutinejobb?** Luna. (Eller Kimi hvis det er greit at den må tenke litt først).
*   **Vanlig feature-utvikling?** Terra. (Eller Auto).
*   **Agent som skal rydde opp med mange verktøy?** Gemini 3.6 Flash (den roter minst).
*   **Kompleks arkitektur og vriene bugs i store repoer?** Sol. (Men bare da).

Og husk: **Auto-modus** velger en passende modell for deg med innebygd kostnadsrabatt. Men med disse retningslinjene vet du i alle fall *hvorfor* du eventuelt overstyrer den.

---

## Kontekst: Hvordan passer de inn i resten?

Disse fem modellene er tillegg til det vi allerede har — ikke erstatninger. Det er fortsatt mange gode grunner til å bruke Anthropic- og Google-modellene du kjenner fra før.

| Bruksmønster | Gode valg | Hvorfor |
| --- | --- | --- |
| Norsk tekst, microcopy, PR-beskrivelser | Claude Sonnet 4.6 / Sonnet 5 | Anthropic-modellene er best på norsk |
| Dyp risikovurdering, sikkerhetskritisk kode | Claude Opus 4.6 / 4.8 | Sterkest på resonering og nyanserte vurderinger |
| Research, store kodebaser med mye kontekst | Gemini 2.5 Pro | Beste pris/ytelse på lange kontekstvinduer |
| Sjekklister, templates, scaffold-prompts | Claude Haiku 4.5 | Rask og billig for enkle strukturerte oppgaver |
| Daglig koding, refaktorering | Terra, Sonnet 5, Auto | Alle tre er solide allroundere i samme prisklasse |
| Agentiske workflows med mange verktøykall | Gemini 3.6 Flash, Sol | Flash for parallelle verktøy, Sol for tung kontekst |
| Kostnadseffektiv agent-looping | Kimi K2.7 Code | Rimeligste alternativ i Versatile-kategorien |

De nye GPT-5.6-modellene er et godt supplement — særlig for utviklere som allerede er vant til GPT-familien eller vil ha et OpenAI-alternativ til Sonnet-klassen. Men Claude Sonnet 5 til $2/$10 (kampanjepris til 31. august) og Gemini 2.5 Pro til $1.25/$10 er fortsatt svært konkurransedyktige valg i den samme klassen.

Se [prissiden](/priser) for fullstendig sammenligning av alle modeller og priser.
