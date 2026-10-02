# Fra Copilot til agentisk utvikling

## Hvor Nav er i dag, og spørsmål for veien videre

---

## 1. Fra Copilot til agentisk utvikling

**Budskap:** KI-verktøy går fra å foreslå kode til å planlegge, bruke verktøy og utføre hele arbeidsoppgaver.

- Modeller får lengre kontekst og større handlingsrom.
- Agenter kan arbeide parallelt og over lengre tid.
- Utviklingsmiljøet kan bli et kontrollrom for mennesker og agenter.

<!--
Tittelen peker på en endring vi allerede ser i verktøyene, men hvor langt den
endrer arbeidshverdagen i Nav, er fortsatt åpent. GitHub Agent HQ støtter
parallelle agenter med kontrollflater i GitHub. Det er en tilgjengelig
funksjon, ikke dokumentasjon på bedre resultater. Bildet av utviklingsmiljøet
som et kontrollrom er en mulig retning, ikke en prognose vi kan tidfeste.
For oss er Copilot CLI og nav-pilot konkrete utgangspunkt for å prøve ut
mer selvstendig arbeid med kontrollpunkter. Vi skal se på hva som finnes
nå, og hva det gir grunn til å undersøke. Neste lysbilde viser hva Nav
allerede har å bygge på.
-->

---

## 2. Nav har allerede tatt de første stegene

**Budskap:** Vi starter ikke fra null.

- GitHub Copilot og Copilot CLI
- `nav-pilot` med faseinndeling og kontrollpunkter
- Agenter, prompts, instructions, skills og MCP
- Eksplisitte modellvalg, fallback-modeller og egne målinger

Dette er dagens situasjon, ikke en ferdig strategi.

<!--
Verktøyutviklingen på forrige lysbilde møter noe vi allerede har i Nav.
GitHub Copilot, Copilot CLI og nav-pilot gir oss et utgangspunkt for
utprøving, men oversikten sier ikke hvor utbredt eller vellykket bruken
er i hvert team. Faseinndeling og kontrollpunkter beskriver hvordan vi
ønsker at arbeidet skal foregå. Om agentene følger prosessen, må vi
undersøke i faktiske kjøringer. Egne modelltester gir avgrensede
observasjoner, mens leverandørenes brukstall beskriver deres tjenester.
Ingen av delene dokumenterer alene bedre tjenester for Navs brukere.
Dette er heller ikke et forslag om at alle team skal jobbe likt.
Neste spørsmål er derfor hva vi vil få igjen for bruken.
-->

---

## 3. Mer bruk gir ikke automatisk mer verdi

**Budskap:** Individuell produktivitet og organisatoriske resultater er forskjellige størrelser.

- Utviklere kan løse enkelte oppgaver raskere.
- Spart tid i implementeringen kan bli oppveid av mer avklaring, review, testing og drift.
- Vi bør måle resultater for brukerne, kvalitet, gjennomløpstid og samlet arbeidsmengde.

> **Spørsmål 1:** Hvilke resultater for brukerne og Nav skal agentisk AI forbedre, og hvordan måler vi verdien utover spart utviklertid og produsert kode?

<!--
At vi har verktøyene, svarer ikke på om de gir verdi. METRs studie
fra tidlig 2025 fant at erfarne open source-utviklere brukte 19 % lengre
tid med KI på oppgavene de undersøkte. Det er et funn fra en bestemt
gruppe og situasjon, ikke et anslag for Nav i dag. Råestimatene fra
2026 gir heller ikke et pålitelig mål på effekten, fordi utvalget er skjevt.
Leverandørtall om aktivitet kan ikke fylle det kunnskapshullet. Når vi
prøver Copilot CLI og nav-pilot, bør vi følge arbeidet gjennom review
og helt frem til brukeren, også tiden andre må bruke. Det leder til
neste tema: hvilke oppgaver utvikleren selv skal mestre og ha ansvar for.
-->

---

## 4. Utviklerrollen kan bli bredere

**Budskap:** Billigere implementering kan flytte mer av arbeidet mot problemforståelse, produktvalg og kvalitetssikring.

- Rollen kan bevege seg fra programvareutvikler mot produktutvikler.
- Teknisk dybde er fortsatt nødvendig for å vurdere og korrigere agentene.
- Teamene må finne nye måter å bygge erfaring hos juniorer når færre rutineoppgaver gjøres manuelt.

> **Spørsmål 2:** Hvordan bør ansvar, kompetanse og arbeidsdeling endres når utviklere bruker mer tid på problemforståelse, styring og kvalitetssikring enn på selve implementeringen?

