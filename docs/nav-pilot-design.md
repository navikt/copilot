# nav-pilot – design og beslutningsgrunnlag

## Formål og omfang

`nav-pilot` er et navigasjons- og beslutningslag rundt Copilot CLI som skal gjøre det enklere å bruke riktige arbeidsformer for riktig type oppgave. Systemet finnes for å redusere støy, bevare kontekst og holde arbeidsflyten konsistent når oppgaver varierer i risiko, kompleksitet og behov for spesialisering.

Denne dokumentasjonen beskriver:

- hvorfor `nav-pilot` finnes
- hvilke prinsipper som styrer designet
- hvilke beslutninger som er låst
- hva som ikke skal drive videre utvikling
- hvordan docs, changelog og agent-prompter henger sammen

Målet er å bevare den opprinnelige intensjonen og hindre at senere endringer glir bort fra researchen og erfaringene som formet systemet.

## Bakgrunn: hvorfor `nav-pilot` eksisterer

`nav-pilot` ble laget fordi generiske Copilot-arbeidsflyter ofte er for brede for NAVs behov. Mange oppgaver krever:

- tydelig faseinndeling
- bevisst skille mellom trygg og risikofylt aktivitet
- styring til riktig spesialist ved behov
- forutsigbar start- og synkatferd
- dokumentasjon som bevarer både struktur og intensjon

Uten en fast modell blir det lett:

- for tidlig implementering
- uklare ansvarslinjer
- blanding av utforskning og endring
- dokumentasjon som beskriver hva, men ikke hvorfor

`nav-pilot` er derfor en koordinerende ramme, ikke bare en samling tips.

## Designprinsipper

| Prinsipp | Betydning |
|---|---|
| Fasedisiplin | Arbeid skal deles i tydelige faser med ulike mål og risikonivå. |
| Rød/grønn-sone | Utforskning og endring må ikke blandes ukritisk; sonene styrer trygghet og fokus. |
| Spesialistruting | Oppgaver skal styres mot riktig rolle, ikke presses gjennom én generell flyt. |
| Lav overraskelse | Samme type oppgave skal gi samme type håndtering så langt som mulig. |
| Dokumentert intensjon | Beslutninger skal forklares kort og klart, ikke bare implementeres. |
| Bevarende endring | Endringer skal bygge videre på eksisterende modell, ikke erstatte den uten grunn. |

### Fasedisiplin

Faser skal være eksplisitte. Det skal være lett å se om man er i kartlegging, vurdering, planlegging, endring eller verifisering. Dette er en sentral mekanisme for å unngå at utforskning glir over i handling for tidlig.

### Rød/grønn-sone

Rød sone brukes for usikkerhet, risiko, avklaringer og vurderinger. Grønn sone brukes for utførte, avgrensede og verifiserte endringer. Sonene er et styringsverktøy, ikke pynt.

### Spesialistruting

Når en oppgave krever særskilt kompetanse eller høyere presisjon, skal den routes til riktig spesialist i stedet for å behandles som en generell oppgave. Dette er en del av arkitekturen, ikke en ettertanke.

## Beslutningshistorikk og milepæler

### 1. Fasedisiplin som grunnmur

Den første viktige beslutningen var å gjøre faseinndeling til en eksplisitt del av arbeidsformen. Dette ga bedre kontroll over risiko og reduserte blanding av analyse og endring.

### 2. Rød/grønn-sone som operativ modell

Deretter ble rød/grønn-sone etablert for å skille usikkerhet fra gjennomføring. Dette gjorde det enklere å kommunisere status og begrense utilsiktet drift inn i feil arbeidsmodus.

### 3. Spesialistruting

For å unngå at alt ble behandlet likt, ble ruting til spesialister en del av modellen. Det ga bedre presisjon for sikkerhet, migrering, arkitektur og andre høy-risiko områder.

### 4. Launch/sync-atferd

