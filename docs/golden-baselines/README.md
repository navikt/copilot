# Modellbenchmark

Her ligger målingene bak [ki-utvikling.nav.no/modeller](https://ki-utvikling.nav.no/modeller). Hver kjøring er rådata fra testoppsettet i `scripts/nav-pilot-golden.sh`, og `summary.json` er sammendraget siden leser. CI feiler hvis `summary.json` ikke stemmer med filene.

## Testpakkene

| Testpakke | Agent | Hva vi sjekker |
| --- | --- | --- |
| `planning` | `@nav-pilot` | Stopper etter fase 1, tar opp personvern og tilgang, erklærer rød sone, velger TokenX (test 2–5, samme protokoll som 23. september) |
| `review` | `@code-review` | Finner de plantede feilene i en Kotlin- og en TSX-fil, og oppgir riktig linje |
| `norsk` | `@forfatter` | Skriver om et utkast og skriver en notis: ingen nynorsk, ingen KI-floskler, «KI» og ikke «AI», 30–90 ord |
| `coding` | `@nav-pilot` | Får feilende Go- og TS-tester grønne, og endrer bare filene med feilen, også når rettingen går over to filer |
| `research` | `@research` | Oppgir riktig fil og linje, sier ærlig at noe ikke finnes, og oppsummerer i høyst tre punkter |

Alle sjekkene er deterministiske, og hver sjekk har en kontroll som viser at den kan feile. Fiksturene feiler sjekkene før modellen har gjort noe, og den kjente rettingen får testene grønne. Testoppsettet nekter å kjøre hvis en av delene ikke stemmer. I tillegg spiller `scripts/nav-pilot-golden.bats` en agent som gjør feil, for hver testpakke.

Agenten installeres i arbeidsområdet under navnet `golden-<agent>`. En agent med samme navn i `~/.copilot/agents/` ville ellers overstyre filen vi tester. Med `--model` fjernes også modellpinnen i agentfila, fordi pinnen ellers går foran `--model`. Sammendraget avviser en kjøring der modellen i bruksradene ikke er modellen i headeren.

## Før du kjører

Testoppsettet kjører Copilot CLI som deg, utenfor cplt, med `--allow-all-tools`. Agenten jobber i en midlertidig mappe, men `coding`-pakken kjører skallkommandoer uten tilsyn, og ingenting hindrer en kommando i å gå utenfor mappa. Kjør på en maskin der det er akseptabelt.

## Kjør en benchmark

Skriv en matrisefil med én arm per linje, `testpakke modell innsatsnivå n`:

```
planning  gpt-6-sol        high     5
review    claude-opus-5.5  medium  10
coding    gpt-6-luna       default  5
```

Innsatsnivået (effort) er `low`, `medium`, `high` eller `default`. `default` betyr at testoppsettet ikke sender `--effort`. Bruk det for modeller som ikke tar imot et innsatsnivå. Samme testpakke, modell og innsatsnivå kan bare stå én gang i en matrise.

```sh
scripts/benchmark-matrix.py docs/golden-baselines/2026-10-01-gpt-6.1.matrix --dry-run
scripts/benchmark-matrix.py docs/golden-baselines/2026-10-01-gpt-6.1.matrix --jobs 3
mise run benchmark:summary
```

`--dry-run` viser armene og anslår kostnaden ut fra kjøringer som alt ligger i `summary.json`. Resultatene havner i en mappe med samme navn som matrisefilen. Stopper kjøringen, starter du samme kommando på nytt: ferdige armer hoppes over, og en arm som ble avbrutt, kjøres fra start. Commit matrisefilen, mappen og `summary.json` sammen.

## summary.json

Hver kjøring i `runs` har disse feltene:

| Felt | Innhold |
| --- | --- |
| `date`, `suite`, `model`, `cli_version`, `n`, `source` | Når, hvilken testpakke, modell-ID, CLI-versjon, antall kjøringer og rådatafila (sti fra repo-roten) |
| `effort` | Innsatsnivået vi ba om, `default` hvis vi ikke ba om noe |
| `ran_at` | Innsatsnivået bruksradene viser at modellen faktisk kjørte med, `default` hvis radene ikke har noe |
| `checks[].passed` | Hvor mange av de `n` kjøringene som besto sjekken |
| `credits` | Median og snitt per kjøring, eksakt fra `assistant_usage_events`, inkludert nye forsøk og subagenter. `null` når bruken ikke ble registrert for alle kall, og da er `usage_complete` `false`. Siden viser `null` som «–» |
| `wall_seconds` | Median veggklokketid per kjøring |
| `smoke` | `true` for røyktester av oppsettet (filer med `smoke` i stien). Det er ikke resultater, og siden merker dem «(røyktest)» |

Modellnavnet slår siden opp selv. Bruksradene krever at `~/.copilot/session-store.db` er lesbar.

## Legg til en modell

Legg til en linje i en matrisefil. Modell-ID-en må være en Copilot CLI godtar for `--model`.

## Legg til en sjekk

Skriv sjekken i testpakkens `run_pass_*` i `scripts/nav-pilot-golden.sh`, vis at den feiler på en kontroll i `scripts/nav-pilot-golden.bats`, og gi den en norsk beskrivelse i `CHECKS` i `scripts/benchmark-summary.py`. Sammendraget stopper på en sjekk uten beskrivelse.
