# Grunnlag for drøfting: forbruk per team på /innsikt/team

Til tillitsvalgte, som grunnlag for drøfting etter arbeidsmiljøloven § 9-2. Hører til planen i [1511-offentlige-tall.md](1511-offentlige-tall.md).

## Hva siden viser

`/innsikt/team` på ki-utvikling.nav.no viser hvor mye GitHub Copilot hvert team har brukt i en måned:

- forbruk og kostnad per team i AI Credits og dollar
- hvilke typer modeller, funksjoner og programmeringsspråk teamet bruker mest, hvis brukeren velger å vise det

Tallene er per måned. Siden viser ikke tall per dag.

## Hvorfor vi har den

Kostnaden for Copilot steg kraftig i 2026. Teamene trenger å se sitt eget forbruk for å velge riktige modeller og holde kostnaden nede. Siden skal gi oversikt. Den er ikke et mål på produktivitet og rangerer ingen.

## Terskler

- Et team vises bare hvis minst fem personer har brukt Copilot i teamet den måneden.
- Mindre team slås sammen i én samlet gruppe uten navn.
- Tersklene håndheves i API-et, ikke bare i nettleseren.

## Hvem ser den

Alle innloggede Nav-ansatte. Siden er ikke offentlig.

## Hva som aldri vises

- tall per person
- rangering av personer
- lister over hvem som bruker mest eller minst

## Hvor lenge data lagres

Dataene om bruk per person ligger i BigQuery uten sluttdato. Det er et bevisst valg foreløpig, og vi skal vurdere det på nytt.

## Kjent svakhet

Fordelingen på leverandør og modelltype skjuler ikke små grupper. I et team på fem kan én persons bruk skille seg ut. Løsningen er å bruke regelen om minst fem også der. Det er ikke prioritert ennå.

## Det vi ber dere vurdere

1. Er det greit at alle innloggede ser tall per team, eller bør hvert team bare se sine egne tall?
2. Er minst fem personer nok, eller bør terskelen være høyere?
3. Bør svakheten i fordelingen rettes før siden brukes videre?
4. Hvor lenge bør data om bruk per person lagres?
5. Hvordan bør ansatte informeres om siden?
