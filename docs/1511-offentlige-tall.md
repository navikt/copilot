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
| `/innsikt/team` | Forbruk og modell/funksjon/språk per team | Team, k ≥ 5 per undergruppe | – | Alle innloggede |
| `/adopsjon`, `copilot-adoption` (`repo_scan`) | Tilpasninger per repo | Repo | Daglige øyeblikksbilder | Innlogget |
| `benchmark/`, `docs/modellvalg.md`, `/modeller` | Modellmålinger og valg | Modell | Git | Alle (offentlig repo og side) |
| `copilot-survey` (Postgres) | Svar på undersøkelser | Person, ment anonymt | Slettes 180 dager etter at undersøkelsen stenger, backup 7 kopier | Plattformteamet |
| `docs/utviklerundersokelsen-2026-oppsummering.md` | Sammendrag, 163 svar | Org | Git | Alle |

### Rutetilgang på ki-utvikling.nav.no

Alle sider, også de private, står i `autoLoginIgnorePaths` i `apps/my-copilot/.nais/app.yaml`. Wonderwall slipper dem gjennom, og den eneste sperren er `PRIVATE_PAGE_PATHS` i `src/proxy.ts`. `scripts/check-public-routes.mjs` kjenner bare `/abonnement` og `/kostnad` som private. Ingen test sjekker at `/statistikk`, `/innsikt/team` og `/adopsjon` faktisk krever innlogging. I prod sendes de i dag til innlogging (307), og `/statistikk/json` svarer 401. De er altså beskyttet, men bare av `proxy.ts`. Som ekstra sikring bør en test fange det hvis den sperren forsvinner.

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
| [Offentleglova §§ 3, 9](https://lovdata.no/lov/2006-05-19-16) | Saksdokumenter og sammenstillinger fra databaser er i utgangspunktet åpne. Kostnadstall for hele Nav kan uansett kreves innsyn i. Personopplysninger er unntatt etter § 13. |
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
| Antall brukere og aktive brukere | Offentlig, bare total for hele Nav | Allerede publisert av kode24 og `/reisen` | Hele Nav, per måned, avrundet til nærmeste 50 | Produkteier, kommunikasjon |
| Vekst i bruk | Offentlig, bare total for hele Nav | Samme | Hele Nav, per måned | Produkteier, kommunikasjon |
| Kostnad totalt | Offentlig, bare total for hele Nav | Kan kreves innsyn i; relativ endring er publisert | Hele Nav, per måned eller kvartal | Produkteier, kommunikasjon |
| Fordeling på modell, editor, språk | Internt | Ingen offentlige fordelinger, bare totaler | – | Produkteier |
| Kodeforslag og aksepterte linjer | Internt | Lett å lese som produktivitetsmål | – | Produkteier |
| Kostnad per oppgave (#1424) | Internt | Metoden er ikke avklart | – | Produkteier |
| Tall per team (#345, `/innsikt/team`) | Internt | Kontrolltiltak etter aml. kap. 9 | Team ≥ 5, per måned, bare innlogget | Produkteier, personvernombud, tillitsvalgte |
| Svar fra undersøkelser | Offentlig aggregert | Samtykke og formål må dekke ekstern bruk | Totaler for hele Nav, ingen fordeling på grupper | Personvernombud, kommunikasjon |
| Alt per person | Aldri | Personopplysninger, kontrolltiltak | – | – |
| Nye skjermbilder av interne sider | Aldri uten gjennomgang | Kan vise flere felt enn tiltenkt. Skjermbildet fra 2025 er et godkjent unntak. | – | Produkteier |

## 5. Godkjenningsrute

| Rolle | Godkjenner |
|---|---|
| Produkteier | Alle rader. Eier listen og reglene. |
| Personvernombud | Undersøkelser og alt på team-nivå. Bekrefter om PVK trengs. |
| Kommunikasjon | Bruker- og kostnadstall før de brukes eksternt. |
| Tillitsvalgte | Drøfting etter aml. § 9-2 før tall per team vises, også internt. Orienteres om den offentlige listen. |

## 6. Tall på `/reisen` i første versjon

Bare tall som allerede er offentlige eller kommer fra repoet.

| Tall | Kilde |
|---|---|
| Rundt 600 daglige brukere (juni 2026) | kode24, 4. juni 2026 |
| 20 % vekst i brukere per måned (juni 2026) | kode24, 4. juni 2026 |
| 3–4 ganger høyere kostnad etter AI Credits | kode24, 4. juni 2026 |
| 900 sammenslåtte PR-er i navikt/copilot (9. oktober 2026) | GitHub search API, kommentar i #1512 |
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
| Ruter (ekstra sikring) | Legg til en test som henter `/statistikk`, `/innsikt/team`, `/adopsjon` og `/statistikk/json` uten token og forventer omdirigering eller 401. Legg rutene i `PRIVATE_ROUTES` i `check-public-routes.mjs`. |

Et live API gir ferskere tall, men også en ny offentlig flate mot BigQuery. Det er ikke verdt det for tall som endres månedlig.

## 8. Åpne spørsmål

1. Hvem er produkteier og dataeier for `copilot-metrics`?
2. Finnes det en PVK for `copilot-metrics` og `copilot-survey`? Hvis ikke, trengs en før tall per team.
3. Er utvidet bruk av kode24-tallene på `/reisen` greit for kommunikasjon?
4. Skal BigQuery-tabellene med persondata få en lagringstid?
5. Dekker samtykket i utviklerundersøkelsen bruk utenfor Nav?

## 9. Steg

| # | Steg | Størrelse |
|---|---|---|
| 1 | Ekstra sikring: test som sjekker at private ruter krever innlogging, og oppdatert `check-public-routes.mjs` | S |
| 2 | Avklare tabellen med produkteier og personvernombud | M |
| 3 | `/reisen`: tallene i punkt 6 med kilde og dato | S |
| 4 | JSON-øyeblikksbilde, test for bare totaler og generatorskript | M |
| 5 | Drøfting med tillitsvalgte om tall per team (#345, #1424) | M |
| 6 | Lagringstid for persondata i BigQuery | S |
| 7 | Første eksterne sak med godkjente tall (#1512) | L |
