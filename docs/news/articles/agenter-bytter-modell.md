---
title: "Flere agenter bytter modell: dette viser målingene"
date: 2026-08-31
author: starefossen
category: nav
excerpt: "@research-agent, @code-review og @accessibility-agent går over til GPT-5.6 Luna, @security-champion-agent og @nav-pilot-opus til GPT-5.6 Sol. Målingene skiller ikke modellene fra hverandre på sikkerhet og kvalitet, og da er det prisen som avgjør. Alt oppdateres automatisk"
tags:
  - models
  - nav-pilot
  - agents
  - cost-optimization
---

Vi bytter modellene bak flere av Copilot konfigurasjonene i nav-pilot basert på en rekke målinger (se lenger nede for detaljer). Oppdateringen skjer automatisk neste gang du starter `nav-pilot` eller kjøre `nav-pilot sync` manuelt.

| Agent eller prompt | Fra | Til |
|---|---|---|
| `@research-agent`, `@code-review` | GPT-5.3-Codex | GPT-5.6 Luna |
| `@accessibility-agent` | Claude Sonnet 4.6 | GPT-5.6 Luna |
| Fire prompt-maler for nye tjenester | Claude Haiku 4.5 | GPT-5.6 Luna |
| `@security-champion-agent`, `@nav-pilot-opus` | Claude Opus 4.6 | GPT-5.6 Sol |

`@forfatter` blir stående på Claude Sonnet 4.6. Jobben omfatter norsk tekst, og vi har ingen måling som sier at en annen modell gjør den like godt.

## Hva vi målte

Vi kjørte nav-pilot konfigurasjonen mot fire modeller med samme golden prompt: en tjeneste som henter fødselsnummer fra ID-porten, der agenten skal si fra om en personvernblindsone. Rundt 195 kjøringer med ekte modeller.

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
| GPT-5.6 Sol | 2 | 10 |
| Claude Sonnet 4.6 | 3 | 15 |
| Claude Opus 4.6 | 5 | 25 |

Luna koster under en tidel av Sonnet 4.6. Sol koster en tredjedel mindre enn Sonnet 4.6 og 60 % mindre enn Opus 4.6, som er modellen de to tyngste agentene flytter fra.

Tallene har en dato av en grunn. Sol lå på 5 og 30 dollar da vi sist synkroniserte prisene 10. august, og falt til 2 og 10 i løpet av måneden. Prislista i `apps/my-copilot/src/lib/model-pricing.ts` er den vi regner ut fra.

## Hvis en agent svarer dårligere

Si fra i [#github-copilot]([https://github.com/navikt/copilot/discussions](https://nav-it.slack.com/archives/C055TNXBM17)). Modellvalget står i metadaten til hver agent, så det er én linje å sette tilbake hvis det skulle være behov.
