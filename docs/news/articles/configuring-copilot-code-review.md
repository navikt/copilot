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

Beskriv feilen i instruksene, og prøv om Copilot finner den. Flere instruksjonsfiler eller flere review-runder er ikke i seg selv et bedre oppsett. Tester, sikkerhetssjekker og en kollega trengs fortsatt.

Artikkelen gjelder **Copilot som reviewer på PR-er på GitHub.com**. Code review i IDE-en, Copilot Chat og Navs egen `code-review`-agent har andre tilpasningsmuligheter. Rådene bygger på oppsett i noen Nav-repoer og GitHubs dokumentasjon per **16. oktober 2026**. Vi har ikke målt effekten av tilpasningene.

## Kom i gang

1. Velg to–tre feil dere vil fange før merge.
2. Skriv felles regler i `.github/copilot-instructions.md`. Legg Kotlin- og Next.js-regler i egne filer med `applyTo`.
3. Prøv reglene på kjente feil i et testrepo eller en PR som ikke skal merges.
4. Vurder funnene og tiden dere bruker på kommentarene. Juster reglene før dere legger til flere.

## Velg oppgaver som passer for Copilot

Bruk tidligere feil og tilbakemeldinger fra review til å velge noen få regler. Spør først om feilen heller kan fanges av en test eller en statisk sjekk.

- **Formatering og importrekkefølge:** la CI håndtere det hvis dere har sjekker for det.
- **En kjent feil som kan gjenskapes:** lag en regresjonstest. Copilot kan lete etter lignende feil i andre kodeveier.
- **Regler som krever forståelse av flere deler av koden:** beskriv sammenhengen for Copilot. Det kan være hvem som bruker et API, eller hva som må fungere mens to versjoner kjører samtidig.
- **Faglige avklaringer og sikkerhetsbeslutninger:** be riktig kollega om review. Copilot kan peke på mulige problemer, men ikke godkjenne beslutningen på teamets vegne.

Bytt for eksempel ut «kontroller bakoverkompatibilitet» med en konkret regel: «v1 og v2 er i bruk samtidig; endringen må ikke fjerne v1-kodeveien før overgangen er ferdig». Tilpass regelen til løsningen deres, og fjern den når overgangen er avsluttet.

