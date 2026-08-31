---
title: "Flere agenter bytter modell: dette viser målingene"
date: 2026-08-31
author: starefossen
category: nav
excerpt: "@research-agent og fire prompt-maler går over til GPT-5.6 Luna, @security-champion-agent og @nav-pilot-opus til GPT-5.6 Sol. @code-review og @accessibility-agent er satt på vent til de er målt for seg. Målingene skiller ikke modellene fra hverandre på sikkerhet og kvalitet, og da er det prisen som avgjør. nav-pilot tilbyr oppdateringen neste gang du starter den."
tags:
  - models
  - nav-pilot
  - agents
  - cost-optimization
---

Vi bytter modellene bak flere av Copilot-konfigurasjonene i nav-pilot basert på en rekke målinger (se lenger nede for detaljer). Neste gang du starter `nav-pilot` spør den om du vil synke, og sier du ja er byttet inne. Du kan også kjøre `nav-pilot sync` selv.

| Agent eller prompt | Fra | Til |
|---|---|---|
| `@research-agent` | GPT-5.3-Codex | GPT-5.6 Luna |
| Fire prompt-maler for nye tjenester | Claude Haiku 4.5 | GPT-5.6 Luna |
| `@security-champion-agent`, `@nav-pilot-opus` | Claude Opus 4.6 | GPT-5.6 Sol |
| `@code-review`, `@accessibility-agent` | GPT-5.3-Codex, Claude Sonnet 4.6 | satt på vent |

`@code-review` og `@accessibility-agent` venter fordi de kjører kommandoer, skriver filer og starter subagenter, og det har vi ikke målt noen av de nye modellene på. De to måles for seg.

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

Claude Opus 4.6 var ikke med i målingen. For `@security-champion-agent` og `@nav-pilot-opus`, som flytter fra den, hviler byttet på prisen alene.

## Blindsonen bommer alle modellene på

Alle de fire modellene overser personvernblindsonen. Tre av modellene bommer med noen få prosent, Terra 11,1 prosent, og modellen vi kjører i dag er blant dem som bommer. Det er ikke et modellproblem, og ikke noe modellbytte fikser for oss dessverre. Vi følger det som en bug i instruksjonene til agenten selv.

## Prisen

Pris (USD) per million tokens, slik GitHub publiserte dem 30. august 2026:

| Modell | Input | Output |
|---|---|---|
| GPT-5.6 Luna | 0,20 | 1,20 |
| Claude Haiku 4.5 | 1 | 5 |
| GPT-5.6 Sol (kampanjepris ut 3. september 2026) | 2 | 10 |
| Claude Sonnet 4.6 | 3 | 15 |
| Claude Opus 4.6 | 5 | 25 |

Luna koster under en tidel av Sonnet 4.6. Sol-prisen er en kampanjepris; fra 4. september er den 4 og 20 dollar, og da ligger Sol rundt 20 prosent under Opus 4.6, som er modellen de to tyngste agentene flytter fra. Den gevinsten gjelder kontekster under 272K tokens. Over 272K koster Sol 8 og 30 dollar og er dyrere enn Opus 4.6.

Tallene har en dato av en grunn. Sol lå på 5 og 30 dollar da vi sist synkroniserte prisene 10. august, og falt til 2 og 10 i løpet av måneden. Prislista i `apps/my-copilot/src/lib/model-pricing.ts` er den vi regner ut fra.

## Hvis en agent svarer dårligere

Si fra i [#github-copilot](https://nav-it.slack.com/archives/C055TNXBM17). Modellvalget står i metadaten til hver agent, så det er én linje å sette tilbake hvis det skulle være behov.

*Rettet 31. august 2026: Sol-prisen og hva målingen faktisk dekker.*