Arbeidsflyten rundt start og synk ble strammere definert for å unngå uforutsigbar atferd og sikre at riktig kontekst er tilgjengelig før videre arbeid.

### 5. Dokumentstruktur

Docs-strukturen ble viktig for å holde beslutninger, retningslinjer og praksis samlet uten å duplisere innhold. Design, changelog og prompts fikk ulike roller.

### 6. OpenCode-export optimalisering

Eksport til OpenCode ble optimalisert for å gjøre innhold mer nyttig i praksis, uten å endre den underliggende modellen. Fokus var representasjon og brukbarhet, ikke ny logikk.

## Låste beslutninger / invariants

1. **Fasedisiplin skal bestå**
   - Det er ikke valgfritt eller kosmetisk.

2. **Rød/grønn-sone skal være en del av modellen**
   - Sonene brukes for å styre risiko og aktivitet.

3. **Spesialistruting skal beholdes**
   - Ikke alt skal presses inn i én generell flyt.

4. **Dokumentasjon skal speile intensjon**
   - Ikke bare resultater, men også hvorfor beslutningene ble tatt.

5. **Start- og synkatferd skal være bevisst**
   - For å bevare forutsigbarhet og riktig kontekst.

6. **OpenCode-export kan optimaliseres, men ikke endre mening**
   - Form kan forbedres uten at innholdets rolle glir.

## Hva som ikke skal drive

Følgende skal ikke være primær drivkraft for videre utvikling:

- tilfeldig feature creep
- generiske Copilot-tips som ikke passer arbeidsmodellen
- forenkling som fjerner fasedisiplin
- dokumentasjon som blir mer omfattende, men mindre presis
- ruting som erstatter spesialistvurdering med standardflyt
- endringer som gir kortsiktig enkelhet, men svekker sporbarhet eller trygghet

### Viktig presisering

Noen generiske Copilot CLI-tips gjelder **ikke direkte** for `nav-pilot`. `nav-pilot` er ikke bare en CLI-bruksguide; det er en strukturert beslutningsmodell med egne soner, faser og rutingsregler. Tips som er nyttige i en generell Copilot-kontekst må derfor vurderes mot denne modellen før de tas inn.

## Rolle- og rutingsmodell

`nav-pilot` organiserer arbeid etter type og risiko:

- **Generell koordinering**  
  For oppgaver som handler om retning, struktur og arbeidsflyt.

- **Fag-/domene-spesialister**  
  For sikkerhet, migrering, arkitektur og andre høy-risiko områder.

- **Utforskning vs. gjennomføring**  
  Utforskning hører hjemme i rød sone; verifisert endring hører hjemme i grønn sone.

- **Dokumentasjonsarbeid**  
  Skal støtte beslutninger og gjenbruk, ikke erstatte dem.

Ruting skal alltid ha en grunn: hva er oppgaven, hva er risikoen, og hvilken kompetanse trengs?

## Agentroller og delegasjon

Nav-pilot trenger ikke flere brede hovedroller enn det som allerede finnes. Dagens modell dekker behovet:

- `@nav-pilot` er planner/orchestrator og faseeier.
- `@nav-pilot-opus` er smal, høyrisiko leaf-spesialist.
- `code-review` dekker kode- og PR-review.
- `security-champion` dekker sikkerhetsarkitektur og threat modeling.

### Delegasjonsregel

Spesialistagenter skal være **leaf-only**: de skal løse sitt smale delproblem og ikke delegere videre. Det hindrer rolle-glidning, matcher det faktiske behovet og unngår nested delegation-problemer som `Tool 'task' does not exist`.

### Hva som ikke mangler

- **Ny planner-agent:** ikke nødvendig, fordi `@nav-pilot` allerede er koordinator.
- **Ny implementer-agent:** ikke nødvendig nå. Hvis det oppstår smerte her, er en smal delivery-/scaffold-skill bedre enn en ny bred agent.
- **Ny review-agent:** ikke nødvendig. Review-behovet er allerede delt mellom fase 3, `code-review` og `security-champion`.