<!--
Hvis vi skal måle hele utviklingsløpet, må vi også se på hvem som
tar beslutningene. Anthropic fant i leverandørtelemetri fra rundt
400 000 Claude Code-økter at mennesker tok omtrent 70 % av
planleggingsbeslutningene og 20 % av utførelsesbeslutningene.
Domenekunnskap hang sammen med bedre utfall. Dette er observerte
mønstre hos én leverandør, ikke et eksperiment som viser hvordan
Nav bør organisere teamene. En bredere utviklerrolle er en mulig
følge, ikke en vedtatt rolleendring. I nav-pilot kan skillet mellom
avklaring og utførelse hjelpe oss å undersøke arbeidsdelingen.
Vi må også finne ut hvordan juniorer lærer å vurdere det agenten
gjør. Neste lysbilde handler om ansvaret for koden som blir igjen.
-->

---

## 5. Mer kode kan bli en belastning

**Budskap:** Agenter kan gjøre kode billigere å produsere. Den må fortsatt forstås, eies og driftes.

- Høyere produksjonstakt kan gi større review-kø og mer vedlikehold.
- Nye systemer kan bli opprettet raskere enn gamle blir avviklet.
- Kodefjerning, forenkling og utfasing av gamle løsninger kan bli viktigere utviklingsoppgaver.

> **Spørsmål 3:** Hvordan unngår vi at høyere utviklingstakt gir mer kode, større vedlikeholdsbyrde og flere systemer enn Nav trenger?

<!--
En bredere utviklerrolle betyr også å vurdere om en endring bør
lages i det hele tatt. Her beskriver vi en risiko, ikke en målt
økning i Navs vedlikeholdsbyrde. Hvis Copilot og Copilot CLI gjør
enkelte endringer raskere, kan teamene få flere forslag enn de
rekker å vurdere. Det følger ikke automatisk av høyere aktivitet,
og leverandørtelemetri om produsert kode avgjør ikke om koden trengs.
I nav-pilot kan avklaringen brukes til å spørre om vi heller bør
forenkle eller fjerne noe. Det er et forslag til utprøving, ikke
en dokumentert gevinst. Når agenten får gjøre mer selv, blir neste
spørsmål hvem som kan stoppe eller avvise arbeidet.
-->

---

## 6. Autonomi krever tydelige kontrollflater

**Budskap:** Mer kompetente agenter reduserer ikke behovet for menneskelig ansvar.

- Tilganger må følge oppgaven og være tidsavgrensede.
- Plan, endringer og verktøybruk må kunne observeres og etterprøves.
- Risikoen bør bestemme hvor agenten kan arbeide selvstendig.
- Mennesker må eie produktvalg, sikkerhetsbeslutninger og konsekvensene av endringer.

> **Spørsmål 4:** Hvilke kontrollmekanismer må være på plass før vi gir agenter større handlingsrom, og hvor skal mennesker fortsatt ta beslutningene?

<!--
Muligheten til å avvise unødvendig kode er én del av kontrollen.
Anthropics leverandørtelemetri viser svært lange arbeidssekvenser uten nye
brukerinnspill. Erfarne brukere godkjente mer automatisk, men avbrøt også oftere.
Det dokumenterer bruksmønstre, ikke at høy autonomi er trygt i Nav.
Copilot SDK eksponerer planlegging, verktøy og endringer, med økter,
hooks og OpenTelemetry. Dette gir tekniske muligheter for innsyn
og kontroll, men er ingen garanti for at kontrollene er riktige.
Vi har kontrollpunkter i nav-pilot og må undersøke om de faktisk
følges når Copilot CLI utfører arbeidet. Tilganger og menneskelig
ansvar er krav vi må vurdere, ikke egenskaper vi kan anta.
Kontroll og verifisering bruker også ressurser. Det tar oss til kostnad.
-->

---

## 7. Billigere modeller kan gi større regninger

**Budskap:** Lavere pris per token kan bli oppveid av mer bruk.

- Lengre kontekst, flere samtidige agenter og mer test-time compute øker forbruket.
- Verifisering og feilretting er også en del av kostnaden.
- Kostnad må vurderes per løst oppgave og oppnådd resultat, ikke bare per modellkall.

<!--
Kontrollarbeidet fra forrige lysbilde må med i regnestykket.
I Navs kontrollerte tester matchet GPT-6 Luna GPT-5.6 Luna på
avgrensede testkrav med omtrent 45 % færre kreditter. Det er en
lokal observasjon for disse oppgavene, ikke en prognose for
Navs samlede regning eller en måling av all utviklertid.
Flere parallelle agenter og lengre kjøringer kan spise opp
besparelsen. Med Copilot CLI og nav-pilot bør vi derfor telle
mislykkede forsøk og menneskelig etterarbeid sammen med
modellforbruket. Lavere pris gjør mer bruk mulig, men sier ikke
om mer bruk lønner seg. Neste lysbilde åpner spørsmålet om
hvor modellene bør kjøre, uten å forutsette at lokal drift blir billigere.
-->

