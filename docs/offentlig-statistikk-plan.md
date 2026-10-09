# Plan: offentlig statistikk på ki-utvikling.nav.no (#1511)

Status: forslag. Bygger på reglene i #1520 (`docs/1511-offentlige-tall.md`). Dokumentet har ingen nye bruks- eller kostnadstall. Alle tall i eksempler er oppdiktet.

## Hvorfor

Tall for hele Nav kan uansett kreves innsyn i etter [offentleglova § 9](https://lovdata.no/lov/2006-05-19-16/§9) (sammenstilling fra databaser). Vi publiserer dem derfor selv, én gang i måneden, i stedet for å svare på innsynskrav ett for ett.

Regler, godkjent av produkteier:

- Bare tall for hele Nav, per måned. Ingen tall per team, editor, språk eller person.
- Tall per bruker vises som median og snitt. Aldri maksimum eller persentiler over p90, fordi ytterpunktene peker på enkeltpersoner.
- To fordelinger er tillatt: bruksband og andel per modellfamilie. Hver gruppe må ha minst 20 brukere i måneden, ellers slås den sammen med en annen.
- Produkteier godkjenner alene. Kommunikasjon orienteres.

**Endring av regelen i #1520.** Regelen der er «bare totaler, ingen fordeling på modell». Denne planen tillater bruksband og andel per modellfamilie med terskelen på 20 brukere. #1520 må endres på samme måte.

## Siden

| Valg | Forslag |
|---|---|
| Rute | `/innsikt/tall`, under den offentlige innsiktssiden `/innsikt` (sammen med `/innsikt/lokale-modeller`). Ledig: ingen side, ingen omdirigering i `next.config.ts`. `proxy.ts` gjør bare `/innsikt/team` og undersider private, så `/innsikt/tall` er offentlig. `/statistikk/offentlig` går ikke: alt under `/statistikk/` krever innlogging. |
| Formål | Vise hvor mye Nav bruker GitHub Copilot, målt i brukere, lisenser og AI Credits per bruker, med kilde og dato for hvert tall. |
| Målgruppe | Journalister, andre etater, innbyggere og Nav-ansatte uten innlogging. |
| Tilgang | Offentlig. Ingen innlogging, ingen API. |

Oppsett:

| # | Seksjon | Innhold |
|---|---|---|
| 1 | Innledning | Hva tallene er, at de gjelder hele Nav, oppdatert dato. |
| 2 | Nøkkeltall | Kort for siste hele måned: lisenser, aktive brukere, median AI Credits per bruker. Snittet står i mindre skrift under. Endring fra forrige måned. |
| 3 | Utvikling | Linjediagram per nøkkeltall, alle måneder i filen. Tabell under hvert diagram (tilgjengelighet). |
| 4 | Bruksband | Stablet stolpe per måned: andel lett, middels og tung bruk. Tersklene står i teksten. |
| 5 | Modeller | Stablet arealdiagram: andel av AI Credits per modellfamilie per måned. Alle familier vises hver måned. |
| 6 | Katalog og målinger | Antall skills, agenter og instruksjoner; antall modeller målt. |
| 7 | Om tallene | Definisjon, kilde og avrunding for hvert tall. Rettede måneder merkes. |
| 8 | Innsyn | Lenke til offentleglova og hvordan man ber om mer. |

## Tallkatalog

Nevneren for alle tall per bruker er `aktive_brukere`: brukere som er aktive i måneden etter GitHubs definisjon av `monthly_active_users`.

| Felt | Definisjon | Kilde | Aggregering | Avrunding | Enhet | Hvorfor trygt | Fase |
|---|---|---|---|---|---|---|---|
| `lisenser` | Copilot-lisenser i Nav ved månedsslutt | GitHub billing API `seat_breakdown.total` (samme som `githubSeatsTotal` i copilot-api). **Uklart:** lagres ikke historisk i BigQuery. | Verdien når jobben kjører | Nærmeste 50 | antall | Innkjøpstall, ikke om personer | 1 |
| `aktive_brukere` | `monthly_active_users` på siste dag i måneden | `v_daily_summary`, `scope = 'enterprise'` | Én verdi per måned | Nærmeste 50 | antall | Total for hele Nav, publisert av kode24 tidligere | 1 |
| `credits_per_bruker_median` / `_snitt` | Median og snitt av AI Credits per aktiv bruker. Aktive brukere uten forbruk teller med som 0. | `billing_user_monthly.net_amount` per bruker (USD) / 0,01. 1 AI Credit = 0,01 USD, slik det står i `src/lib/model-pricing.ts` og på `/priser`. Filen har bare en kommentar, så PR 2 legger til en eksportert konstant der og bruker den. **Uklart:** om brukerne i tabellen samsvarer med `monthly_active_users`. | `APPROX_QUANTILES(...)[OFFSET(50)]` og `AVG` i SQL | Nærmeste 10 | AI Credits | Bare to tall for over 20 brukere; ingen ytterpunkter | 1 |
| `bruksband` | Andel aktive brukere med lett (under 100), middels (100–999) og tung (minst 1 000) bruk, målt i AI Credits i måneden. Tersklene er faste og står i siden. Tallene i parentes er forslag. | Samme som `credits_per_bruker` | `COUNTIF` per band i SQL | Hel prosent | % | Faste grenser, ikke utledet av dataene. Band under 20 brukere slås sammen med naboen (`lett_middels` eller `middels_tung`). | 1 |
| `modellandeler` | Hver modellfamilies andel av alle AI Credits i måneden. Alle familier hver måned, også med 0 %. | Andel: `billing_usage_daily_model`, `SUM(net_quantity)` per `model`, gruppert med `modellfamilie()`. Brukere per familie: **Uklart.** Tabellen har ikke brukere. Forslag: `user_metrics` (`totals_by_model_feature`), telt med `COUNT(DISTINCT user_id)` i SQL. | Sum per måned | Hel prosent | % | Ikke om personer. En familie med under 20 brukere i måneden legges i «andre»; antallet publiseres ikke. Familier i stedet for versjoner gir en stabil serie over tid. | 1 |
| `katalog_elementer` | Skills, agenter og instruksjoner i katalogen | `copilot-manifest.json` i repoet | Antall ved generering | Ingen | antall | Åpent repo | 1 |
| `modeller_malt` | Modeller med minst én målt kjøring | `benchmark/` | Antall ved generering | Ingen | antall | Åpent repo | 1 |
| `ansiennitet` | Aktive brukere etter kvartal de først var aktive | `user_metrics`, `MIN(day)` per bruker i SQL, bare antall per kvartal ut | Per kvartal | Hel prosent | % | Kan regnes ut uten eksport per person, men krever historikk tilbake til start | Senere |
| `engasjerte_brukere` | **Uklart.** Ingen definisjon i dag. Forslag: brukere med `user_initiated_interaction_count > 0` i `user_metrics`. | `user_metrics` | Distinkt per måned | Nærmeste 50 | antall | Total, men krever ny definisjon | Senere |
| `pr_copilot_agent` | PR-er opprettet av Copilot coding agent | `v_daily_summary.pr_created_by_copilot`. **Uklart:** om tallet gjelder hele enterprise eller bare navikt. | Sum per måned | Nærmeste 10 | antall | Ingen persondata i tallet | Senere |

### Modellfamilier

Mønstrene ligger i `modellfamilie()` i `offentlig-statistikk.schema.ts`, som både generatoren og testen bruker. Navnet gjøres om til små bokstaver med bindestrek, og første treff vinner. En ny versjon, for eksempel «Claude Opus 6», havner i riktig familie av seg selv. Et ukjent navn går til «andre» og logges, så tabellen kan oppdateres.

| Familie | Mønster | Eksempler fra `model-pricing.ts` |
|---|---|---|
| `claude_opus` | `claude-opus*` | Claude Opus 4.8, 5, 5.5 |
| `claude_sonnet` | `claude-sonnet*` | Claude Sonnet 4.6, 5, 5.5 |
| `claude_haiku` | `claude-haiku*` | Claude Haiku 4.5, 5.5 |
| `claude_fable` | `claude-fable*` | Claude Fable 5, 5.1 |
| `gpt_mini` | `gpt-*` med `mini`, `nano` eller `luna` | GPT-5 mini, GPT-5.4 nano, GPT-6 Luna |
| `gpt` | resten av `gpt-*` | GPT-5.5, GPT-5.3-Codex, GPT-6 Sol, Astra, Terra |
| `gemini` | `gemini*` | Gemini 3.7 Flash, 3.8 Flash |
| `andre` | alt annet, og familier under 20 brukere | Kimi K3, MAI-Code-1.1-Flash |

Claude Fable er med fordi den finnes i prislisten. OpenAIs resonneringsmodeller (o1, o3, o4-mini) er ikke med, fordi de ikke lenger står i prislisten.

Siden viser ikke kroner, bare AI Credits. Totalt forbruk av AI Credits og totalkostnad publiseres ikke. Innsynskrav om dem behandles som vanlig (se under).

Nye felt krever at produkteier godkjenner både feltet og en endring i skjemaet.

## Dataflyt

| Valg | Forslag |
|---|---|
| Jobb | GitHub Action i navikt/copilot, `workflow_dispatch` og cron den 5. hver måned. Leser BigQuery med Workload Identity Federation og en tjenestekonto med bare lesetilgang. Ikke en nais-jobb: resultatet skal uansett bli en PR. |
| Generator | `apps/my-copilot/scripts/generate-offentlige-tall.ts`. All aggregering skjer i SQL (`AVG`, `APPROX_QUANTILES`, `COUNTIF`, `COUNT(DISTINCT)`), så bare ferdige tall per måned forlater BigQuery. Generatoren håndhever terskelen på 20, slår sammen band og familier, og avrunder før skriving. |
| Utdata | `apps/my-copilot/src/data/offentlige-tall.json` (samme navn som i #1520), hele historikken fra januar 2025, nøkkel per måned. Før juni 2026 finnes ikke AI Credits: de månedene har bare `lisenser` og `aktive_brukere`, og feltene for credits mangler (de er ikke 0). |
| Publisering | PR fra jobben. Produkteier godkjenner. Siden leser filen ved bygg. Ingen offentlig API. |
| Sene data | Bare hele måneder. En måned tas med når alle dager har data (`isMonthComplete` i `month-utils.ts`); ellers venter den til neste kjøring. |
| Rettelser | Jobben regner ut de tre siste månedene på nytt. Endrede verdier vises i PR-en, og siden merker dem «rettet» med dato. |

## Sikkerhet

| Tiltak | Hvordan |
|---|---|
| Godkjente felt | `FELT` i `offentlig-statistikk.schema.ts`, med avrunding per felt. Ingen felt for maksimum eller persentiler over p90. |
| Faste bandnavn | `lett`, `middels`, `tung`, og sammenslåingene `lett_middels` og `middels_tung`. Andre navn avvises. |
| Modellandeler | Bare faste familienavn. Ingen antall brukere i filen. Terskelen på 20 brukere sjekkes i generatoren, som har tallet. |
| Andeler | Hele prosent som summerer til 98–102. |
| Test | `offentlig-statistikk.schema.test.ts` feiler på ukjente felt, maks og p95, lister, ukjente band, modellnavn som ikke er en familie, feil familie for kjente modellnavn, andeler som ikke summerer, tall uten kilde eller dato, og tall som ikke er avrundet. |
| Avrunding | Gjøres i generatoren. Testen sjekker den. |
| Ingen persondata ut | Rader per person leses bare inne i BigQuery-spørringen. Generatoren får aldri rader per person. |
| Eksempeldata | Fixturen har `eksempel: true` og oppdiktede tall. Siden skal nekte å vise en fil med `eksempel: true`. |
| Rute | `/innsikt/tall` legges i listen over offentlige ruter i `check-public-routes.mjs`. Testen må sjekke at `/innsikt/tall` er offentlig og at `/innsikt/team` fortsatt krever innlogging. |

## Innsyn og offentleglova

| Situasjon | Svar |
|---|---|
| Krav om tall som står på siden | Henvis til `/innsikt/tall`. Det er nok etter offentleglova § 9. |
| Krav om tall for hele Nav som ikke står der, for eksempel totalkostnad | Behandles som vanlig innsynskrav. Vurder å legge tallet til siden. |
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
| 2 | Generator og SQL, kjørt lokalt mot BigQuery; første fil med ekte tall, godkjent av produkteier | L | 1, avklaring av kildene merket «uklart» |
| 3 | Siden `/innsikt/tall` med nøkkeltall, band, modellandeler og «Om tallene»; rute i `check-public-routes.mjs` | M | 2 (kan bygges mot eksempelfilen) |
| 4 | GitHub Action med WIF og månedlig PR | S | 2 |
| 5 | Tall fra fase «Senere», ett felt per PR | S | 3, godkjenning per felt |
| 6 | `NavCard` for `/innsikt/tall` på `/innsikt`, oppføring i `src/app/sitemap.ts` og i nav-items | S | 3 |

Kommunikasjon orienteres før PR 3 slås sammen.

## Åpne spørsmål

1. Samsvarer brukerne i `billing_user_monthly` med `monthly_active_users`?
2. Hvor finner vi antall brukere per modellfamilie for terskelen på 20?
5. Stemmer navnene i `billing_usage_daily_model` med navnene i `model-pricing.ts`? Mønstrene er laget mot `model-pricing.ts`; generatoren logger ukjente navn i PR 2.
3. Er tersklene 100 og 1 000 AI Credits riktige for bruksbandene?
4. Kildene er ikke sjekket mot BigQuery ennå (gcloud-innloggingen var utløpt 9. oktober 2026). Før PR 2 må vi sjekke:
   - modellnavnene i `billing_usage_daily_model` fra juni 2026 mot mønstrene for familier
   - om `user_metrics` har brukere per modell, så terskelen på 20 kan regnes ut
   - at `v_daily_summary` har `monthly_active_users` tilbake til januar 2025
   - om lisenshistorikk finnes noe sted. Hvis ikke, starter `lisenser` den måneden jobben kjører første gang.
