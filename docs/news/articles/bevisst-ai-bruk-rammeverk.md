---
title: "Rammeverk for bevisst AI-bruk og kompetansebevaring"
date: 2026-04-30
category: praksis
excerpt: "Grønn og rød sone, treforsøksregelen og «generer-så-forstå» — nye retningslinjer for å bruke AI-verktøy uten å miste den dype forståelsen."
tags:
  - praksis
  - kompetanse
  - ai-bruk
  - forskning
---

59 % av Navs utviklere er bekymret for at AI-verktøy svekker den dype tekniske forståelsen. Forskning viser at problemet ikke er AI i seg selv, men *hvordan* vi bruker det. Nå har vi bygget dette inn i verktøyene.

## Forskningen bak

| Studie | Funn |
| --- | --- |
| **Anthropic (2026)** | Utviklere som delegerte blindt scoret 35–39 % på forståelse. De som aktivt stilte spørsmål etter kodegenerering scoret 86 % — høyere enn de uten AI (67 %). |
| **METR (2025)** | Erfarne open source-utviklere var 19 % tregere med AI, men *trodde* de var 20 % raskere. |
| **INNOQ (2026)** | Full delegering fjerner all kognitiv belastning — også den produktive som bygger forståelse. |

Vi trenger ikke bruke AI *mindre*, men *smartere*.

## Grønn og rød sone

Vi har innført en enkel tommelfingerregel for når AI gir mest verdi:

**🟢 Grønn sone — bruk AI fritt:**
- Boilerplate og repetitiv kode
- Kjent teknologi du allerede behersker
- Konfigurasjon og infrastruktur
- Refaktorering med kjent mål

**🔴 Rød sone — kode manuelt først:**
- Debugging (sterkeste læringsmekanismen)
- Nye konsepter og ukjent teknologi
- Kjernelogikk og forretningsregler
- Sikkerhetskritisk kode

## Treforsøksregelen

Prøv å løse problemet selv i minst tre forsøk før du ber AI om hjelp. Hvert forsøk bygger forståelse som gjør at du bedre kan vurdere AI-ens forslag.

## Generer-så-forstå

Når AI genererer kode: ikke bare godta den. Still spørsmål om *hvorfor* koden er skrevet slik. Verifiser at du kan forklare hver del. Tilpass bevisst — aldri ren copy-paste.

## Hva er nytt i verktøyene?

- **Global instruksjon** som gjelder alle filer — agenter forklarer nå *hvorfor*, ikke bare *hva*
- **7 prompt-maler** har fått «Forstå koden»-seksjon med rød-sone-markør
- **Nav-pilot** varsler om kompetansebevaring i blindflekk-sjekken
- **Kodegjennomgang** sjekker at utvikleren forstår AI-genererte designvalg

## Les mer

- [Anthropic: How AI assistance impacts coding skills](https://www.anthropic.com/research/AI-assistance-coding-skills)
- [METR: AI experienced OS dev study](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/)
- [INNOQ: AI Coding Patterns Through Cognitive Load Theory](https://www.innoq.com/en/blog/2026/03/ai-cognitive-lens-cognitive-load-theory/)
