---
title: "Flere agenter bytter modell: dette viser målingene"
date: 2026-08-31
author: starefossen
category: nav
excerpt: "@research-agent og fire prompt-maler går over til GPT-5.6 Luna, @security-champion-agent og @nav-pilot-opus til GPT-5.6 Sol. @code-review og @accessibility-agent er satt på vent til de er målt for seg. Målingene skiller ikke modellene fra hverandre på sikkerhet og kvalitet, og da er det prisen som avgjør. Alt oppdateres automatisk."
tags:
  - models
  - nav-pilot
  - agents
  - cost-optimization
---

Vi bytter modellene bak flere av Copilot-konfigurasjonene i nav-pilot basert på en rekke målinger (se lenger nede for detaljer). Oppdateringen skjer automatisk neste gang du starter `nav-pilot`, eller når du kjører `nav-pilot sync` manuelt.

| Agent eller prompt | Fra | Til |
|---|---|---|
| `@research-agent` | GPT-5.3-Codex | GPT-5.6 Luna |
| Fire prompt-maler for nye tjenester | Claude Haiku 4.5 | GPT-5.6 Luna |
| `@security-champion-agent`, `@nav-pilot-opus` | Claude Opus 4.6 | GPT-5.6 Sol |
| `@code-review`, `@accessibility-agent` | GPT-5.3-Codex, Claude Sonnet 4.6 | satt på vent, se rettingen under |

`@forfatter` blir stående på Claude Sonnet 4.6. Jobben omfatter norsk tekst, og vi har ingen måling som sier at en annen modell gjør den like godt.

## Hva vi målte

Vi kjørte nav-pilot-konfigurasjonen mot fire modeller med samme golden prompt: en tjeneste som henter fødselsnummer fra ID-porten, der agenten skal si fra om en personvernblindsone. Rundt 195 kjøringer med ekte modeller.

| Modell | Bom | Andel |
|---|---|---|
| Claude Sonnet 4.6 (dagens) | 2 av 50 | 4,0 % |
| GPT-5.6 Sol | 1 av 50 | 2,0 % |
| GPT-5.6 Luna | 1 av 50 | 2,0 % |
| GPT-5.6 Terra | 5 av 45 | 11,1 % |

## Ingen av modellene kom bedre ut

Fishers eksakte test mot Claude gir p = 1,00 for Sol, p = 1,00 for Luna og p = 0,25 for Terra. Alle konfidensintervallene overlapper.

Målingen sier altså ikke at de nye modellene er tryggere. Den sier at vi ikke klarer å skille dem fra den vi kjører i dag. Det er nettopp derfor prisen får avgjøre.

## Blindsonen bommer alle modellene på

Alle de fire modellene overser personvernblindsonen. Tre av modellene bommer med noen få prosent, Terra 11,1 prosent, og modellen vi kjører i dag er blant dem som bommer. Det er ikke et modellproblem, og ikke noe modellbytte fikser for oss dessverre. Vi følger det som en bug i instruksjonene til agenten selv.

## Prisen

Pris (USD) per million tokens, slik GitHub publiserte dem 30. august 2026:

| Modell | Input | Output |
|---|---|---|
| GPT-5.6 Luna | 0,20 | 1,20 |
| Claude Haiku 4.5 | 1 | 5 |
| GPT-5.6 Sol (kampanjepris, se rettingen under) | 2 | 10 |
| Claude Sonnet 4.6 | 3 | 15 |
| Claude Opus 4.6 | 5 | 25 |

Luna koster under en tidel av Sonnet 4.6. Sol koster en tredjedel mindre enn Sonnet 4.6 og 60 % mindre enn Opus 4.6, som er modellen de to tyngste agentene flytter fra.

> **Retting 31. august 2026.** To ting i artikkelen stemmer ikke, og begge ble
> oppdaget samme dag.
>
> **1. To av de ni pinningene er satt på vent.** `@code-review` og
> `@accessibility-agent` bytter ikke modell nå. Grunnen er verktøytilgangen
> deres: `@code-review` har `execute`, og `@accessibility-agent` har `execute`,
> `edit` og `runSubagent`, altså kjøre kommandoer, skrive filer og starte
> subagenter. Målingen som bærer Luna-valget kjørte bare nav-pilot-personaen,
> som ikke bruker verktøy i det hele tatt, så vi har ingen tall for en
> verktøytung agent på Luna. Det er ikke det samme som at Luna er utrygg der.
> Det er at vi ikke har målt det, og de to måles for seg før noen bestemmer noe.
> Harnesset må utvides først. De fem andre Luna-pinningene står:
> `@research-agent`, som bare leser og søker, og de fire prompt-malene.
>
> **2. Sol-prisen er en kampanjepris.** Prisen på 2 og 10 dollar er 50 prosent
> av standardprisen, og den varer ut 3. september 2026. Det står i en fotnote i
> [GitHubs pristabell](https://docs.github.com/en/copilot/reference/copilot-billing/models-and-pricing#user-content-fn-gpt-56-sol-promo)
> som vi ikke fanget opp da vi skrev dette. Fotnoten oppgir ikke standardprisen,
> men doblet kampanjepris gir 4 og 20 dollar fra 4. september. Det tallet er
> regnet ut fra «50 % off», ikke lest av en publisert prisliste. Blander vi
> input og output 10:1, som er et anslag og ikke noe vi har målt, koster Sol da
> 5,45 dollar per million tokens mot 2,73 i dag. Sol går dermed fra en tredjedel
> billigere enn Sonnet 4.6 (4,09) til en tredjedel dyrere. Mot Opus 4.6 (6,82),
> som er modellen de to agentene faktisk kjører i dag, går den fra 60 prosent
> billigere til rundt 20 prosent billigere. Beslutningen står, det er
> regnestykket som endrer seg. Luna er ikke berørt: OpenAIs egen modellside
> oppgir 0,20 og 1,20 dollar som listepris, og GitHub har ingen fotnote på
> Luna-raden. Gemini Flash-radene er også kampanjepris, ut 31. desember 2026.

Tallene har en dato av en grunn. Sol lå på 5 og 30 dollar da vi sist synkroniserte prisene 10. august, og falt til 2 og 10 i løpet av måneden. Prislista i `apps/my-copilot/src/lib/model-pricing.ts` er den vi regner ut fra.

## Hvis en agent svarer dårligere

Si fra i [#github-copilot](https://nav-it.slack.com/archives/C055TNXBM17). Modellvalget står i metadaten til hver agent, så det er én linje å sette tilbake hvis det skulle være behov.