Copilot vurderer **endringene i PR-en**. Den kan lese andre filer for å forstå koden, men det betyr ikke at hele repoet får en code review. GitHub har også en [liste over filer som utelates fra review](https://docs.github.com/en/copilot/reference/review-excluded-files), blant annet mange genererte filer og avhengighetsfiler.

## Begynn med noen få felles regler

Legg reglene i `.github/copilot-instructions.md`. Finnes filen allerede, legger du til en kort del om code review. Behold informasjonen om bygging og arkitektur som ligger der fra før.

**Har dere en fil som heter `.github/copilot-review-instructions.md`?** Vi fant slike filer i flere Nav-repoer, men GitHub dokumenterer ikke at de lastes automatisk. Legg review-reglene i hovedfilen eller i `.github/instructions/review.instructions.md` med `applyTo`. Sjekk om andre verktøy bruker den gamle filen før dere fjerner den.

Eksemplene under er på engelsk, slik instruksjonsfilene i dette repoet er. Tilpass innholdet til det teamet faktisk trenger.

```markdown
## Code review

- Prioritize incorrect behavior, security issues and data loss.
- Describe a concrete failing scenario and its impact. Point to the code.
- Check callers when shared code changes, not just the changed function.
- When a regression test is missing, describe the case it should cover.
- Report a defect only when you can explain its consequence. A change in
  security-sensitive code is not by itself a defect.
- If you suspect a problem but cannot confirm it, state what needs to be
  verified. Do not present the suspicion as a confirmed defect.
- Skip formatting and import order when CI already checks them.
- Avoid refactoring suggestions without a concrete defect in this change.
```

Hold filen kort. GitHub advarer om at lange instrukser kan bli oversett. Et krav som «legg igjen nøyaktig tre kommentarer» gir heller ingen pålitelig kontroll over resultatet.

Unngå «kommenter alltid endringer i autentisering». Beskriv heller hva som skal sjekkes, og bruk `CODEOWNERS` og review-regler hvis bestemte kolleger alltid skal vurdere slike endringer.

## Egne regler for Kotlin-backend

Regler for en backend trenger ikke gjelde for frontend-koden. Bruk en egen fil, for eksempel `.github/instructions/kotlin-review.instructions.md`:

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

Beskriv teamets egne regler der det trengs: Hvem skal kunne lese en sak? Hvilket lag håndhever det? Hvilke mottakere bruker hendelsene deres?

Ta bare med reglene som gjelder hos dere. Har tjenesten ingen meldingsbehandling, trenger den heller ikke en sjekkliste for gjenleverte meldinger. Snevr inn `applyTo`, for eksempel til `apps/api/**/*.kt`, hvis repoet også inneholder annen Kotlin-kode.

**Tilgangskontroll og behandling av personopplysninger er rød sone: forstå dette grundig.** En kommentar fra Copilot er ikke en sikkerhetsgodkjenning.

Bruker dere både Ktor og Spring Boot, bør instruksene følge løsningene dere har. Ikke be Copilot kreve Spring-annotasjoner i Ktor-kode eller foreslå et nytt databaselag bare fordi det er vanlig et annet sted.

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

Bytt `apps/web/` med mappen deres. Ligger koden rett i `src/`, bruker du for eksempel `src/**/*.ts,src/**/*.tsx`. Ta med relevante JavaScript-filer også hvis dere har dem. En sti som ikke treffer noen filer, bidrar ikke med instrukser.

Reglene om tilgang og data følger [Next.js sin veiledning om datasikkerhet](https://nextjs.org/docs/app/guides/data-security). Kontroller særlig hvilke data som sendes som props til Client Components. Selv om data hentes på serveren, kan de havne i nettleseren via props.

Be Copilot finne konkrete tilgjengelighetsfeil, som et felt uten tilknyttet etikett. En generell instruks om å «følge Aksel» sier lite om hva den skal sjekke.

Instrukser med `applyTo` brukes **sammen med** de felles instruksene. Ikke regn med at en lokal instruks opphever en felles regel. Fjern motstridende regler. Se [GitHubs oppsett for instruksjonsfiler](https://docs.github.com/en/copilot/how-tos/copilot-on-github/customize-copilot/add-custom-instructions/add-repository-instructions).

## Prøv reglene på kjente feil

En instruksjonsfil og en review fra Copilot-boten viser ikke at instruksene ble fulgt eller at reviewen var nyttig. Prøv oppsettet selv:

1. Velg noen tidligere PR-er med en kjent feil, og noen der Copilot ga unyttige kommentarer. Bruk et testrepo eller PR-er som ikke skal merges. Ikke merge feil dere har lagt inn for å teste.
2. Kjør først med dagens instrukser. Endre deretter én gruppe regler. Bruk samme kode og innstilling for review effort når dere sammenligner.
3. Prøv både filer som treffer `applyTo`, og filer som ikke gjør det. Sjekk at Kotlin-regler ikke havner i frontend-koden.
4. Be om ny review manuelt hvis review ved nye push ikke er aktivert. Noter nyttige funn, uriktige kommentarer, kjente feil som ikke ble funnet, forbruk og tiden dere brukte på å avklare kommentarene. Gjenta forsøket; ett godt resultat beviser ikke at oppsettet er pålitelig.
5. Presiser eller fjern regler som gjentatte ganger gir uriktige kommentarer. Lag tester eller statiske sjekker for feil som kan oppdages automatisk.

Vurder nytten ut fra hvilke feil dere oppdaget og hvor mye tid dere brukte på uriktige funn, ikke antall kommentarer. Fravær av funn viser ikke at en regel er unyttig. Prøv den mot et kjent feilscenario før dere konkluderer.

Ikke test oppsettet bare ved å be Copilot skrive et bestemt ord i en kommentar. [GitHubs veiledning om tilpasning](https://docs.github.com/en/copilot/tutorials/customize-code-review#unsupported-instruction-types) sier at endringer i kommentarformat og PR-oversikt ikke støttes. Instrukser kan heller ikke få review til å følge eksterne lenker. Skriv selve regelen i instruksjonsfilen.

### Test på head branch, og les instruksjonsendringer selv

Copilot leser repoets instrukser og skills fra PR-ens **head branch**, altså branchen med endringene. Dere kan dermed prøve nye instrukser i samme PR uten å merge dem først.

Det betyr også at en PR kan svekke instruksene den selv vurderes etter. Les endringer i instruksjonsfiler selv. Ikke la Copilot vurdere endringer i sine egne review-instrukser alene. For å be om code-owner-review bruker GitHub derimot `CODEOWNERS` fra base branch, altså branchen PR-en skal merges til.

## Vedlikehold reglene sammen med koden

Når dere endrer byggoppsett, rammeverk eller arkitektur, sjekk instruksene i samme PR:

- Stemmer testkommandoene med `package.json` eller Gradle-oppsettet?
- Krever to filer ulike verdier eller ulike løsninger for samme oppgave?
- Har dere kopiert en generell sjekkliste til flere steder som nå må vedlikeholdes hver for seg?
- Gjelder regelen fortsatt, eller beskriver den en overgang dere er ferdige med?

Hvis Copilot gjentar et feilaktig funn, kontroller først om utdaterte instrukser er årsaken. Er koden riktig, kan dere legge til en smal forklaring av akkurat det tilfellet. Ikke løs ett feilaktig funn med «ignorer alle feil av denne typen»; da kan reelle feil bli oversett.

## Velg når Copilot skal gjøre review

Automatisk review kan komme fra personlige Copilot-innstillinger eller rulesets. **Review new pushes** gir ny review ved nye push, mens **Review draft pull requests** også tar med utkast. Flere innstillinger kan utløse review av samme PR, men GitHub legger igjen én review. Personlige innstillinger kan ikke slå av review som et ruleset krever.

Prøv review når PR-en er klar, og ny review etter endringer i logikk, tilgang eller datahåndtering. Uten **Review new pushes** må dere be om ny review manuelt. Copilot kan gjenta kommentarer som allerede er løst eller har fått negativ tilbakemelding.

Se på hva de ekstra rundene gir før dere slår på review ved hver push. Finner de nye feil, eller gjentar de tidligere kommentarer etter små rettelser? Ved mye gjentakelse kan dere be om ny review etter en samlet endring, så lenge rulesets tillater det.

Dere kan også velge hvor grundig review skal være. GitHub tilbyr **Lite** og **Balanced**, med Balanced som innebygd standard. Mer innsats er ingen garanti for at feil blir funnet. Se [innstillingene for review](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/request-a-code-review/configure-automatic-review).

### Avklar om Copilot skal kunne godkjenne

Som standard gir Copilot kommentarer. **Copilot approvals** er i public preview og er avslått som standard. Hvis funksjonen aktiveres i de aktuelle innstillingene, kan Copilots godkjenning telle med i kravet til godkjenninger før merge. En vurdering i oversiktskommentaren er ikke i seg selv en slik godkjenning. En senere push opphever godkjenningen.

Avklar med teamet om en automatisk godkjenning skal telle. Copilot gir ikke review av typen **Request changes**, og en instruks om å «blokkere merge til alle funn er rettet» oppretter ingen regel i GitHub. Se [GitHubs veiledning om code review](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/request-a-code-review/use-code-review).

## Andre innstillinger og avgrensninger

### `excludeAgent` utelater instrukser, ikke kode

Noen instrukser er nyttige når en agent skriver kode, men ikke når Copilot gjør review. Da kan dere bruke dette i en egen instruksjonsfil:

```markdown
---
applyTo: "**"
excludeAgent: "code-review"
---

When implementing a feature, update its changelog entry.
```

Code review hopper over **denne instruksjonsfilen**. Dette utelater ikke kildekoden fra review. Med `excludeAgent: "cloud-agent"` holder dere i stedet instruksjonsfilen unna cloud agent, mens code review fortsatt kan bruke den.

En setning som «ignorer `legacy/**`» er også bare en instruks. Den sikrer ikke at filene utelates.

### Hvis bestemte filer ikke skal leses

Administratorer kan sette opp [content exclusion](https://docs.github.com/en/copilot/concepts/context/content-exclusion) for Copilot Business og Enterprise. GitHub oppgir at filene som omfattes, ikke får Copilot-review på GitHub.com. Dette påvirker også andre Copilot-funksjoner som støtter ordningen. Avklar derfor med administratoren før dere bruker det for å redusere antall review-kommentarer.

Content exclusion er ikke en garanti mot datalekkasje. Støtten varierer mellom verktøyene, og GitHub beskriver begrensninger, blant annet for symbolske lenker. Ikke regn med at et unntak på GitHub.com beskytter det samme innholdet i Copilot CLI eller IDE-ens agent mode. Hemmeligheter skal ikke ligge i repoet, med eller uten slike unntak.

### Andre instruksjonsfiler

[GitHubs oversikt](https://docs.github.com/en/copilot/reference/custom-instructions-support) lister også `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `REVIEW.md` og organisasjonsinstrukser for code review på GitHub.com. Gjenbruk reglene dere har. Plassering og virkeområde er ikke beskrevet likt for alle filnavn; oversikten nevner for eksempel `REVIEW.md`, men den generelle oppsettsveiledningen beskriver ikke filen.

### Hvis instruksene ikke ser ut til å virke

Sjekk **Settings → Copilot → Code review → Use custom instructions when reviewing pull requests**. GitHub oppgir at valget er på som standard. Dokumentasjonen kaller det et personlig valg, men viser til repoets innstillinger. Ikke behandle det som en håndhevet organisasjonspolicy.

### Vent med skills og MCP til dere trenger dem

Begynn med instruksjonsfilene. En skill i `.github/skills/<name>/SKILL.md` kan beskrive en mer spesialisert gjennomgang. Copilot velger relevante skills; en sjekkliste der blir ikke nødvendigvis brukt i hver PR. En skill laget for en lokal agent kan dessuten forutsette at brukeren oppgir hva som skal sammenlignes. Ikke regn med at den virker uendret i GitHub.com-review.

MCP kan gi tilgang til opplysninger som ikke finnes i repoet. GitHub deler repoets MCP-oppsett mellom cloud agent og code review, så en ny server kan gi verktøy til begge. Verktøyene kan brukes uten at noen blir spurt om å godkjenne hvert kall.

Code review bruker bare verktøy merket med `annotations.readOnlyHint: true`. Dette er metadata, ikke tilgangskontroll. Serveren og tilgangen den bruker, må faktisk hindre skriving. Cloud agent er ikke begrenset til verktøy med denne merkingen.

Avklar hvilke data som blir tilgjengelige, og hvem som får tilgang, før dere kobler til en server. Ikke gi Copilot tilgang til saksdata eller andre skjermede opplysninger bare for å få mer kontekst i en review. Se [GitHubs MCP-oppsett](https://docs.github.com/en/copilot/how-tos/copilot-on-github/customize-copilot/configure-mcp-servers).
