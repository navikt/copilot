# Plan: hvilke KI-tall vi kan dele offentlig (#1511)

Status: forslag, ikke godkjent. Ingen tall i dette dokumentet er nye. Alle er allerede offentlige eller hentet fra det åpne repoet.

Relatert: #1510 (kildedata), #1512 (historien), #345 (innsikt per team), #1424 (forbruk per team).

## 1. Hva vi har i dag

| Kilde | Innhold (tabeller, felt) | Nivå | Lagring | Hvem ser det nå |
|---|---|---|---|---|
| `copilot-metrics`, BigQuery `copilot_metrics` | `usage_metrics` (rå NDJSON per dag), `user_metrics`, `user_teams`, `repository_metrics`, `billing_usage`, `billing_usage_reports`, `billing_usage_daily_model`, `billing_user_monthly`, budsjett-tabeller, views `v_*` | Person per dag, team, repo, org | Ingen utløpstid på partisjoner: lagres på ubestemt tid | Plattformteamet med tilgang til GCP-prosjektet |
| `copilot-api` `/api/v1/` | Statistikk, kostnad, budsjett, team-forbruk (`team_spend.go`, `team_usage_composition.go`) | Org og team. Team skjules under 5 bidragsytere (`minTeamContributors = 5`), fordelinger under 5 brukere (`minUsersForDistribution = 5`) | Ingen egen lagring | Innloggede Nav-brukere via my-copilot |
| `copilot-api` `/public/v1/` | Bare video-endepunkter | – | – | Alle |
| `/statistikk`, `/statistikk/json` | Aktive brukere, forslag, språk, editor, modeller, repo-bruk | Org, aggregert | – | Innlogget (`src/proxy.ts`) |
| `/kostnad`, `/abonnement` | Kostnad, budsjett, egen lisens | Org, og egen person | – | Innlogget |
| `/innsikt/team` | Forbruk og modell/funksjon/språk per team | Team med minst 5 bidragsytere. Funksjon og språk har samme terskel per undergruppe; leverandør og modelltype har det ikke (se risiko). | – | Alle innloggede |
| `/adopsjon`, `copilot-adoption` (`repo_scan`) | Tilpasninger per repo | Repo | Daglige øyeblikksbilder | Innlogget |
| `benchmark/`, `docs/modellvalg.md`, `/modeller` | Modellmålinger og valg | Modell | Git | Alle (offentlig repo og side) |
| `copilot-survey` (Postgres) | Svar på undersøkelser | Person, ment anonymt | Slettes 180 dager etter at undersøkelsen stenger, backup 7 kopier | Plattformteamet |
| `docs/utviklerundersokelsen-2026-oppsummering.md` | Sammendrag, 163 svar | Org | Git | Alle |

### Rutetilgang på ki-utvikling.nav.no

Alle sider, også de private, står i `autoLoginIgnorePaths` i `apps/my-copilot/.nais/app.yaml`. Wonderwall slipper dem gjennom. Innloggingen håndheves i appen: `src/proxy.ts` (`PRIVATE_PAGE_PATHS`), og i tillegg kaller hver side og `/statistikk/json` `getUser()`. `src/middleware.test.ts` tester listen over private stier, omdirigering og 401. I prod sendes sidene til innlogging (307), og `/statistikk/json` svarer 401. Hullet er at `scripts/check-public-routes.mjs` bare kjenner `/abonnement` og `/kostnad` som private, så listene i `app.yaml`, `proxy.ts` og skriptet holdes ikke i takt automatisk. Som ekstra sikring bør skriptet sjekke at de private rutene står i `PRIVATE_PAGE_PATHS`.

### Allerede offentlig

