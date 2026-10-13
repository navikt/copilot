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

## Runde 2: vanskeligere oppgaver, 12.–13. oktober 2026

Etter første runde gjorde vi begge oppgavene vanskeligere, slik det ble anbefalt
over. Filene ligger i [`runde-2/`](runde-2/), med samme oppsett som over.

### Hva som er endret

- **K1:** En ny klient, `ArkivClient`, bruker samme cache mot `arkiv-api`. Marginen
  står i `TokenConfig.expiryLeeway`, men starttilstanden bruker den ikke. ADR-en
  krever at samtidige forespørsler fra samme bruker deler én veksling, og prompten
  nevner topper med samtidige vekslinger. Nye skjulte tester: samme bruker får ett
  token per API, marginen hentes fra konfigurasjonen, og to samtidige kall gir ett
  kall til Texas. Nye kontroller: `invalid-4` (nøkkel bare på bruker), `invalid-5`
  (ingen lås) og `invalid-6` (marginen er hardkodet).
- **T2:** En tredje rute, `POST /api/intern/saker/[id]/notater`, har samme feil.
  De skjulte testene avviser også et utløpt token. Ny kontroll: `invalid-4` retter
  begge GET-rutene, men ikke `POST`.
- **Agentmiljøet:** Agenten kan nå kjøre Gradle og npm i cplt. Tre ting hindret det i
  runde 1:
  1. cplt nekter å kjøre programmer under `/tmp` og `~/.cache`. Forsøkene ligger nå
     i `~/nav-benchmark-attempts/`.
  2. Gradle-klienten snakker med daemonen over localhost, og `--proxy-forced`
     stenger localhost. Vi bruker `--with-proxy --default-allowlist
     --allow-localhost-any` i stedet. Det svekker isolasjonen: Uten
     `--proxy-forced` gjelder domenelista bare trafikk som går gjennom proxyen.
     Agenten kan likevel koble seg direkte til port 443 og nå alle porter på
     localhost. Kravet i #1409 om en agentcontainer uten nettverk er derfor
     fortsatt ikke oppfylt.
  3. `JAVA_HOME` pekte på JDK 27, som Gradle 8.14.5 ikke støtter. Vi sender med
     JDK 21.

  npm-avhengighetene installeres før agenten starter, med maskinens egen
  GitHub Packages-token. Agenten får ikke tokenet. Vi fant det ikke i noen
  transkripter.
- **Evaluatoren:** Hver evaluering kopierer versjonscachen til Gradle fra den varme
  cachen, mens avhengighetene fortsatt er skrivebeskyttet. Uten kopien tok én
  Gradle-kjøring 8 minutter på denne maskinen, og alle kontrollene stoppet på
  tidsgrensen.

### Kontroller

Hver oppgave ble kjørt 10 ganger før første modellkall: K1 med åtte tilstander og
T2 med seks. Alle kjøringene ga samme resultat, uten ustabile utfall
(`runde-2/kontroller-*.log`).

### Resultater

| Oppgave | Modell | Runde 1 | Runde 2 | Credits per forsøk, runde 2 | Snitt | Tid i CLI |
| --- | --- | --- | --- | --- | ---: | --- |
| K1 | GPT-6.1 Sol | 3 av 3 | 3 av 3 | 24,65 / 22,03 / 23,18 | 23,29 | 263–307 s |
| K1 | GPT-6 Luna | 3 av 3 | 3 av 3 | 1,65 / 1,39 / 1,55 | 1,53 | 144–151 s |
| T2 | GPT-6.1 Sol | 3 av 3 | 2 av 3 | 32,06 / 37,82 / 32,07 | 33,98 | 254–306 s |
| T2 | GPT-6 Luna | 2 av 3 | 1 av 3 | 2,39 / 2,29 / 1,71 | 2,13 | 142–203 s |

Runde 2 brukte **182,79 credits** av budsjettet på 250. Budsjettet er ikke et
hardt tak, fordi `--max-ai-credits` er en myk grense. `batch.sh` setter av 40
credits per Sol-forsøk og 5 per Luna-forsøk, og starter ikke et forsøk som ikke
får plass. Sammen med runde 1 er
det 378,93. Evalueringen tok 122–181 sekunder per forsøk.

Alle bruksradene har innsatsnivået `medium`. 219 av 223 rader har modellen vi
ba om. Fire rader er fra underagenter som kjørte `gpt-5.6-luna` i Luna-forsøk 1
og 2 på T2. De kostet til sammen 0,52 credits. Copilot CLI valgte den modellen
selv. Forsøkene er tatt med, men de er ikke rene GPT-6 Luna-målinger.

**Vanskelighetsgrad:** I alt ble 9 av 12 forsøk fullført (75 %), mot 11 av 12 i
runde 1. Det er innenfor målet på 50–80 %, men bare fordi T2 ble vanskeligere.
T2 traff 3 av 6 (50 %). K1 traff fortsatt 6 av 6 og er for lett.

**Anslaget mot målingen:** Sol brukte 22–38 credits per forsøk, under budsjettet
på 40. Luna brukte 1,4–2,4 credits.

### Luna mot Sol

| | Fullført | Snitt credits per forsøk |
| --- | --- | ---: |
| GPT-6.1 Sol | 5 av 6 | 28,64 |
| GPT-6 Luna | 4 av 6 | 1,83 |

Luna kostet omtrent 1/16 av Sol. Sol fullførte ett forsøk mer, og forskjellen
ligger i T2. Med n=3 per oppgave er dette fortsatt bare et hint, ikke bevis.

### Gjennomgang av feil

Alle tre feilene er reelle. Ingen skyldes evaluatoren:

- **T2, Luna 1, Sol 2 og Luna 3:** Agenten rettet bare `/api/intern/saker`,
  som prompten nevnte. `[id]`-ruten og `POST …/notater` ble stående åpne. Luna 1
  la også ruten til i `matcher`. Ingen av dem søkte etter andre route handlers
  som brukte `getToken` uten validering.

I K1 rettet alle seks forsøkene alle fire kravene, og agentene kjørte Gradle selv
i cplt. Vi har lest gjennom patchene: alle bruker `(bruker, audience)` som nøkkel,
henter marginen fra `TokenConfig` og har en lås per nøkkel.

### Anbefaling

- **T2** kan brukes i hovedkjøringen som den er.
- **K1** er fortsatt for lett. Vi foreslår å flytte hele cache-logikken bak et grensesnitt med to
  implementasjoner (for eksempel én for TokenX og én for Azure OBO), der bare
  den ene har feilene. Et annet alternativ er å kutte K1 og bruke K4 eller K3 fra
  designforslaget.
- Budsjetter 35 credits per Sol-forsøk og 2,5 per Luna-forsøk.
