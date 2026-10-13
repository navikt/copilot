# Test 4 i opencode, 12. oktober 2026

Første kjøring av test 4 med `--client opencode` ([#1451](https://github.com/navikt/copilot/issues/1451)). Før 12. oktober kjørte harnessen bare enkeltprompter i opencode. Test 4 ble ført som feilet uten at modellen ble kalt, se for eksempel [planleggingskjøringen 7. oktober](../2026-10-07-personvern-pr1442-v6-planning-opencode-gpt-6-sol-results.psv).

## Oppsett

- opencode 1.19.0, GPT-6 Sol med innsatsnivå `low`
- én kjøring av t4a (intervjuet) og t4b (svarene og planen)

```sh
./scripts/nav-pilot-golden.sh --client opencode --model gpt-6-sol --effort low \
  --only 4 --repeat 1 --save-baseline docs/golden-baselines/2026-10-12-opencode-t4/baseline.txt
```

opencode lager sine egne sesjons-ID-er. Harnessen gir derfor første tur tittelen til sin egen sesjons-ID, slår opp opencode-ID-en på den tittelen og sender andre tur med `--session`. Begge turene havnet i samme opencode-sesjon, med seks svar fra modellen til sammen.

## Resultat

Test 4 besto: t4b inneholder en Fase 2-plan med erklæringen 🔴 Rød sone. t4a stilte spørsmål og stoppet før planen. Svarene ligger i [transkripter](transkripter/).

## Forbruk

opencode har ingen `assistant_usage_events`, så forbruket er et anslag og ikke målt. Tokentallene fra `opencode export` er regnet om med satsene fra Copilot CLI-radene for GPT-6 Sol i [2026-10-01-batch3](../2026-10-01-batch3/). Anslaget er om lag 12 credits for begge turene, pluss en liten sjekkprompt før kjøringen. Taket var 30 credits.

Én kjøring sier at oppsettet virker. Den sier ikke hvor ofte GPT-6 Sol består test 4 i opencode.