| Tall | Hvor |
|---|---|
| Rundt 600 daglige Copilot-brukere, 20 % vekst per måned, 3–4 ganger høyere kostnad | [kode24, 4. juni 2026](https://www.kode24.no/artikkel/nav-ma-betale-tre-til-fire-ganger-mer-for-sine-600-copilot-brukere/264699), sitat fra Navs plattformutvikler |
| 111 aktive brukere og 54 chatbrukere (januar 2025), skjermbilde av intern statistikk | `/reisen`, `public/images/reisen/statistikk-2025.webp` |
| Over 300 aktive brukere; 709 000 forslag, 1,22 mill. genererte linjer, 190 000 tatt i bruk på 100 dager | `/reisen` (`milestones.ts`) |
| «Rundt 600 daglige brukere», «tre til fire ganger høyere regning» | `/reisen` |
| «Nav får ca. 950 000 credits i måneden» (avslører antall Business-lisenser), «600 brukere» | `docs/news/articles/model-pinning-kostnadsoptimalisering.md` |
| «Navs ~500 utviklere» | `docs/news/articles/mars-2026.md` |
| 163 svar på utviklerundersøkelsen, prosentandeler | `docs/news/articles/utviklerundersokelsen-2026.md` |
| Listepriser per modell | Flere nyhetssaker (offentlige priser fra leverandørene) |

Disse kan ikke trekkes tilbake, og de setter presedens for tall på org-nivå.

Godkjente unntak, bestemt av produkteier:

- Tallene fra 2025 på `/reisen` (skjermbildet med 111 og 54, over 300 brukere og tallene for 100 dager) blir stående. De er historiske, gjelder hele Nav og er godkjent av produkteier.
- Tallene i nyhetssakene (950 000 credits, ~500 utviklere) blir stående. De gjelder hele Nav, og antall lisenser er ikke sensitivt.

## 2. Regler

| Regel | Hva den betyr for oss |
|---|---|
| GDPR art. 4 nr. 1 og fortale 26 | Aggregater er personopplysninger hvis en person kan identifiseres med rimelige midler. Anonyme tall faller utenfor. |
| [Datatilsynets veileder om anonymisering (2015)](https://www.datatilsynet.no/globalassets/global/dokumenter-pdfer-skjema-ol/regelverk/veiledere/anonymisering-veileder-041115.pdf) | Gruppestørrelse vurderes konkret. Fire eller færre gir høy risiko for identifisering. Internt bruker vi 5 som gulv. Offentlig viser vi bare totaler for hele Nav. |
| [Arbeidsmiljøloven kap. 9](https://lovdata.no/lov/2005-06-17-62/§9-1) | Kontrolltiltak må ha saklig grunn og ikke være uforholdsmessig (§ 9-1). De skal drøftes med tillitsvalgte så tidlig som mulig (§ 9-2). Tall per team kan oppleves som kontroll; tall for hele Nav er det i praksis ikke. |
| [Datatilsynet om personvern på arbeidsplassen](https://www.datatilsynet.no/personvern-pa-ulike-omrader/personvern-pa-arbeidsplassen/) | Overvåking av ansattes bruk av IT-utstyr er strengt regulert (forskrift om innsyn i e-post og elektronisk lagret materiale). Publisering per team eller person øker risikoen. |
| [Offentleglova §§ 3, 9](https://lovdata.no/lov/2006-05-19-16) | Saksdokumenter og sammenstillinger fra databaser er i utgangspunktet åpne. Etter § 9 kan det kreves innsyn i en sammenstilling fra en database hvis den kan lages med enkle framgangsmåter, så samlede kostnadstall kan trolig kreves utlevert. Andre unntak kan likevel gjelde. § 13 unntar opplysninger som er underlagt lovbestemt taushetsplikt, ikke alle personopplysninger. |
| Interne regler i repoet | #345: tall per team, n ≥ 5, aldri per person, ingen rangering. `docs/copilot-team-spend-research.md`: undertrykk i API-et, fast månedsnivå. `copilot-survey`: svar skal være anonyme, personvernombudet bør bekrefte. |

Ikke verifisert: om Nav har en egen veileder for publisering av statistikk om ansattes KI-bruk, og om det finnes en PVK for `copilot-metrics`. Se åpne spørsmål.

## 3. Presedens

- [GDS, AI coding assistant trial](https://www.gov.uk/government/publications/ai-coding-assistant-trial/ai-coding-assistant-trial-uk-public-sector-findings-report): offentlig rapport om kodeassistenter i britisk offentlig sektor.
- [GDS, Microsoft 365 Copilot cross-government findings](https://www.gov.uk/government/publications/microsoft-365-copilot-experiment-cross-government-findings-report/microsoft-365-copilot-experiment-cross-government-findings-report-html): 20 000 deltakere, tidsbesparelse og adopsjonsrate, bare på aggregert nivå.
- [HMRC-evaluering av Copilot](https://www.gov.uk/government/publications/evaluation-report-phase-3-trial-of-microsoft-copilot/evaluating-the-impact-of-microsoft-copilot-in-hmrc).

Mønsteret: offentlige etater deler brukertall, adopsjonsrate og opplevd nytte for hele virksomheten, ikke per enhet eller person.

## 4. Beslutningstabell

| Data | Nivå | Begrunnelse | Minste aggregering | Godkjenner |
|---|---|---|---|---|
| Benchmark-resultater og modellvalg | Offentlig | Allerede i åpent repo | Per modell | Produkteier |
| Størrelse på katalogen (skills, agenter, instruksjoner) | Offentlig | Repo-data, vises på `/reisen` | – | Produkteier |
| Repo-aktivitet (sammenslåtte PR-er, PR-er fra Copilot coding agent) | Offentlig | Åpent repo, ingen persondata i tallet | Hele repoet | Produkteier |
| Antall brukere og aktive brukere | Offentlig, bare total for hele Nav | Allerede publisert av kode24 og `/reisen` | Hele Nav, per måned, avrundet til nærmeste 50 | Produkteier (kommunikasjon orienteres) |
| Vekst i bruk | Offentlig, bare total for hele Nav | Samme | Hele Nav, per måned | Produkteier (kommunikasjon orienteres) |
| Kostnad totalt | Offentlig, bare total for hele Nav | Kan kreves innsyn i; relativ endring er publisert | Hele Nav, per måned eller kvartal | Produkteier (kommunikasjon orienteres) |
| Fordeling på modell, editor, språk | Internt | Ingen offentlige fordelinger, bare totaler | – | Produkteier |
| Kodeforslag og aksepterte linjer | Internt | Lett å lese som produktivitetsmål | – | Produkteier |
| Kostnad per oppgave (#1424) | Internt | Metoden er ikke avklart | – | Produkteier |
| Tall per team (#345, `/innsikt/team`) | Internt | Kontrolltiltak etter aml. kap. 9 | Team ≥ 5, per måned, bare innlogget | Produkteier, personvernombud, tillitsvalgte |
| Publisert sammendrag av undersøkelser | Offentlig | Bare sammendraget i repoet kan brukes utenfor Nav | Totaler for hele Nav, slik de står i sammendraget | Produkteier |
| Rå svar fra undersøkelser | Aldri | Skal aldri brukes, verken offentlig eller i nye analyser utenfor Nav | – | – |
| Alt per person | Aldri | Personopplysninger, kontrolltiltak | – | – |
| Nye skjermbilder av interne sider | Aldri uten gjennomgang | Kan vise flere felt enn tiltenkt. Skjermbildet fra 2025 er et godkjent unntak. | – | Produkteier |

## 5. Godkjenningsrute

| Rolle | Godkjenner |
|---|---|
| Produkteier | Godkjenner alene publisering av totaler for hele Nav. Eier listen og reglene. |
| Personvernombud | Rådføres bare når vi vil bruke en ny type data. |
| Kommunikasjon | Orienteres før tall brukes eksternt. Godkjenner ikke. |
| Tillitsvalgte | Drøfting etter aml. § 9-2 før tall per team vises, også internt. Orienteres om den offentlige listen. |

### Kjente mangler og risiko

| Punkt | Status |
|---|---|
| Ingen PVK for `copilot-metrics` eller `copilot-survey` | Kjent mangel. En PVK anbefales, men stopper ikke publisering av totaler for hele Nav som allerede er offentlige eller godkjent av produkteier. |
| Ingen lagringstid for persontabellene i BigQuery (`billing_user_monthly`, `user_metrics` og lignende) | Bevisst valg foreløpig. Skal vurderes på nytt. Risiko: jo lenger data lagres, jo større blir skaden ved en lekkasje eller feil bruk. |
| `/innsikt/team` (#1419) viser allerede tall per team internt, uten dokumentert drøfting med tillitsvalgte | Siden blir stående som den er. Drøftingen starter nå, med [grunnlaget for drøfting](1511-drofting-innsikt-team.md). |
| Fordelingen på leverandør og modelltype på `/innsikt/team` skjuler ikke små grupper. I et team på fem kan én person skille seg ut. | Åpen risiko, ikke prioritert nå. Løsning: bruk regelen om minst fem også på disse fordelingene. |

## 6. Tall på `/reisen` i første versjon

Bare tall som allerede er offentlige eller kommer fra repoet.

| Tall | Kilde |
|---|---|
| Rundt 600 daglige brukere (juni 2026) | kode24, 4. juni 2026 |
| 20 % vekst i brukere per måned (juni 2026) | kode24, 4. juni 2026 |
| 3–4 ganger høyere kostnad etter AI Credits | kode24, 4. juni 2026 |
| 900 sammenslåtte PR-er i navikt/copilot (9. oktober 2026) | GitHub search API (`repo:navikt/copilot is:pr is:merged`), kommentar i #1512 |
| Antall skills, agenter og instruksjoner | Katalogen, vises allerede |
| Antall modeller målt og antall benchmark-kjøringer | `benchmark/`, `docs/modellvalg.md` |

Hvert tall får dato og kildelenke. Nye bruks- og kostnadstall venter på godkjenning.

## 7. Teknisk løsning

| Valg | Forslag |
|---|---|
| Regel | Offentlige tall er bare totaler for hele Nav. Ingen fordeling på team, modell, editor, språk eller andre grupper. |
| Kilde | Øyeblikksbilde ved bygg: en JSON-fil i repoet (`apps/my-copilot/src/data/offentlige-tall.json`) med verdi, dato og kilde. Ikke et live API. |
| Oppdatering | Månedlig PR, generert av et skript som leser totaler fra BigQuery. En person godkjenner i PR-en. |
| Test | En test feiler hvis øyeblikksbildet eller et offentlig endepunkt har noe annet enn totaler for hele Nav: et felt som ikke står på godkjent liste, en liste eller gruppering, eller et tall uten kilde og dato. |
| Ruter (ekstra sikring) | La `check-public-routes.mjs` lese `PRIVATE_PAGE_PATHS` fra `proxy.ts`, eller sjekke at listene stemmer, så en privat rute ikke kan bli offentlig uten at bygget feiler. |

Et live API gir ferskere tall, men også en ny offentlig flate mot BigQuery. Det er ikke verdt det for tall som endres månedlig.

## 8. Steg

| # | Steg | Størrelse |
|---|---|---|
| 1 | Ekstra sikring: test som sjekker at private ruter krever innlogging, og oppdatert `check-public-routes.mjs` | S |
| 2 | Produkteier godkjenner tabellen | S |
| 3 | `/reisen`: tallene i punkt 6 med kilde og dato | S |
| 4 | JSON-øyeblikksbilde, test for bare totaler og generatorskript | M |
| 5 | Drøfting med tillitsvalgte om `/innsikt/team` og tall per team (#345, #1424) | M |
| 6 | Gjennomføre PVK for `copilot-metrics` og `copilot-survey` (anbefalt, blokkerer ikke steg 3) | M |
| 7 | Første eksterne sak med godkjente tall (#1512) | L |
| 8 | Vurdere lagringstid for persontabellene i BigQuery på nytt | S |
