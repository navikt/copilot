# Modellbenchmark

Her ligger målingene bak [ki-utvikling.nav.no/modeller](https://ki-utvikling.nav.no/modeller). Hver kjøring er rådata fra `scripts/nav-pilot-golden.sh`, og `summary.json` er sammendraget siden leser. CI feiler hvis `summary.json` ikke stemmer med filene.

## Suitene

| Suite | Agent | Hva vi sjekker |
| --- | --- | --- |
| `planning` | `@nav-pilot` | Stopper etter fase 1, tar opp personvern og tilgang, erklærer rød sone, velger TokenX (test 2–5, samme protokoll som 23. september) |
| `review` | `@code-review` | Finner de plantede feilene i en Kotlin- og en TSX-fil, og oppgir riktig linje |
| `norsk` | `@forfatter` | Skriver om et utkast og skriver en notis: ingen nynorsk, ingen KI-floskler, «KI» og ikke «AI», 30–90 ord |
| `coding` | `@nav-pilot` | Får feilende Go- og TS-tester grønne, og endrer bare filene med feilen |

Alle sjekkene er deterministiske. Hver sjekk har en kontroll som viser at den kan feile: fiksturene feiler sjekkene før modellen har gjort noe (harnesset nekter å kjøre ellers), og `scripts/nav-pilot-golden.bats` spiller en agent som gjør feil for hver suite.

## Kjør en benchmark

Skriv en matrisefil med én arm per linje, `suite modell effort n`:

```
planning  gpt-6-sol        high     5
review    claude-opus-5.5  medium  10
coding    gpt-6-luna       default  5
```

`default` betyr at harnesset ikke sender `--effort`. Bruk det for modeller som ikke tar imot effort.

```sh
scripts/benchmark-matrix.py docs/golden-baselines/2026-10-01-gpt-6.1.matrix --dry-run
scripts/benchmark-matrix.py docs/golden-baselines/2026-10-01-gpt-6.1.matrix --jobs 3
mise run benchmark:summary
```

`--dry-run` viser armene og anslår kostnaden ut fra kjøringer som alt ligger i `summary.json`. Resultatene havner i en mappe med samme navn som matrisefilen. Stopper kjøringen, starter du samme kommando på nytt: ferdige armer hoppes over, og en arm som ble avbrutt kjøres fra start. Commit matrisefilen, mappen og `summary.json` sammen.

Kreditter er eksakte tall fra `assistant_usage_events`, inkludert nye forsøk og subagenter. Tallene krever at `~/.copilot/session-store.db` er lesbar. Effort i sammendraget er det modellen faktisk kjørte med ifølge de samme radene.

## Legg til en modell

Modellen må finnes i `cli/nav-pilot/internal/domain/known_models_gen.go` (`mise run models:sync`), ellers viser siden modell-ID-en i stedet for navnet. Deretter trenger du bare en ny linje i en matrisefil.

## Legg til en sjekk

Skriv sjekken i suitens `run_pass_*` i `scripts/nav-pilot-golden.sh`, vis at den feiler på en kontroll i `scripts/nav-pilot-golden.bats`, og gi den en norsk beskrivelse i `CHECKS` i `scripts/benchmark-summary.py`. Sammendraget stopper på en sjekk uten beskrivelse.

Filer med `smoke` i stien er røyktester av oppsettet, ikke resultater. Siden merker dem «(røyktest)».
