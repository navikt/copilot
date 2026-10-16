---
title: "Få mer nyttig code review fra Copilot"
date: 2026-10-16
category: copilot
lang: nb
draft: false
excerpt: "Bruk teamets egne feilscenarier til å styre Copilot-review. Konkrete råd for Kotlin og Next.js, og en enkel måte å vurdere om oppsettet hjelper."
tags:
  - code-review
  - custom-instructions
  - github-copilot
---

Begynn med en feil dere vil oppdage før merge. I en Kotlin-backend kan det være manglende tilgangssjekk på en sak eller en operasjon som kjøres to ganger ved nye forsøk. I Next.js kan det være personopplysninger som sendes til nettleseren uten grunn, eller mellomlagrede svar som havner hos feil bruker.

Flere instruksjonsfiler eller flere review-runder er ikke tilstrekkelig i seg selv. Tester, sikkerhetssjekker og en kollega trengs fortsatt.

Artikkelen gjelder **Copilot som reviewer på PR-er på GitHub.com**, ikke review i IDE-en eller Navs egen `code-review`-agent. Rådene i denne artikkelen bygger på oppsett i noen Nav-repoer og GitHubs egen dokumentasjon.

## Kom i gang

1. Velg to–tre feil dere vil fange før merge.
2. Skriv felles regler i `.github/copilot-instructions.md`. Legg Kotlin- og Next.js-regler i egne filer med `applyTo`.
3. Prøv reglene på kjente feil i et testrepo eller en PR som ikke skal merges.
4. Vurder funnene og tiden dere bruker på kommentarene. Juster reglene før dere legger til flere.

## Velg oppgaver som passer for Copilot

Velg oppgaver ut fra hva som best fanger feilen:

- **Formatering og importrekkefølge:** la CI håndtere det hvis dere har sjekker for det.
- **En kjent feil som kan gjenskapes:** lag en regresjonstest. Copilot kan lete etter lignende feil i andre kodeveier.
- **Feil på tvers av kode:** beskriv sammenhengen for Copilot, for eksempel hvem som bruker et API.
- **Faglige avklaringer og sikkerhetsbeslutninger:** be riktig kollega om review. Copilot kan peke på mulige problemer, men ikke godkjenne beslutningen på teamets vegne.

Bytt «kontroller bakoverkompatibilitet» med en konkret regel: «v1 og v2 er i bruk samtidig; ikke fjern v1-kodeveien før overgangen er ferdig». Fjern regelen når overgangen er avsluttet.

