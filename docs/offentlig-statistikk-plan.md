# Plan: offentlig statistikk på ki-utvikling.nav.no (#1511)

Status: forslag. Bygger på reglene i #1520 (`docs/1511-offentlige-tall.md`). Dokumentet har ingen nye bruks- eller kostnadstall. Alle tall i eksempler er oppdiktet.

## Hvorfor

Tall for hele Nav kan uansett kreves innsyn i etter [offentleglova § 9](https://lovdata.no/lov/2006-05-19-16/§9) (sammenstilling fra databaser). Vi publiserer dem derfor selv, én gang i måneden, i stedet for å svare på innsynskrav ett for ett.

Regler, godkjent av produkteier:

- Bare totaler for hele Nav, per måned.
- Ingen fordeling på team, modell, editor, språk eller person.
- Produkteier godkjenner alene. Kommunikasjon orienteres.

## Siden

| Valg | Forslag |
|---|---|
| Rute | `/tall`. Ledig: ingen side, ingen omdirigering i `next.config.ts`, ikke i `PRIVATE_PAGE_PATHS`. `/statistikk/offentlig` går ikke: `proxy.ts` krever innlogging for alt under `/statistikk/`. |
| Formål | Vise hvor mye Nav bruker GitHub Copilot og hva det koster, med kilde og dato for hvert tall. |
| Målgruppe | Journalister, andre etater, innbyggere og Nav-ansatte uten innlogging. |
| Tilgang | Offentlig. Ingen innlogging, ingen API. |

Oppsett:

| # | Seksjon | Innhold |
|---|---|---|
| 1 | Innledning | Hva tallene er, at de gjelder hele Nav, oppdatert dato. |
| 2 | Nøkkeltall | Fire kort for siste hele måned: lisenser, aktive brukere, AI Credits, kostnad i kroner. Endring fra forrige måned. |
| 3 | Utvikling | Ett linjediagram per tall, alle måneder i filen. Tabell under hvert diagram (tilgjengelighet). |
| 4 | Katalog og målinger | Antall skills, agenter og instruksjoner; antall modeller målt. |
| 5 | Om tallene | Definisjon, kilde og avrunding for hvert tall. Rettede måneder merkes. |
| 6 | Innsyn | Lenke til offentleglova og hvordan man ber om mer. |

## Tallkatalog

| Felt | Definisjon | Kilde | Aggregering | Avrunding | Enhet | Hvorfor trygt | Fase |
|---|---|---|---|---|---|---|---|
| `lisenser` | Copilot-lisenser i Nav ved månedsslutt | GitHub billing API `seat_breakdown.total` (samme som `githubSeatsTotal` i copilot-api). Lagres ikke historisk i BigQuery i dag. | Verdien når jobben kjører | Nærmeste 50 | antall | Innkjøpstall, ikke om personer. Allerede antydet i nyhetssak. | 1 |
| `aktive_brukere` | `monthly_active_users` på siste dag i måneden | `v_daily_summary`, `scope = 'enterprise'` | Én verdi per måned | Nærmeste 50 | antall | Total for hele Nav, publisert av kode24 tidligere | 1 |
| `ai_credits` | Sum brukte AI Credits i måneden | `billing_usage_daily_model`, `SUM(net_quantity)`. **Uklart:** hvilken `unit_type` som er AI Credits, og om `billing_usage_reports` er riktigere. | Sum per måned | Nærmeste 10 000 | AI Credits | Kostnad kan kreves innsyn i | 1 |
| `kostnad_nok` | Netto kostnad i måneden omregnet til kroner | `v_billing_monthly_trend.total_net_amount` (USD) × månedssnitt USD/NOK fra [Norges Banks valutakurser](https://www.norges-bank.no/tema/Statistikk/Valutakurser/) (API: `data.norges-bank.no`, serie `EXR/M.USD.NOK.SP`) | Sum per måned | Nærmeste 10 000 kr | NOK | Kan kreves innsyn i. Kursen oppgis som kilde. | 1 |
| `katalog_elementer` | Skills, agenter og instruksjoner i katalogen | `copilot-manifest.json` i repoet | Antall ved generering | Ingen | antall | Åpent repo | 1 |
| `modeller_malt` | Modeller med minst én målt kjøring | `benchmark/` | Antall ved generering | Ingen | antall | Åpent repo | 1 |
| `engasjerte_brukere` | **Uklart.** Ingen definisjon i dag. Forslag: distinkte brukere med `user_initiated_interaction_count > 0` i `user_metrics`, telt i SQL. | `user_metrics` | Distinkt per måned | Nærmeste 50 | antall | Total, men krever ny definisjon | Senere |
| `bruk_totalt` | Brukerstartede interaksjoner, alle funksjoner | `v_daily_summary.total_interactions` | Sum per måned | Nærmeste 1 000 | antall | Total, men kan leses som produktivitetsmål | Senere |
| `pr_copilot_agent` | PR-er opprettet av Copilot coding agent i navikt | `v_daily_summary.pr_created_by_copilot`. **Uklart:** om GitHubs tall gjelder hele enterprise eller bare navikt, og om det dekker coding agent alene. | Sum per måned | Nærmeste 10 | antall | Ingen persondata i tallet | Senere |

Nye felt krever at produkteier godkjenner både feltet og en endring i `FELT` i skjemaet.

## Dataflyt

| Valg | Forslag |
|---|---|
| Jobb | GitHub Action i navikt/copilot, `workflow_dispatch` og cron den 5. hver måned. Leser BigQuery med Workload Identity Federation og en tjenestekonto som bare har tilgang til views. Ikke en nais-jobb: resultatet skal uansett bli en PR. |
| Generator | `apps/my-copilot/scripts/generate-offentlig-statistikk.ts`. Kjører SQL med `SUM`/`MAX` per måned, slik at bare én verdi per felt og måned forlater BigQuery. Avrunder før skriving. |
| Utdata | `apps/my-copilot/src/data/offentlig-statistikk.json`, hele historikken, nøkkel per måned. |
| Publisering | PR fra jobben. Produkteier godkjenner. Siden leser filen ved bygg. Ingen offentlig API. |
| Sene data | Bare hele måneder. En måned tas med når alle dager har data (`isMonthComplete` i `month-utils.ts`); ellers venter den til neste kjøring. |
| Rettelser | Jobben regner ut de tre siste månedene på nytt. Endrede verdier vises i PR-en, og siden merker dem «rettet» med dato. |

## Sikkerhet

| Tiltak | Hvordan |
|---|---|
| Liste over godkjente felt | `FELT` i `offentlig-statistikk.schema.ts`, med avrunding per felt |
| Test | `offentlig-statistikk.schema.test.ts` feiler på ukjente felt, lister, grupperinger, tall uten kilde eller dato, og tall som ikke er avrundet |
| Avrunding | Gjøres i generatoren. Testen sjekker den. |
| Ingen persondata ut | SQL aggregerer i BigQuery. Generatoren leser aldri rader per person. |
| Eksempeldata | Fixturen har `eksempel: true` og oppdiktede tall. Siden skal nekte å vise en fil med `eksempel: true`. |
| Rute | `/tall` legges i listen over offentlige ruter i `check-public-routes.mjs` |

## Innsyn og offentleglova

| Situasjon | Svar |
|---|---|
| Krav om tall som står på siden | Henvis til `/tall`. Det er nok etter offentleglova § 9. |
| Krav om tall for hele Nav som ikke står der | Behandles som vanlig innsynskrav. Vurder å legge tallet til siden. |
| Krav om tall per team eller person | Vurderes etter [§ 13](https://lovdata.no/lov/2006-05-19-16/§13) (taushetsplikt) og personvernreglene. Personvernombudet rådføres. |

Siden lenker til [offentleglova](https://lovdata.no/lov/2006-05-19-16) og forklarer hvordan man ber om innsyn.

## Roller

| Rolle | Oppgave |
|---|---|
| Produkteier | Godkjenner hver månedlige PR og hvert nytt felt |
| Kommunikasjon | Orienteres før lansering |
| Plattformteamet | Drifter jobben og svarer på spørsmål om tallene |

## Steg

| # | PR | Størrelse | Blokkeres av |
|---|---|---|---|
| 1 | Denne PR-en: plan, skjema, test og eksempelfil | S | – |
| 2 | Generator og SQL, kjørt lokalt mot BigQuery; første fil med ekte tall, godkjent av produkteier | M | 1, avklaring av `ai_credits` |
| 3 | Siden `/tall` med nøkkeltall, diagrammer og «Om tallene»; rute i `check-public-routes.mjs` | M | 2 (kan bygges mot eksempelfilen) |
| 4 | GitHub Action med WIF og månedlig PR | S | 2 |
| 5 | Tall fra fase «Senere», ett felt per PR | S | 3, godkjenning per felt |

Kommunikasjon orienteres før PR 3 slås sammen.

## Åpne spørsmål

1. Hvilken `unit_type` og tabell gir riktig sum av AI Credits?
2. Skal kostnad vises før eller etter rabatt? Forslaget er netto, altså det Nav betaler.
3. Er avrunding til 50 brukere og 10 000 kr riktig nivå?
4. Fra hvilken måned skal historikken starte?
