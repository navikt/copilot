# Tilpasset Copilot-review i Nav: stikkprøve 16. oktober 2026

Nav-repoer har egne review-instrukser, review-skills og generelle Copilot-instrukser. Flere av repoene har også observerte reviews fra GitHubs Copilot-bot. Vi har ikke kontrollert hvilke instrukser boten faktisk brukte, eller om tilpasningene ga bedre funn.

Det tydeligste forbedringspunktet er filplassering: søket fant 16 repoer med `.github/copilot-review-instructions.md`. GitHub dokumenterer ikke automatisk støtte for dette filnavnet. I seks offentlige repoer vi undersøkte, refererte ikke den vanlige `.github/copilot-instructions.md` til review-filen. Instruksene bør flyttes til en dokumentert plassering hvis de er ment for GitHub.com-review.

Ingen repoer, GitHub-innstillinger eller tenantressurser ble endret. Rapporten bruker offentlige kildefiler og PR-metadata. Private søketreff er bare tatt med i antallet treff, ikke navngitt eller analysert.

## Hva undersøkelsen dekker

Vi undersøkte to forskjellige ting:

- **Oppsett:** innhold i utvalgte instruksjonsfiler og skills, med lenker til bestemte commits.
- **Bruk:** reviews fra `copilot-pull-request-reviewer` i fem PR-er fra hvert av seks repoer.

Utvalget er valgt fra GitHub-søk for å finne ulike løsninger. Det er ikke tilfeldig eller representativt for Nav. Et repo er heller ikke det samme som et team: vi har ikke verifisert eierskap eller gruppert repoene etter team.

Søkene gjelder GitHubs kodeindeks, som kan ligge etter siste endring. De fleste søkene nådde grensen på 30 eller 50 treff. Antall treff kan derfor ikke brukes som antall repoer med tilpasset review i Nav. Noen treff var dokumentasjon, distribusjonspakker eller OpenCode-oppsett, ikke aktiv GitHub.com-konfigurasjon.

Organisasjonsomfattende `gh api` ble blokkert av sandboxen. Vi endret ikke sikkerhetsoppsettet. Vanlige `gh search code` og repoavgrensede `gh pr list`/`gh pr view` var tillatt. Innstillinger for automatisk review, organisasjonsinstrukser, MCP og content exclusion er ikke undersøkt.

## Mønstre i instruksjonsfilene