Copilot vurderer **endringene i PR-en**, med andre filer som kontekst. Den gjør ikke review av hele repoet. GitHub [utelater også enkelte filtyper](https://docs.github.com/en/copilot/reference/review-excluded-files).

## Begynn med noen få felles regler

Legg til en kort del om code review i `.github/copilot-instructions.md`. Behold eksisterende informasjon om bygging og arkitektur.

**Bruker dere `.github/copilot-review-instructions.md`?** GitHub dokumenterer ikke automatisk innlasting av filen. Flytt reglene til hovedfilen eller `.github/instructions/review.instructions.md` med `applyTo`. Sjekk om andre verktøy bruker den gamle filen før dere fjerner den.

Eksemplene følger repoets konvensjon om engelske instrukser. Tilpass dem til teamets behov.

```markdown
## Code review

- Prioritize incorrect behavior, security issues and data loss.
- Describe the failing scenario, its consequence and the affected code.
- Check callers when shared code changes, not just the changed function.
- When a regression test is missing, describe the case it should cover.
- Distinguish confirmed defects from concerns that need verification.
  A change in security-sensitive code is not by itself a defect.
- Skip formatting and import order when CI already checks them.
- Avoid refactoring suggestions without a concrete defect in this change.
```

Hold filen kort. Lange instrukser kan bli oversett. Krav om et bestemt antall kommentarer gir heller ikke pålitelig styring av reviewen.

Unngå «kommenter alltid endringer i autentisering». Beskriv heller hva som skal sjekkes, og bruk `CODEOWNERS` og review-regler hvis bestemte kolleger alltid skal vurdere slike endringer.

## Egne regler for Kotlin-backend

Legg Kotlin-reglene i `.github/instructions/kotlin-review.instructions.md`:

```markdown
---
applyTo: "**/*.kt"
---

## Kotlin backend review

- Verify access to the requested case or person, not just that the caller
  is authenticated. Use the application's existing authorization rules.
- Flag tokens, national identity numbers or other personal data added to
  logs or error responses.
- Check whether a retry or redelivered message can duplicate a write,
  payment or external call.
- When database writes and message publishing belong to one operation,
  check what happens if only one of them succeeds.
- For API or event schema changes, check compatibility with existing
  consumers, including consumers running the previous version.
```

Beskriv i instruksene hvem som skal ha tilgang til sakene, og hvem som bruker hendelsene deres. Fjern regler som ikke gjelder. Snevr inn `applyTo`, for eksempel til `apps/api/**/*.kt`, hvis repoet inneholder annen Kotlin-kode.

**Tilgangskontroll og behandling av personopplysninger er rød sone: forstå dette grundig.** En kommentar fra Copilot er ikke en sikkerhetsgodkjenning.

Tilpass reglene til rammeverket: ikke krev Spring-annotasjoner i Ktor-kode.

## Egne regler for Next.js

For en Next.js-app med App Router kan `.github/instructions/nextjs-review.instructions.md` se slik ut:

```markdown
---
applyTo: "apps/web/**/*.ts,apps/web/**/*.tsx"
---

## Next.js review

- Check authorization in Server Actions and Route Handlers. Hiding a
  button or checking access in a page is not sufficient.
- Check which fields cross from Server Components to Client Components.
  Flag credentials or personal data the browser does not need.
- When caching changes, check whether one user's data can be returned
  to another user. Check the installed Next.js version and the affected
  cache configuration before reporting a defect.
- Flag server credentials or privileged backend code imported into the
  client bundle.
- For form changes, check labels, keyboard access and whether validation
  errors are associated with the affected fields.
```

Bytt `apps/web/` med mappen deres, for eksempel `src/**/*.ts,src/**/*.tsx`. Ta med JavaScript-filer hvis dere har dem. Sjekk at stiene treffer riktig kode.

Se [Next.js sin veiledning om datasikkerhet](https://nextjs.org/docs/app/guides/data-security). Data hentet på serveren kan havne i nettleseren via props til Client Components.

«Følg Aksel» er lite presist. Be heller om konkrete tilgjengelighetssjekker, som at felt har tilknyttede etiketter.

Instrukser med `applyTo` brukes **sammen med** felles instrukser, ikke som erstatning. Fjern motstridende regler. Se [GitHubs oppsettsveiledning](https://docs.github.com/en/copilot/how-tos/copilot-on-github/customize-copilot/add-custom-instructions/add-repository-instructions).

## Prøv reglene på kjente feil

En review fra Copilot viser ikke at instruksene ble fulgt. Prøv oppsettet selv:

1. Velg tidligere PR-er med kjente feil eller unyttige kommentarer. Gjenskap dem i et testrepo eller PR-er som ikke skal merges. Ikke merge innlagte testfeil.
2. Kjør først med dagens instrukser. Endre deretter én gruppe regler. Bruk samme kode og innstilling for review effort når dere sammenligner.
3. Prøv både filer som treffer `applyTo`, og filer som ikke gjør det. Sjekk at Kotlin-regler ikke havner i frontend-koden.
4. Be om ny review. Noter nyttige funn, uriktige kommentarer, oversette feil, forbruk og tidsbruk. Gjenta forsøket; ett godt resultat beviser ikke at oppsettet er pålitelig.
5. Presiser eller fjern regler som gjentatte ganger gir uriktige kommentarer. Lag tester eller statiske sjekker for feil som kan oppdages automatisk.

Vurder nytten, ikke antall kommentarer. Fravær av funn betyr ikke at en regel er unyttig; prøv den mot en kjent feil først.

[Tilpasning av kommentarformat og PR-oversikt støttes ikke](https://docs.github.com/en/copilot/tutorials/customize-code-review#unsupported-instruction-types). Å be om et bestemt ord i kommentaren er derfor en dårlig test. Instrukser kan heller ikke få review til å følge eksterne lenker; skriv regelen i filen.

### Test på head branch, og les instruksjonsendringer selv

Copilot leser instrukser og skills fra PR-ens **head branch**, branchen med endringene. Nye instrukser kan dermed prøves uten å merges først.

En PR kan derfor svekke sine egne review-instrukser. La en kollega lese slike endringer. GitHub bruker derimot `CODEOWNERS` fra base branch, branchen PR-en skal merges til, for å be om code-owner-review.

## Vedlikehold reglene sammen med koden

Når dere endrer byggoppsett, rammeverk eller arkitektur, sjekk instruksene i samme PR:

- Stemmer testkommandoene med `package.json` eller Gradle-oppsettet?
- Krever to filer ulike verdier eller ulike løsninger for samme oppgave?
- Har dere kopiert en generell sjekkliste til flere steder som nå må vedlikeholdes hver for seg?
- Gjelder regelen fortsatt, eller beskriver den en overgang dere er ferdige med?

Ved gjentatte feilaktige funn, sjekk først om instruksene er utdaterte. Legg eventuelt til en forklaring av akkurat tilfellet, ikke «ignorer alle feil av denne typen».

## Velg når Copilot skal gjøre review

Automatisk review styres av personlige innstillinger og rulesets. **Review new pushes** gir review ved nye push; **Review draft pull requests** tar også med utkast. Personlige innstillinger kan ikke slå av review som et ruleset krever.

Prøv review når PR-en er klar, og etter endringer i logikk, tilgang eller datahåndtering. Uten **Review new pushes** ber dere om ny review manuelt. Hvis rundene bare gjentar tidligere kommentarer, vurder færre kjøringer etter samlede endringer, så lenge rulesets tillater det.

GitHub tilbyr review effort **Lite** og **Balanced**, med Balanced som innebygd standard. Vurder nytten mot forbruket. Se [innstillingene for review](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/request-a-code-review/configure-automatic-review).

### Avklar om Copilot skal kunne godkjenne

Som standard gir Copilot kommentarer. **Copilot approvals** er i public preview og avslått som standard. Hvis funksjonen aktiveres, kan Copilots godkjenning telle mot kravet om godkjenninger før merge. En vurdering i oversiktskommentaren teller ikke i seg selv. En senere push opphever godkjenningen.

Avklar med teamet om Copilots godkjenning skal telle. Copilot gir ikke **Request changes**, og instrukser oppretter ikke mergekrav. Se [GitHubs veiledning](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/request-a-code-review/use-code-review).

## Andre innstillinger og avgrensninger

### `excludeAgent` utelater instrukser, ikke kode

Hold implementeringsinstrukser utenfor review med `excludeAgent`:

```markdown
---
applyTo: "**"
excludeAgent: "code-review"
---

When implementing a feature, update its changelog entry.
```

Code review hopper over **instruksjonsfilen**, ikke kildekoden. `excludeAgent: "cloud-agent"` gjør i stedet instruksene tilgjengelige for review, men ikke cloud agent. En instruks som «ignorer `legacy/**`» sikrer heller ikke at kode utelates.

### Hvis bestemte filer ikke skal leses

Administratorer kan sette opp [content exclusion](https://docs.github.com/en/copilot/concepts/context/content-exclusion) for Business og Enterprise. GitHub oppgir at filene da ikke får review på GitHub.com. Ordningen påvirker flere Copilot-funksjoner; avklar med administratoren før dere bruker den mot review-støy.

Dette er ingen garanti mot datalekkasje. Støtten varierer mellom verktøy, med begrensninger blant annet for symbolske lenker. Et unntak på GitHub.com beskytter ikke nødvendigvis innholdet i Copilot CLI eller IDE-ens agent mode. Hemmeligheter skal uansett ikke ligge i repoet.

### Andre instruksjonsfiler

[GitHub lister også støtte for](https://docs.github.com/en/copilot/reference/custom-instructions-support) `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `REVIEW.md` og organisasjonsinstrukser. Gjenbruk reglene dere har, men sjekk plassering og virkeområde før dere flytter dem mellom filtyper.

### Hvis instruksene ikke ser ut til å virke

Sjekk **Settings → Copilot → Code review → Use custom instructions when reviewing pull requests**, som er på som standard. Dokumentasjonen kaller det et personlig valg, men viser til repoets innstillinger. Innstillingen viser ikke at organisasjonen håndhever en policy.

### Vent med skills og MCP til dere trenger dem

En skill i `.github/skills/<name>/SKILL.md` kan beskrive en spesialisert gjennomgang, men velges ikke nødvendigvis i hver PR. Skills for lokale agenter kan kreve tilpasning til GitHub.com-review.

MCP gir kontekst utenfor repoet. Oppsettet deles mellom cloud agent og code review, og verktøy kan brukes uten godkjenning av hvert kall.

Code review bruker bare verktøy med `annotations.readOnlyHint: true`. Det er metadata, ikke tilgangskontroll: serveren og tilgangen må faktisk hindre skriving. Cloud agent er ikke begrenset til denne merkingen.

Avklar data og tilgang før dere kobler til en server. Ikke eksponer saksdata eller andre skjermede opplysninger bare for å få mer review-kontekst. Se [GitHubs MCP-oppsett](https://docs.github.com/en/copilot/how-tos/copilot-on-github/customize-copilot/configure-mcp-servers).
