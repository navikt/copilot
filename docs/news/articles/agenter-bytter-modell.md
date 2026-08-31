---
title: "Flere agenter bytter modell: dette viser målingene"
date: 2026-08-31
author: starefossen
category: nav
excerpt: "@research, @code-review og @accessibility går over til GPT-5.6 Luna, @security-champion og @nav-pilot-opus til GPT-5.6 Sol. Målingene skiller ikke modellene fra hverandre på sikkerhet, og da er det prisen som avgjør. Du får endringen med nav-pilot sync."
tags:
  - models
  - nav-pilot
  - agents
  - cost-optimization
---

Vi bytter modell bak flere av agentene og fire av prompt-malene som lager stillas for ny kode. Kjør `nav-pilot sync`, så er du oppdatert. Du trenger ikke installere noe.

| Agent | Fra | Til |
|---|---|---|
| `@research`, `@code-review` | GPT-5.3-Codex | GPT-5.6 Luna |
| `@accessibility` | Claude Sonnet 4.6 | GPT-5.6 Luna |
| `@security-champion`, `@nav-pilot-opus` | Claude Opus 4.6 | GPT-5.6 Sol |

`@forfatter` blir stående på Claude Sonnet 4.6. Jobben er norsk tekst, og vi har ingen måling som sier at en annen modell gjør den like godt.

## Hva vi målte

Vi kjørte nav-pilot-personaen mot fire modeller med samme golden prompt: en tjeneste som henter fødselsnummer fra ID-porten, der agenten skal si fra om en personvernblindsone. Rundt 195 kjøringer mot ekte modeller.

| Modell | Bom | Andel |
|---|---|---|
| Claude Sonnet 4.6 (dagens) | 2 av 50 | 4,0 % |
| GPT-5.6 Sol | 1 av 50 | 2,0 % |
| GPT-5.6 Luna | 1 av 50 | 2,0 % |
| GPT-5.6 Terra | 5 av 45 | 11,1 % |

## Ingen av modellene kom bedre ut

Fisher eksakt test mot Claude gir p = 1,00 for Sol, p = 1,00 for Luna og p = 0,25 for Terra. Alle konfidensintervallene overlapper.

Målingen sier altså ikke at de nye modellene er tryggere. Den sier at vi ikke klarer å skille dem fra den vi kjører i dag. Det er nettopp derfor prisen får avgjøre.

## Blindsonen bommer alle modellene på

Alle fire modellene overser personvernblindsonen noen få prosent av gangene, også den vi kjører i dag. Det er ikke et modellproblem, og ingen modellbytte fikser det. Vi følger det som en bug i instruksjonene til agenten selv.

## Prisen

Per million tokens i dollar, fra `apps/my-copilot/src/lib/model-pricing.ts`:

| Modell | Input | Output |
|---|---|---|
| GPT-5.6 Luna | 0,20 | 1,20 |
| Claude Sonnet 4.6 | 3 | 15 |
| Claude Opus 4.6 | 5 | 25 |
| GPT-5.6 Sol | 5 | 30 |

Luna koster rundt en tidel av Sonnet 4.6, og det er der gevinsten ligger. Sol er ikke billigere enn Opus 4.6 den erstatter: samme pris inn, og 30 mot 25 dollar ut. Det byttet sparer ingenting på tokenprisen.

## Hvis en agent svarer dårligere

Si fra i [discussions](https://github.com/navikt/copilot/discussions). Modellvalget står i frontmatteren til hver agent, så det er én linje å sette tilbake.
