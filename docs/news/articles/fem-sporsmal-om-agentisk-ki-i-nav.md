---
title: "Slik utforsker vi agentisk KI i produktutviklingen"
date: 2026-09-24
author: starefossen
category: praksis
excerpt: "KI-utvikling jobber med agentisk KI for produktutvikling i Nav. Her er oppsettet vi bygger, hva vi tester nå, og fem spørsmål som styrer arbeidet videre."
tags:
  - agentisk-ai
  - nav-pilot
  - produktutvikling
  - kompetanse
---

Denne artikkelen handler om agentisk KI i produktutviklingen, som er ansvarsområdet til KI-utvikling og [`navikt/copilot`](https://github.com/navikt/copilot). Andre miljøer jobber med KI i tjenester, saksbehandling og resten av organisasjonen. Vi svarer ikke for hele Navs KI-satsing.

«Hva er planen?» Det ærlige svaret er at vi ikke har en ferdig plan. Vi har derimot et konkret oppsett for å prøve, måle og justere. Målet er å finne ut hvilke oppgaver agentene kan hjelpe med, hvilke kontrollmekanismer vi trenger, og hva bruken faktisk koster.

Vi har ikke fått mer penger til dette i budsjettet for 2026, men har meldt inn et større behov for 2027. Forsøkene må gi oss grunnlag for å prioritere det vi får.

## Vi bygger et felles oppsett, ikke en autonom utvikler

[`nav-pilot`](https://ki-utvikling.nav.no/nav-pilot) samler Nav-kontekst for GitHub Copilot og andre støttede klienter. Utvikleren får agenter, instructions, skills, prompts og kontrollpunkter uten å sette sammen alt selv. Team kan installere oppsettet i repoet og dele det, eller bruke det personlig på tvers av repoer.

Målet er at utviklere skal kunne begynne med oppgaven, ikke med å lære hvordan modell, kontekst, agent-harness og verktøy henger sammen. De som trenger å tilpasse eller forstå oppsettet i dybden, skal fortsatt kunne gjøre det.

Oppsettet kombinerer to typer styring:

- **Faste kontroller** som formattering, lint, typesjekk, tester, CI og sandbox-regler.
- **Kontekstavhengig veiledning** gjennom agenter og instructions for blant annet sikkerhet, universell utforming, Aksel, Kafka og kodegjennomgang.

`nav-pilot` deler også arbeidet i faser. Agenten skal forstå oppgaven og avklare risiko før den begynner å endre kode. Et kontrollpunkt er likevel bare nyttig hvis modellen følger det. Derfor tester vi at agentene faktisk stopper, spør og validerer, ikke bare at instruksjonen finnes i en fil.

## Dette jobber vi med nå

Arbeidet i `navikt/copilot` er samlet rundt fire områder.

### Gjøre oppsettet enkelt å installere og dele

`nav-pilot` installerer og oppdaterer tilpasningene i brukerscope eller i teamets repo. En sync-workflow kan oppdage at oppsettet er utdatert og åpne en pull request med endringene.

Vi bygger også støtte for agentpakker. Et team kan da distribuere egne agenter, skills og instructions uten å forke `navikt/copilot`. Pakka beskriver hvilke klienter den støtter, hvilket innhold den har, og hvilke MCP-servere den forventer.

### Begrense hva agentene får gjøre

Agentene kjører verktøy, leser filer og kan endre kode. Det gjør tilgangsstyring til en del av produktet, ikke et tillegg vi kan ta senere.

Vi bruker sandboxing rundt Copilot CLI og skiller mellom tekst modellen leser og kode som senere kan kjøre på utviklerens maskin. En agentpakke kan for eksempel inneholde hooks og extensions. `nav-pilot` lar ikke en prosess inne i sandboxen installere slike kjørbare artefakter utenfor sandboxen.

MCP-servere gir agentene tilgang til eksterne verktøy og data. De styres derfor gjennom [Navs MCP-register](https://mcp-registry.nav.no). `nav-pilot` kan validere at en agentpakke viser til en server registeret faktisk publiserer, men slår ikke serveren på for brukeren.

### Teste modeller på Nav-oppgaver

Vi velger modell ut fra oppgaven, ikke ut fra leverandør eller lanseringsdato. En research-agent trenger ikke samme modell som en sikkerhetsagent. En fast scaffold-prompt trenger ikke modellen vi bruker til høyrisiko planlegging.

[Våre kontrollerte tester](/nyheter/gpt-6-sol-luna-opus-5-5) viser hvorfor vi må teste konkret. GPT-6 Luna løste de samme avgrensede kravene som GPT-5.6 Luna med omtrent 45 % færre kreditter. GPT-6 Sol hoppet over et påkrevd intervju i én av fem kjøringer. Opus 5.5 Medium oppga feil TSX-linjenumre i to av fem kodegjennomganger, mens High lyktes i alle fem.

Fem kjøringer kan avdekke tydelige regresjoner, men ikke bevise at en modell er minst like god som en annen. Vi bruker derfor eksplisitte modellvalg, kontrollert utrulling og fallback-modeller. Vi måler også bruk, varighet og resultat fremfor å anta at nyeste modell er best eller billigst.

### Finne ut om oppsettet hjelper teamene

Vi samler bruksdata og skanner repoer for å se hvilke tilpasninger som er tatt i bruk. Det forteller oss om noe brukes, men ikke om det gir bedre produkter.

Den viktigste tilbakemeldingen kommer fortsatt fra teamene. Vi trenger eksempler på oppgaver som gikk bedre, kontrollpunkter som ikke ble fulgt, feil som slapp gjennom, og arbeid som agenten gjorde unødvendig komplisert. Et oppsett som består en kontrollert test, kan fremdeles skape friksjon i en vanlig arbeidsdag.

## Teamet må eie konteksten og resultatet

Dagens KI-kodeverktøy er først og fremst laget for enkeltpersoner. Nav utvikler produkter i tverrfaglige team. Vi ønsker ikke en arbeidsform der «min agent» og «din agent» utveksler arbeid gjennom issues og pull requests mens menneskene mister den felles forståelsen.

Team kan teste par-prompting, felles økter med KI-chat på storskjerm og bruk av agent under feilsøking. Da kan flere vurdere antakelsene og forslagene mens arbeidet skjer.

Agentene trenger også den skjulte konteksten i systemutviklingen: hvorfor en tilsynelatende unødvendig sjekk finnes, hvilken avhengighet som ikke vises i kodebasen, eller hvilket alternativ teamet allerede har prøvd.

Prompts og agentinstruksjoner inneholder ofte slike begrunnelser. De bør tas vare på i kode, dokumentasjon eller en ADR. Pull requesten bør forklare hvorfor endringen trengs, hvilke alternativer som ble vurdert, og hvordan teamet kom frem til løsningen. En liste over endrede filer tilfører mindre.

Teamet må også kunne avvise kode agenten foreslår. Når kode blir billigere å produsere, kan nye løsninger komme raskere enn vi avvikler gamle. Vi vil derfor undersøke hvordan agentene kan hjelpe oss å forenkle, fjerne kode og avslutte gamle løsninger, ikke bare bygge mer.

## Fem spørsmål som styrer arbeidet videre

Vi bruker fem spørsmål for å vurdere forsøkene og velge hva vi skal jobbe med videre:

1. **Gir oppsettet bedre resultater?** Vi må måle hele utviklingsløpet, ikke bare tiden frem til en pull request.
2. **Følger agentene kontrollene?** Tester, faseporter og tilgangsregler må virke i faktiske oppgaver.
3. **Beholder teamet forståelsen?** KI-bruken skal støtte samarbeid og kompetansebygging, også for juniorer.
4. **Hjelper agentene oss å eie mindre kode?** Forenkling og utfasing må telle som resultater.
5. **Bruker vi riktig modell til riktig pris?** Kostnaden må inkludere nye forsøk, verifisering og menneskelig etterarbeid.

Dette er spørsmål for ansvarsområdet vårt: agentisk KI i produktutviklingen. Svarene kommer ikke fra én benchmark eller én modellrelease. De kommer fra kontrollerte tester og erfaringene til team som bruker oppsettet på reelle oppgaver.

Nav-ansatte kan ta med erfaringer til Åpent KI-kontor onsdager kl. 09.00. Vi trenger særlig å høre om det som ikke fungerer.

## Les mer

- [Kom i gang med nav-pilot](https://ki-utvikling.nav.no/nav-pilot)
- [Kildekoden og dokumentasjonen i navikt/copilot](https://github.com/navikt/copilot)
- [Navs MCP-register](https://mcp-registry.nav.no)
- [Harness engineering](https://martinfowler.com/articles/harness-engineering.html) (Martin Fowler, 2026)
