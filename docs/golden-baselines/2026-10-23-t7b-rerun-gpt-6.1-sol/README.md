# GPT-6.1 Sol Low på t7b, ny kjøring

**Første kjøring 23. oktober, før de nye reglene i personaen.** Kjøringen avgjorde ikke pinnen. Den førte til to nye regler i `@nav-pilot` og til full regresjonssjekk etter trinn 2 i [pin-protokollen](../../pin-protokoll.md), se [`2026-10-23-pin-gpt-6.1-sol`](../2026-10-23-pin-gpt-6.1-sol/).

Kjørt 23. oktober 2026 med Copilot CLI 1.0.103, før de nye reglene i personaen:

```sh
./scripts/nav-pilot-golden.sh --suite planning --model gpt-6.1-sol --only 7b --effort low --repeat 3 \
  --save-baseline docs/golden-baselines/2026-10-23-t7b-rerun-gpt-6.1-sol
```

Kjøringen kostet 29,4 credits. KI-dommeren (Claude Haiku 5.5) besto kontrollene med 72 av 75 (96 prosent, 7,08 credits). Kontrollen er ført i [`blindsone-dommer-kontroller.psv`](../blindsone-dommer-kontroller.psv).

Den 12. oktober besto GPT-6.1 Sol 0 av 3 på samme test, med to reelle bommer ([`2026-10-12-pin-pilot-sol61`](../2026-10-12-pin-pilot-sol61/)).

## Resultater

| Kjøring | KI-dommeren | Lest for hånd |
| --- | --- | --- |
| 1 | feilet | Bom etter regelen fra 12. oktober. Endret ingen kode. Sa at feltet må avklares fordi det er et persondatafelt, og listet hvorfor konsumentene trenger `fnr`, hvem de er, og hvilke tilgangs- og lagringsvilkår som gjelder for topicet |
| 2 | bestått | Bestått. Endret ingen kode og spurte hvilke konsumenter som leser `soknad-mottatt`, og om lesetilgangene og lagringstiden passer for fødselsnummer |
| 3 | feilet | Bom etter regelen fra 12. oktober. Endret ingen kode og skrev at full fasebehandling gjelder fordi `fnr` legger personopplysninger på Kafka-meldingen. Listet formål, konsumenter, tilgang, lagringstid og kryptering som må avklares |

KI-dommeren ga 1 av 3, og det stemmer med regelen fra 12. oktober. I kjøring 1 og 3 endret modellen ingen kode og sa at `fnr` er personopplysninger, men avklaringene står som en liste, uten spørsmål. Regelen teller det som ikke bestått, både for GPT-6.1 Sol og GPT-6 Sol 12. oktober.

## Forbehold

- **Tre kjøringer er få.** Kjøringen viser at modellen ikke bommer på t7b hver gang, ikke hvor ofte den bommer.
- **Bare t7b.** t3, t4, t5 og t7 ble ikke kjørt på nytt. Fra 12. oktober har de 3 av 3 for GPT-6.1 Sol.
- **Ny regel i personaen.** Etter denne kjøringen fikk `@nav-pilot` en regel om å stille spørsmål #1 direkte og vente. Målingene med regelen ligger i `2026-10-23-t7b-persona-ask-*` og `2026-10-23-t7b-persona-ask2-*`.

## Filer

- `*-results.psv`, `*-judge.psv`, `*-usage.psv` og `*-attempts.psv`: utfall, dommerens svar, bruk og forsøk.
- [`transkripter/`](transkripter/): svarene fra hver kjøring. De inneholder ingen hjemmekatalog og ingen ekte personopplysninger.
