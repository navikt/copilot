---
title: "Flere agenter bytter modell: dette viser målingene"
date: 2026-08-31
author: starefossen
category: nav
excerpt: "@research-agent, @code-review og @accessibility-agent går over til GPT-5.6 Luna, @security-champion-agent og @nav-pilot-opus til GPT-5.6 Sol. Målingene skiller ikke modellene fra hverandre på sikkerhet, og da er det prisen som avgjør. Når byttet er merget, henter du det med nav-pilot sync."
tags:
  - models
  - nav-pilot
  - agents
  - cost-optimization
---

Vi bytter modell bak flere av agentene og fire prompt-maler. Byttet er én linje i
frontmatteren til hver agent, og når den linjen er merget, henter du den med
`nav-pilot sync`. Du trenger ikke installere noe.

| Agent eller prompt | Fra | Til |
|---|---|---|
| `@research-agent`, `@code-review` | GPT-5.3-Codex | GPT-5.6 Luna |
| `@accessibility-agent` | Claude Sonnet 4.6 | GPT-5.6 Luna |
| Fire prompt-maler for nye tjenester | Claude Haiku 4.5 | GPT-5.6 Luna |
| `@security-champion-agent`, `@nav-pilot-opus` | Claude Opus 4.6 | GPT-5.6 Sol |

Prompt-malene er `ktor-endpoint`, `spring-boot-endpoint`, `golang-service` og `nextjs-api-route`.

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

Fishers eksakte test mot Claude gir p = 1,00 for Sol, p = 1,00 for Luna og p = 0,25 for Terra. Alle konfidensintervallene overlapper.

Målingen sier altså ikke at de nye modellene er tryggere. Den sier at vi ikke klarer å skille dem fra den vi kjører i dag. Det er nettopp derfor prisen får avgjøre.

## Blindsonen bommer alle modellene på

Alle fire modellene overser personvernblindsonen. Tre av dem bommer noen få prosent av gangene, Terra 11,1 prosent, og modellen vi kjører i dag er blant dem som bommer. Det er ikke et modellproblem, og ikke noe modellbytte fikser det. Vi følger det som en bug i instruksjonene til agenten selv.

## Prisen

Dollar per million tokens, slik GitHub publiserte dem 30. august 2026:

| Modell | Input | Output |
|---|---|---|
| GPT-5.6 Luna | 0,20 | 1,20 |
| Claude Haiku 4.5 | 1 | 5 |
| GPT-5.6 Sol | 2 | 10 |
| Claude Sonnet 4.6 | 3 | 15 |
| Claude Opus 4.6 | 5 | 25 |

Luna koster under en tidel av Sonnet 4.6. Sol koster en tredjedel mindre enn Sonnet 4.6 og 60 % mindre enn Opus 4.6, som er modellen de to tyngste agentene flytter fra.

Tallene har en dato av en grunn. Sol lå på 5 og 30 dollar da vi sist synkroniserte prisene 10. august, og falt til 2 og 10 i løpet av måneden. Prislista i `apps/my-copilot/src/lib/model-pricing.ts` er den vi regner ut fra.

## Hvis en agent svarer dårligere

Si fra i [discussions](https://github.com/navikt/copilot/discussions). Modellvalget står i frontmatteren til hver agent, så det er én linje å sette tilbake.