---

## 8. Hvor skal inferensen kjøre?

**Budskap:** Fremtidens løsning kan bli en kombinasjon av leverandørtjenester og lokal kapasitet.

- Leverandørtjenester gir tilgang til sterke modeller uten at Nav drifter selve modellen.
- Lokale modeller på utviklermaskinen kan passe for avgrensede oppgaver. Sensitive data krever særskilt vurdering også lokalt.
- Egne eller leide GPU-clustere kan gi mer kontroll, men krever kompetanse, kapasitet og høy utnyttelse.
- Sikkerhet, datakontroll og faktisk totalkostnad må veie tyngre enn ønsket om teknologisk uavhengighet.

> **Spørsmål 5:** Hvordan bør vi balansere kvalitet, kostnad, sikkerhet og kontroll mellom leverandørmodeller, lokal inferens og egen eller leid GPU-kapasitet?

<!--
Kostnad per løst oppgave gjør plasseringen av inferensen til et
praktisk spørsmål. GitHub Copilot og Copilot CLI er dagens
utgangspunkt her, med nav-pilot rundt arbeidsflyten. En kombinasjon
med lokale modeller eller GPU-kapasitet er en mulig fremtidig
løsning, ikke en valgt retning. Vi har ikke lagt frem målinger
som viser at egen drift er billigere eller gir tilstrekkelig
kvalitet for Navs oppgaver. Lokal inferens gjør heller ikke
behandling av sensitive data automatisk trygg. Verktøykall og
logger må vurderes sammen med tilgangene, uansett hvor modellen
kjører. Leverandørenes modellresultater avgjør ikke dette valget
alene. Før vi velger, trenger vi bedre kunnskap om oppgavene.
Det er temaet på neste lysbilde.
-->

---

## 9. Hva bør vi følge med på nå?

**Budskap:** Vi trenger læring og målinger før vi trenger bastante fremtidsvalg.

- Effekt på hele utviklingsløpet, ikke bare kodeproduksjon
- Kvalitet, feilrate, review-belastning og mengden kode vi må eie
- Hvordan ansvar og kompetanse utvikler seg i teamene
- Kostnad per resultat og hvilke modeller oppgavene faktisk trenger
- Hendelser der agenter går utenfor forventet prosess eller handlingsrom

<!--
Valg av kapasitet og modell bør bygge på konkrete oppgaver.
Navs kontrollerte tester viser hvorfor prosess og presisjon må
måles hver for seg. GPT-6 Sol hoppet over et påkrevd intervju
i én av fem kjøringer. Opus 5.5 Medium oppga feil TSX-linjenumre
i to av fem, mens High lyktes i alle fem. Dette er lokale
testobservasjoner, ikke leverandørtelemetri eller anslag for
feilraten i vanlig bruk. Fem kjøringer kan avdekke tydelige
regresjoner, men kan ikke fastslå at en modell er minst like
god som alternativet. For Copilot CLI og nav-pilot er dette
grunner til videre utprøving, ikke en generell modellrangering.
Siste lysbilde samler spørsmålene vi fortsatt må arbeide med.
-->

---

## 10. Fem spørsmål for veien videre

1. Hvilke resultater skal vi forbedre?
2. Hvordan bør roller og kompetanse endres?
3. Hvordan unngår vi unødvendig kode og flere systemer?
4. Hvor går grensene for agentenes handlingsrom?
5. Hvordan balanserer vi kvalitet, kostnad, sikkerhet og kontroll?

Spørsmålene er en ramme for videre utprøving, ikke beslutninger som må tas nå.

<!--
Målingene på forrige lysbilde gir oss noe å undersøke videre,
men avgjør ikke disse fem spørsmålene. Vi har skilt mellom
tilgjengelige verktøy, Navs avgrensede tester og leverandørenes
observasjoner av bruk. Tankene om bredere roller og andre måter
å kjøre modeller på er mulige utviklingsretninger. De er ikke
beslutninger eller sikre prognoser. GitHub Copilot, Copilot CLI
og nav-pilot lar oss utforske spørsmålene med et oppsett vi
allerede har, uten at vi først trenger en komplett strategi.
Bruk avslutningen til samtale om hvilke spørsmål som treffer
teamenes arbeid nå, og hvilke observasjoner som ville endret
vurderingen deres. Målet er å finne noe konkret å lære,
ikke å få enighet om alle fem i dag.
-->
