# Pilot: K1 og T2, 12. oktober 2026

Første kjøring av to nye oppgaver fra designforslaget i
[#1409](https://github.com/navikt/copilot/issues/1409): K1 (token-cache for
TokenX via Texas, Kotlin/Ktor) og T2 (tilgangskontroll med oasis, Next.js).
Fiksturene ligger i `benchmark/realistic/kotlin/token-cache` og
`benchmark/realistic/typescript/tilgang`. Dette er en pilot. Tallene er ikke
lagt inn i `summary.json` og skal ikke brukes til å velge modell.

## Oppsett

- Copilot CLI 1.0.95 og cplt 2026.10.08-190103-d4ae77a, på samme måte som
  `benchmark/realistic/runs/2026-10-02-direct-worker/`. Standardagenten
  (`prepare.py --direct-worker`), Medium innsats, ny økt per forsøk,
  `--max-ai-credits 30`, `--no-ask-user` og `--disable-builtin-mcps`. Skriptene
  ligger i `run-attempt.sh` og `batch.sh`.
- cplt nektet tilgang til evaluatormappa, manifestet og hele repoet, så
  agenten ikke kunne lese de skjulte testene.
- Modellene var GPT-6.1 Sol og GPT-6 Luna, med n=3 per oppgave.
- Agenten fikk bare symptomet i prompten. Kravene står i koden: ADR-en om
  token-cachen og `docs/drift.md` om NAIS-probene.

## Kontroller før første modellkall

Hver oppgave ble kjørt 10 ganger med `check.py`: starttilstand, løsning og alle
`invalid`-kontrollene. Alle kjøringene ga samme resultat, uten ustabile
utfall. Loggene ligger i `kontroller-*.log`.

| Oppgave | Kontroll | Feiler på |
| --- | --- | --- |
| K1 | `invalid-1`: retter bare utløpsmarginen | to brukere får samme token |
| K1 | `invalid-2`: slår av cachen | cache-treff kaller Texas på nytt |
| K1 | `invalid-3`: retter bare cache-nøkkelen | token som snart utløper |
| T2 | `invalid-1`: legger bare ruten til i `matcher` | forfalsket token godtas |
| T2 | `invalid-2`: beskytter hele `/api` i proxy | `/api/isalive` gir 401 |
| T2 | `invalid-3`: retter bare ruten fra prompten | `/api/intern/saker/[id]` er åpen |

Én kontroll tok 10–30 minutter på denne maskinen (Colima, høy last).

## Resultater

| Oppgave | Modell | Fullført | Credits per forsøk | Snitt | Tid i CLI |
| --- | --- | --- | --- | ---: | --- |
| K1 | GPT-6.1 Sol | 3 av 3 | 24,92 / 29,17 / 25,35 | 26,48 | 207–287 s |
| K1 | GPT-6 Luna | 3 av 3 | 1,64 / 1,36 / 2,76 | 1,92 | 130–249 s |
| T2 | GPT-6.1 Sol | 3 av 3 | 37,20 / 31,27 / 37,40 | 35,29 | 255–399 s |
| T2 | GPT-6 Luna | 2 av 3 | 1,51 / 1,96 / 1,61 | 1,69 | 147–216 s |

Totalt brukte vi **196,14 credits** av taket på 250. Tallene kommer fra
`assistant_usage_events` (`usage.psv`). Alle 219 radene har modellen og
innsatsnivået vi ba om. I T2 startet agenten underagenter i seks av forsøkene,
med samme modell. De 47 radene fra underagentene er tatt med i summen. Evalueringen tok 87–165 sekunder per forsøk.

**Anslaget mot målingen:** Designforslaget anslo Sol til 20–35 credits og Luna
til 1–3 credits per forsøk. Luna holdt seg innenfor. Sol lå innenfor på K1,
men to av tre T2-forsøk lå over 35. To av Sol-øktene på T2 gikk over den
myke grensen på 30 credits uten å bli stoppet.

**Luna mot Sol:** Luna fullførte 5 av 6 forsøk, Sol 6 av 6. Luna kostet
omtrent 1/17 av Sol (snitt 1,81 mot 30,88 credits per forsøk). Med n=3 per
oppgave er dette bare et hint, ikke bevis. Vi måler på nytt når oppgavene er
gjort vanskeligere.

**Vanskelighetsgrad:** Målet var 50–80 % fullført. Vi fikk 11 av 12 (92 %).
K1 ble fullført i alle seks forsøkene, og T2 i fem av seks. Begge oppgavene er
for lette i denne formen.

## Gjennomgang av feil

- **T2, Luna 3: reell feil.** Agenten rettet bare `/api/intern/saker`, som
  prompten nevnte, og lot `/api/intern/saker/[id]` stå åpen. Det er samme
  snarvei som `invalid-3`.
- **K1, Sol 2 og Luna 3: feil i evaluatoren, nå rettet.** Begge rettet begge
  feilene i koden. Men Gradle kunne ikke kjøre inne i cplt, så agentene skrev
  nye offentlige tester de ikke fikk kompilert. Evaluatoren kompilerte disse
  testene sammen med de skjulte, og bygget feilet (`untestable`). Nå sletter
  evaluatoren agentens `src/test` før de skjulte testene settes inn. De seks
  K1-forsøkene ble vurdert på nytt uten nye modellkall. K1-kontrollene ble
  kjørt tre ganger til etter endringen, med samme utfall. Resultatene i
  `resultater/` er fra den nye vurderingen.

## Endringer i evaluatoren

- Docker-volum i stedet for bind mount. Colima uten delte mapper ga tomme
  mapper i containeren. Gradle-cachen ligger nå i volumet
  `nav-benchmark-gradle-cache`.
- Flere `invalid*.patch` per oppgave. Alle må feile.
- Én Gradle-kjøring per evaluering i stedet for én per testsamling.
- Containeren fjernes ved tidsavbrudd.
- Gradle-cachen monteres skrivebeskyttet (`GRADLE_RO_DEP_CACHE`), så et forsøk
  ikke kan endre cachen for senere forsøk.
- `@navikt/oasis` finnes bare i GitHub Packages. Bildet
  `nav-benchmark-oasis:local` bygges med token som BuildKit-hemmelighet, og
  tokenet lagres ikke i bildet.

## Filer

- `attempts.psv`: forsøk, økt, avslutningskode, sekunder i CLI, credits, status ved første vurdering og endelig status
- `usage.psv`: rådata fra `assistant_usage_events`
- `resultater/`: evaluatorresultat per forsøk
- `patcher/`: endringene agenten gjorde i arbeidsmappa
- `transkripter/`: øktene fra `--share`, med maskinstier og brukernavn fjernet

## Anbefalt justering

1. **T2:** Legg til en tredje intern rute, for eksempel
   `/api/intern/saker/[id]/notater` med `POST`. Da må agenten lete
   systematisk. Test også at `Authorization` med et utløpt token avvises.
2. **K1:** La en tredje klient bruke cachen med en annen audience, slik at
   nøkkelen må dekke begge. Flytt marginen på 30 sekunder fra ADR-en til en
   konfigurasjonsverdi som ikke er i bruk. Ta med samtidighet: to samtidige
   kall for samme bruker skal gi ett kall til Texas.
3. Agenten kunne ikke kjøre Gradle i cplt og fikk ikke installert
   `@navikt/oasis` uten token. Det bør løses før hovedkjøringen. Det påvirker
   begge modellene likt, men gjør oppgaven til lesing heller enn testing.
4. Bruk 40 credits per Sol-forsøk i kostnadsanslaget for slike oppgaver.