| Repo og kilde                                                                                                                                                       | Hva oppsettet ber om                                                                     | Vurdering                                                                                                                                                                      |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| [k9-sak-web: review](https://github.com/navikt/k9-sak-web/blob/759809bd68ef12d1f86f4c9a2c81d9381cb0ac9c/.github/instructions/review.instructions.md)                | Egne regler for feature toggles, v1/v2-overgang, query keys og tester                    | Konkrete feilscenarier knyttet til repoet. `excludeAgent: "cloud-agent"` lar code review bruke filen, men utelater den fra cloud agent. Appen bruker React/Vite, ikke Next.js. |
| [k9-punsj-frontend](https://github.com/navikt/k9-punsj-frontend/blob/88d9edda18411ec7f083af8aab15243bd60dd57e/.github/instructions/code-review.instructions.md)     | Bare funn med konkret konsekvens og presis fil-/linjereferanse; kontroller hele flyten   | Kort instruks som setter en tydelig terskel for kommentarer. Filen utelukker ikke cloud agent.                                                                                 |
| [su-se-bakover](https://github.com/navikt/su-se-bakover/blob/4a1f38af1f235fe6b6dd1e2a68d0eb9371c1fa33/.github/instructions/code-review.instructions.md)             | Sikkerhet, Nais, workflows, tester, migreringer og feilhåndtering                        | Kotlin-repo med bred sjekkliste. «Always flag» kan gi kommentarer om endringskategorier uten at en feil er påvist. Dette er en risiko i instruksen, ikke målt støy i reviews.  |
| [sif-brukerdialog](https://github.com/navikt/sif-brukerdialog/blob/7b846135fee06fa118069034eab2013d4db205b4/.github/instructions/governance-review.instructions.md) | Sikkerhet, oppgaver og integrasjoner, blant annet `ProsessTask` og navngitte fagsystemer | Kombinerer generell sjekkliste med domenekunnskap. `excludeAgent: "cloud-agent"`. Ber om å fremheve endringer for en kollega, ikke blokkere merge.                             |
| [amt-deltaker-flate](https://github.com/navikt/amt-deltaker-flate/blob/fea4c521a4e052776b45dfcea599d91b85db65cf/.github/instructions/code-review.instructions.md)   | Konkret veiledning om CodeQL og validerte verdier, samt workflow-struktur                | Mer spesifikt enn en generell sikkerhetssjekkliste. Filen utelukker ikke cloud agent.                                                                                          |
| [bidrag-backend](https://github.com/navikt/bidrag-backend/blob/5e6e7c05324b5f71dcd4338938f0626b300bb378/.github/instructions/code-review.instructions.md)           | Hemmeligheter, personopplysninger, autentisering, Nais, workflows og tester              | Kotlin-repo med en hovedsakelig generell sjekkliste. Filen utelukker ikke cloud agent.                                                                                         |
| [pam-stillingsok](https://github.com/navikt/pam-stillingsok/blob/a023d8aa9a4e857a11ed9bfeb3205402d4fdb2dc/.github/copilot-instructions.md)                          | Prosjektkontekst, små endringer og sikkerhetsregler                                      | Next.js 16/React 19, bekreftet i `package.json` ved samme commit. Generelle instrukser som kan gi review-kontekst, ikke en egen review-prosedyre.                              |

Alle de sju undersøkte `.instructions.md`-filene bruker `applyTo: "**"`. Tallet inkluderer også [k9-sak-webs governance-fil](https://github.com/navikt/k9-sak-web/blob/759809bd68ef12d1f86f4c9a2c81d9381cb0ac9c/.github/instructions/governance-review.instructions.md). Filene ligger på en dokumentert plassering, men avgrenser ikke reglene til bestemte deler av koden. Det kan være riktig for felles sikkerhetsregler; stack- og domeneregler bør vurderes mot filene de faktisk gjelder.

## Review-skills finnes, men er ikke bevis på bruk

| Repo og kilde                                                                                                                                        | Hva skillen beskriver                                                                                         | Begrensning                                                                                                                                           |
| ---------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| [tms-min-side](https://github.com/navikt/tms-min-side/blob/fec5d030d1a033343c418c93f7240fa571c0f3fc/.github/skills/code-review/SKILL.md)             | Sammenlign kode med et oppgitt utgangspunkt. Skill dokumenterte standardbrudd fra skjønnsmessige vurderinger. | Beskriver en arbeidsmåte med brukeroppgitt sammenligningspunkt, ikke bevis på at GitHub-review har kjørt den. Repoet bruker Astro/React.              |
| [personopplysninger](https://github.com/navikt/personopplysninger/blob/5ec59cf1f1836d0874eceac99a1fd0ea92747c66/.github/skills/code-review/SKILL.md) | Samme review-opplegg som tms-min-side                                                                         | Viser gjenbruk av samme opplegg. Vi har ikke kontrollert om skillen ble valgt i en review. Repoet bruker Astro/React.                                 |
| [aksel](https://github.com/navikt/aksel/blob/ee50050d27c90c8aa9d5f6ef14783313d0808216/.github/skills/code-review/SKILL.md)                           | API-kompatibilitet for brukerne av designsystemet, React-støtte, tilgjengelighet, tester og releasekrav       | Tydelig tilpasset et delt bibliotek. Krav til tabeller og kommentarformat i skillen er ikke garantier for hvordan GitHub.com-review presenterer funn. |

Et konkret vedlikeholdsfunn: [instruksene i tms-min-side](https://github.com/navikt/tms-min-side/blob/fec5d030d1a033343c418c93f7240fa571c0f3fc/.github/copilot-instructions.md) sier at repoet ikke har et `test`-script. Ved samme commit har [`package.json`](https://github.com/navikt/tms-min-side/blob/fec5d030d1a033343c418c93f7240fa571c0f3fc/package.json) både `test`, `test:watch` og `test:coverage`. Dette bør rettes uavhengig av om instruksene brukes til review eller implementering.

## Filer uten dokumentert automatisk støtte

Søket etter `review` i filer kalt `copilot-review-instructions.md` returnerte **16 filer i 16 repoer: 10 offentlige og 6 private**. Søket hadde en grense på 30. Tallet beskriver disse søketreffene, ikke alle slike filer i Nav.

Vi leste review-filen og `.github/copilot-instructions.md` i seks offentlige repoer:

| Review-fil                                                                                                                                                            | Funn                                                                                                                                                                                                  |
| --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [dp-inntekt](https://github.com/navikt/dp-inntekt/blob/f777d39462c1476ba8b3a69f4dfd5346290ee686/.github/copilot-review-instructions.md)                               | Konkret Ktor/Kotliquery-, sikkerhets- og migreringsveiledning. Review-filen sier poolstørrelse **3–5**, mens de vanlige instruksene sier **3**. Ber også om egne alvorlighetsprefikser i kommentarer. |
| [dp-regel-api](https://github.com/navikt/dp-regel-api/blob/f1e5a0b4c63a41b6439b8f5800d55cf3a2f38800/.github/copilot-review-instructions.md)                           | Ktor, autentisering, Kafka, tester og Nais. Mye overlapper med de vanlige instruksene.                                                                                                                |
| [sokos-utbetalingsportalen](https://github.com/navikt/sokos-utbetalingsportalen/blob/bb7b27e5bff86a3e0ccc4375bd9adb6643a3ffc2/.github/copilot-review-instructions.md) | React/Astro, Aksel, personopplysninger og APM-feilgrenser. Blander konkrete sjekker med generelle råd om komponentoppdeling.                                                                          |
| [sokos-ske-krav](https://github.com/navikt/sokos-ske-krav/blob/5661b3e484961b372f6b8088462119ee35f0d1e0/.github/copilot-review-instructions.md)                       | Én smal regel mot et feilaktig funn om nullbarheten til `ParseException.message`.                                                                                                                     |
| [infotrygd-replikering](https://github.com/navikt/infotrygd-replikering/blob/4752c8e16fa64cdf098b8cffa1899c14d4f571ea/.github/copilot-review-instructions.md)         | Sikkerhet, workflows, Nais, tester og feilhåndtering. Overlapper delvis med de vanlige instruksene.                                                                                                   |
| [familie-baks-ai](https://github.com/navikt/familie-baks-ai/blob/4977abc6448cab9cd6db1187ad9207d734e37479/.github/copilot-review-instructions.md)                     | Review av KI-instrukser. Krever engelske instrukser, mens de vanlige instruksene har et bredt krav om norske dokumenter og artefakter. Språkreglenes virkeområde bør avklares.                        |

Ingen av de seks vanlige instruksjonsfilene refererer eksplisitt til eller gjengir review-filen. Vi har ikke søkt gjennom alle andre filer og verktøy i repoene. Vi kan derfor ikke si at filene aldri blir lest, bare at automatisk innlasting ikke er dokumentert og ikke er koblet opp i de undersøkte hovedfilene.

Et tryggere oppsett er `.github/instructions/review.instructions.md` med passende `applyTo`. Bruk `excludeAgent: "cloud-agent"` hvis instruksene bare skal være tilgjengelige for code review. Dette gjelder instruksene, ikke hvilke kildefiler som vurderes. Se [GitHubs oppsettsveiledning](https://docs.github.com/en/copilot/how-tos/copilot-on-github/customize-copilot/add-custom-instructions/add-repository-instructions) og [oversikten over støttede filer](https://docs.github.com/en/copilot/reference/custom-instructions-support).

## Observerte Copilot-reviews

For hvert repo hentet vi fem PR-er med søket `updated:>=2026-09-16 sort:updated-desc`, uavhengig av om de var åpne eller lukket. For `su-se-bakover` kom resultatene ikke strengt sortert etter oppdateringstid; vi beholdt det returnerte utvalget. Tabellen teller bare reviews med `submittedAt >= 2026-09-16` og forfatter `copilot-pull-request-reviewer`.

| Repo                                                               | PR-numre i utvalget          | PR-er med Copilot-review i perioden | Antall reviews i perioden |
| ------------------------------------------------------------------ | ---------------------------- | ----------------------------------: | ------------------------: |
| [k9-sak-web](https://github.com/navikt/k9-sak-web/pulls)           | 9013, 9126, 9082, 9125, 9123 |                                 5/5 |                         7 |
| [su-se-bakover](https://github.com/navikt/su-se-bakover/pulls)     | 3012, 2923, 3021, 3058, 3057 |                                 3/5 |                         3 |
| [tms-min-side](https://github.com/navikt/tms-min-side/pulls)       | 718, 701, 716, 715, 714      |                                 0/5 |                         0 |
| [bidrag-backend](https://github.com/navikt/bidrag-backend/pulls)   | 267, 282, 252, 281, 280      |                                 5/5 |                        26 |
| [dp-inntekt](https://github.com/navikt/dp-inntekt/pulls)           | 570, 504, 569, 568, 567      |                                 0/5 |                         0 |
| [pam-stillingsok](https://github.com/navikt/pam-stillingsok/pulls) | 1506, 1657, 1684, 1683, 1682 |                                 4/5 |                        23 |
| **Sum**                                                            | **30 PR-er**                 |                           **17/30** |                    **59** |

Teller vi også reviews fra før perioden, hadde 19 av de 30 PR-ene til sammen 61 Copilot-reviews. To eldre reviews faller utenfor periodetallet. Det er PR-ens oppdateringstid som bestemmer utvalget, ikke review-tidspunktet.

Mange reviews er konsentrert på enkelte PR-er: [bidrag-backend#252](https://github.com/navikt/bidrag-backend/pull/252) har 12, og [pam-stillingsok#1657](https://github.com/navikt/pam-stillingsok/pull/1657) har 20. Det viser gjentatte reviews, men sier ikke om de skyldes automatisk review ved push, manuelle forespørsler eller andre utløsere. Det sier heller ikke om kommentarene var nyttige.

Vi leste review-metadata, ikke kommentarene eller instruksjonsfilene ved hver PRs head commit. Dermed er sammenhengen mellom oppsett og observert bruk ikke bekreftet. Null treff i fem PR-er er heller ikke bevis på at teamet aldri bruker Copilot-review.

## Våre egne målinger skiller ikke godt nok

[`apps/copilot-adoption/criteria.go`](../apps/copilot-adoption/criteria.go) teller `.github/copilot-review-instructions.md` som kategorien `copilot_review_instructions`. Den sjekker om filen finnes, ikke om GitHub støtter navnet eller om innholdet brukes.

Samtidig fanges dokumenterte review-instrukser under `.github/instructions/` som generelle instrukser. Dermed kan den særskilte review-kategorien både telle filer uten dokumentert automatisk støtte og overse review-oppsett på riktig plassering. Den må ikke presenteres som «repoer som bruker tilpasset Copilot-review».

Vi har ikke endret skanneren. En senere endring bør skille mellom:

- filer som finnes, med dokumentert eller udokumentert automatisk støtte;
- review-rettet innhold og ruting med `applyTo`/`excludeAgent`;
- observerte reviews fra GitHubs review-bot;
- effekt på kvalitet, som krever en egen evaluering.

## Hva vi bør gjøre først

1. **Sjekk filplasseringen.** Flytt review-regler til en dokumentert instruksjonsfil der dere i dag er avhengige av `.github/copilot-review-instructions.md`. Kontroller andre referanser før den gamle filen fjernes.
2. **Rett motsigelser og utdatert prosjektinfo.** Poolstørrelse i dp-inntekt og testinstruksene i tms-min-side er konkrete steder å begynne.
3. **Bruk lokale feilscenarier.** K9s regler for toggles og v1/v2-overgangen viser hvordan domenekunnskap kan gjøres konkret. For Kotlin kan tilsvarende regler gjelde tilgang til sak og gjentatt behandling. For Next.js kan de gjelde data til nettleseren og mellomlagring mellom brukere.
4. **Krev en konkret konsekvens før en kommentar.** Vurder brede «always flag»-lister opp mot instruksen i k9-punsj-frontend. En endring i sikkerhetskode er ikke i seg selv en feil.
5. **Ikke bruk installasjonsfunn som bruks- eller kvalitetsmål.** Neste audit bør koble et definert PR-utvalg til instruksene ved review-tidspunktet og vurdere funnene manuelt. Teamtilhørighet og tilgang til innstillinger må avklares separat.

## Søk som kan gjentas

Søkene ble kjørt med brukerens eksisterende lesetilgang. Resultatene endrer seg med kodeindeks og tilgang. Begrensningene under er bevisste; dette er ikke en full telling.

```bash
gh search code 'review' --owner navikt --filename copilot-instructions.md \
  --limit 50 --json repository,path,url
gh search code 'review' --owner navikt --extension instructions.md \
  --limit 50 --json repository,path,url
gh search code 'excludeAgent' --owner navikt \
  --limit 30 --json repository,path,url
gh search code 'review' --owner navikt --filename copilot-review-instructions.md \
  --limit 30 --json repository,path,url
gh search code 'code-review' --owner navikt --filename SKILL.md \
  --limit 30 --json repository,path,url
```

For hvert av de seks PR-utvalgene, med `navikt/k9-sak-web` som eksempel:

```bash
gh pr list -R navikt/k9-sak-web --state all \
  --search 'updated:>=2026-09-16 sort:updated-desc' --limit 5 \
  --json number,url,updatedAt,reviewDecision
gh pr view 9013 -R navikt/k9-sak-web --json reviews \
  --jq '.reviews[] | select(.author.login == "copilot-pull-request-reviewer") | {submittedAt, state}'
```

PR-utvalget og datogrensen for review må beholdes hver for seg når tallene beregnes. Tallene over gjelder hentingen 16. oktober, ikke resultatene av et senere søk.
