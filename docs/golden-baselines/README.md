# Modellbenchmark

Her ligger målingene bak modellsiden på ki-utvikling.nav.no. Hver kjøring er rådata fra testoppsettet i `scripts/nav-pilot-golden.sh`, og `summary.json` er sammendraget siden leser. CI feiler hvis `summary.json` ikke stemmer med filene.

[`docs/pin-protokoll.md`](../pin-protokoll.md) skiller disse persona-sjekkene fra den utforskende piloten for kodeoppgaver. Ingen av dem gir alene grunnlag for å endre modellpinne.
De fire oppgavene og kontrollene for den avsluttede kodepiloten ligger i [`benchmark/realistic/`](../../benchmark/realistic/README.md). Den lokale kontrollen kjører uten modellkall. Resultatene ligger i [`benchmark/realistic/runs/`](../../benchmark/realistic/runs/) og inngår ikke i `summary.json` eller `/modeller`.

## Testpakkene

| Testpakke | Agent | Hva vi sjekker |
| --- | --- | --- |
| `planning` | `@nav-pilot` | Stopper etter fase 1, tar opp personvern og tilgang, erklærer rød sone, velger TokenX (test 2–5, samme protokoll som 23. september) |
| `review` | `@code-review` | Finner de plantede feilene i en Kotlin- og en TSX-fil, og oppgir riktig linje |
| `norsk` | `@forfatter` | Skriver om et utkast og skriver en notis: ingen nynorsk, ingen KI-floskler, ikke «AI», 30–90 ord |
| `coding` | `@nav-pilot` | Får feilende Go- og TS-tester grønne, og endrer bare filene med feilen, også når rettingen går over to filer |
| `research` | `@research` | Oppgir riktig fil og linje, sier ærlig at noe ikke finnes, og oppsummerer i høyst tre punkter |

Alle sjekkene er deterministiske, og hver sjekk har en kontroll som viser at den kan feile. For hver testpakke spiller `scripts/nav-pilot-golden.bats` en agent som gjør feil, og sjekken må slå ut. For `norsk` og `coding` sjekker testoppsettet i tillegg fiksturene før hver kjøring: utkastet og de feilende testene må feile sjekkene, og den kjente rettingen må få testene grønne. Ellers kjører det ikke.

Agenten installeres i arbeidsområdet under navnet `golden-<agent>`. En agent med samme navn i `~/.copilot/agents/` ville ellers overstyre filen vi tester. Med `--model` fjernes også modellpinnen i agentfila, fordi pinnen ellers går foran `--model`. Sammendraget avviser en kjøring der hovedagenten har kjørt på en annen modell enn den som står øverst i kjøringsfila. Subagenter kan bruke andre modeller; de listes i `subagent_models`.

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

`--dry-run` viser armene og anslår kostnaden ut fra kjøringer som alt ligger i `summary.json`. Resultatene havner i en mappe med samme navn som matrisefilen. Stopper kjøringen, starter du samme kommando på nytt: ferdige armer hoppes over, og en arm som ble avbrutt, kjøres fra start. Legg matrisefilen, mappen og `summary.json` i samme commit.

## summary.json

Hver kjøring i `runs` har disse feltene:

| Felt | Innhold |
| --- | --- |
| `date`, `suite`, `model`, `cli_version`, `n`, `source` | Når, hvilken testpakke, modell-ID, CLI-versjon, antall kjøringer og rådatafila (sti fra repo-roten) |
| `model_verified` | `true` når bruksradene bekrefter modellen. `false` når kjøringen ikke har bruksrader, og da er modellen bare det testoppsettet ba om |
| `subagent_models` | Andre modeller subagentene brukte. Kredittene deres er med i `credits` |
| `effort` | Innsatsnivået vi ba om, `default` hvis vi ikke ba om noe |
| `ran_at` | Innsatsnivået bruksradene viser at modellen faktisk kjørte med, `default` hvis radene ikke har noe |
| `checks[].passed` | Hvor mange av de `n` kjøringene som besto sjekken |
| `credits` | Median og snitt per kjøring, eksakt fra `assistant_usage_events`, inkludert nye forsøk og subagenter. `null` når bruken ikke ble registrert for alle kall, og da er `usage_complete` `false`. Siden viser `null` som «–» |
| `wall_seconds` | Median veggklokketid per kjøring |
| `smoke` | `true` for benchmarker med én kjøring som sjekker oppsettet (filer med `smoke` i stien). Det er ikke resultater, og siden merker dem «(benchmark)» |

Modellnavnet slår siden opp selv. Bruksradene krever at `~/.copilot/session-store.db` er lesbar.

## Legg til en modell

Legg til en linje i en matrisefil. Modell-ID-en må være en Copilot CLI godtar for `--model`.

## Legg til en sjekk

Skriv sjekken i testpakkens `run_pass_*` i `scripts/nav-pilot-golden.sh`, vis at den feiler på en kontroll i `scripts/nav-pilot-golden.bats`, og gi den en norsk beskrivelse i `CHECKS` i `scripts/benchmark-summary.py`. Sammendraget stopper på en sjekk uten beskrivelse.