## Forholdet mellom docs, changelog og prompts

### Docs

Dokumentasjonen er stedet for varige prinsipper, struktur og forklaringer. Den skal gjøre det mulig å forstå systemet uten å lese historikk i etterkant.

### Changelog

Changelog brukes for endringslogg og milepæler. Den skal vise hva som endret seg, når, og gjerne kort hvorfor.

### Prompts

Prompts skal uttrykke ønsket atferd og arbeidsform. De er operative, men skal være i samsvar med designet. Når prompts endres, må de fortsatt respektere fasedisiplin, soner og ruting.

### Samspill

- Docs forklarer **hvorfor**
- Changelog viser **hva som skjedde**
- Prompts styrer **hvordan systemet oppfører seg**

Hvis disse tre divergerer, mister systemet koherens.

## Copilot-CLI-tips kontra nav-pilot-praksis

Noen tips fra den opprinnelige Copilot-bruksanalysen skal brukes som inspirasjon, men ikke kopieres direkte inn i nav-pilot.

| Tips | Vurdering | Hvordan det skal brukes i nav-pilot |
|---|---|---|
| `/tasks` for å følge agenter | Ikke relevant | Nav-pilot bruker fase- og rutingsmodell, ikke task-monitoring som prinsipp. |
| `/fleet` for automatisk parallellitet | Ikke relevant | Parallelitet skal være eksplisitt orkestrering mellom spesialister, ikke en generell modus. |
| `/research` for raskere feilsøking | Delvis relevant | Oversettes til “research først” før plan eller beslutning. |
| `/plan` for multi-phase roadmaps | Relevant | Matcher fase 2 og `$nav-plan` direkte. |
| `/review` + `security-review` | Delvis relevant | Oversettes til fase 3 review + `security-champion`/`$security-review` ved høy risiko. |

## Aktuelle referanser

Behold disse som aktive referanser når nav-pilot videreutvikles:

- `.github/agents/nav-pilot.agent.md` – styrende agentpolicy, fase-maskin og routing
- `.github/agents/nav-pilot-opus.agent.md` – smal, høyrisiko spesialist
- `docs/README.nav-pilot.md` – operativ inngangsside for brukere og bidragsytere
- `docs/nav-pilot-changelog.md` – sporbar historikk over endringer
- `cli/nav-pilot/export.go` – OpenCode-export og instruksjonssplitting
- `cli/nav-pilot/export_test.go` – regresjonsdekning for eksporten
- `https://ki-utvikling.nav.no/nav-pilot` – primær dokumentasjon for brukere

## Praktisk veiledning for framtidige endringer

Når noe skal endres, bruk denne sjekken:

1. Bryter endringen fasedisiplin eller rød/grønn-sone?
2. Svekkes spesialistruting eller tydelig ansvar?
3. Blir dokumentasjonen mer generell, men mindre sann?
4. Endres launch/sync-atferd på en måte som skaper overraskelse?
5. Er dette en forbedring av form, eller en reell endring av modell?

### Anbefalt praksis

- Behold begrepsapparatet stabilt
- Legg til nye regler bare når de løser et reelt behov
- Dokumenter begrunnelse samtidig med endringen
- Oppdater docs, changelog og prompts samlet når de påvirkes
- Unngå å “rydde” bort begreper som fortsatt bærer designets logikk

## Kort oppsummering

`nav-pilot` er bygget for å gjøre arbeidsflyt mer presis, trygg og sammenhengende gjennom fasedisiplin, rød/grønn-sone og spesialistruting. Designet skal være stabilt nok til å bevare læring, men fleksibelt nok til å forbedres uten å miste retning. Hovedregelen er enkel: endre gjerne formen, men ikke det som gjør systemet begripelig og styrbart.
