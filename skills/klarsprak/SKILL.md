---
name: klarsprak
description: "Redigér og kvalitetssikre norsk tekst: klarspråk, fjerning av AI-markører, anglisismer, fagtermer og nynorsk/svensk-innblanding. Bruk denne skillen hver gang du skriver, redigerer eller språkvasker norsk tekst — README-er, ADR-er, UI-tekst, blogginnlegg, dokumentasjon, e-poster, commit-meldinger og PR-beskrivelser — også når brukeren ikke sier «klarspråk» eksplisitt, men bare ber om hjelp med norsk tekst."
license: "MIT"
---

# Klarspråk

Redigér norsk bokmål så teksten blir klar, korrekt og menneskelig. Målgruppa er typisk utviklere, driftere og arkitekter, men prinsippene gjelder all norsk sakprosa.

Språkloven pålegger offentlige organer å bruke klart, korrekt språk tilpassa mottakerne. Følg Språkrådets klarspråk-prinsipper og ISO 24495-1.

## Arbeidsflyt

1. Les hele teksten først.
2. Skann etter mønstrene under: AI-markører, substantivsyke, anglisismer, feiloversatte fagtermer, nynorsk/svensk-innblanding, dårlig struktur.
3. Skriv om. Bevar meningen og faglig innhold: endre bare språk, form og struktur.
4. Selvrevisjon: «Hva avslører at dette er KI-generert?» Fiks det som gjenstår.
5. Tilpass til teksttypen (ADR, README, UI-tekst, blogg).

## Klarspråk-prinsipper

### Det viktigste først

Start med konklusjonen eller det leseren trenger å vite. Bakgrunn kommer etterpå.

```
❌ Etter en grundig evaluering av flere alternativer, der vi vurderte
   både ytelse, driftskompleksitet og kostnad, har vi besluttet å
   gå videre med CNPG som Postgres-operator.

✅ Vi bruker CNPG som Postgres-operator. Den gir oss automatisk
   failover, backup og oppgradering uten nedetid.
```

### Skriv for leseren

Tenk: Hva trenger leseren å gjøre etter å ha lest dette? Kutt alt som ikke hjelper dem. Ikke definer ting målgruppa allerede vet.

### Unngå substantivsyke

Bruk verb, ikke substantiv laget av verb. Typisk mønster: -ing + av.

```
❌ Vi foretar en gjennomgang av implementasjonen.
✅ Vi gjennomgår implementasjonen.

❌ Gjennomføring av migrering til ny plattform.
✅ Vi migrerer til ny plattform.
```

### Kort over langt

- Kort setning over lang. Én idé per setning — må leseren lese setningen to ganger, del den i to.
- Vanlig ord over fint ord: «bruke» ikke «benytte», «hjelpe» ikke «fasilitere», «mange» ikke «tallrike», «hvis» ikke «i det tilfellet at».
- Aktiv form over passiv: «vi bruker» ikke «det benyttes». Fang «blir/ble + partisipp» og navngi aktøren: «spørringer valideres» → «kompilatoren validerer spørringer». Passiv er greit bare når aktøren er ukjent eller uvesentlig.
- Konkret over abstrakt: «vi bygger nytt image» ikke «det kreves en tilpasning av image-artefaktet».
- Kutt fyllord: «i bunn og grunn», «i stor grad», «på mange måter», «det er viktig å merke seg at».
- Kutt adverb, eller bruk et sterkere verb: «kjører raskt» → «er rask» eller tallet. «forbedrer betydelig» → den målte forskjellen.
- Si hva ting gjør, ikke hvordan det føles. «SQL du kan lese» og «databasen er alltid nær» navngir en følelse. Skriv heller mekanismen eller tallet: «`.toSQL()` returnerer strengen som sendes til databasen». Kan setningen stå uendret i dokumentasjonen til et helt annet prosjekt, sier den ingenting om dette — kutt den.

### Struktur

- Korte avsnitt (2–4 setninger)
- Mellomtitler som sier hva tekstdelen handler om
- Kulepunkter for lister, ikke lange komma-oppramsinger
- Bare første ord og egennavn med stor bokstav i overskrifter (ikke engelsk stil)

## AI-markører

Erstatt eller fjern mønstre som avslører KI-generert tekst.

### Svulstige ord og uttrykk

| AI-markør | Gjør i stedet |
|-----------|---------------|
| «banebrytende», «revolusjonerende», «innovativ» | Bruk konkrete beskrivelser |
| «representerer et betydelig skritt fremover» | Si hva det faktisk gjør |
| «robust», «helhetlig», «sømløs», «holistisk» | Skriv om eller dropp |
| «spiller en avgjørende rolle» | Gå rett på sak |
| «dette understreker behovet for» | Si behovet direkte |
| «har tatt verden med storm» | Dropp helt |
| «synergi», «paradigmeskifte» | Si konkret hva som skjer |
| «effektivisere prosessen» | Si hvilken prosess og hvordan |
| «sette brukeren i sentrum» | Forklar hva dere faktisk gjør for brukeren |
| «digital transformasjon» | Si hva som endres konkret |
| «muliggjør», «tilrettelegger for» | Si hva som skjer |
| «et vitnesbyrd om», «et bevis på» (testament to) | Si hva som skjedde |
| «pulserende», «idyllisk beliggende», «må oppleves» (reklamespråk) | Nøytral beskrivelse |

### Fine måter å si «er» og «har»

«fungerer som», «står som», «byr på», «kan skilte med» → skriv bare «er» eller «har».

### Åpnings- og avslutningsfraser

Kutt disse — start med poenget:

- «det er verdt å merke seg», «det er viktig å påpeke»
- «i dagens verden», «i en verden der», «i en tid der»
- «la oss utforske», «la oss dykke ned i»
- «la meg være ærlig» — falsk fortrolighet, gå rett på poenget
- «dette reiser spørsmål om» — still spørsmålet eller dropp det
- «oppsummert kan man si at», «kort sagt», «avslutningsvis»
- «det bør nevnes at», «husk at»
- «fremtiden ser lys ut», «resultatene taler for seg selv» — klisjeer; si konkrete planer eller fakta

### Retoriske AI-mønstre

- **«Ikke bare X, men også Y»** — skriv om til to setninger eller velg det viktigste.
- **«Det handler ikke om X, men om Y»** — falsk kontrast. Si bare Y.
- **«I en tid der...» + avsluttende perspektiv** — det mest kjente AI-mønsteret. Kutt hele innramminga.
- **Tredeling (trikolon)** — tre substantiv eller leddsetninger i serie («mennesker, teknologi og samhandling»). Ikke tving ideer inn i tregrupper — bruk det naturlige antallet. Én tredeling er OK, flere i samme tekst er et tydelig AI-tegn.
- **Falske spenn** — «fra X til Y» der X og Y ikke ligger på en meningsfull skala («fra frontend til bedriftskultur»). List temaene direkte.
- **Synonymveksling** — hovedperson, protagonist, sentral skikkelse i samme avsnitt. Velg ett ord og gjenta det.
- **Vage kilder** — «eksperter mener», «rapporter antyder», «flere har pekt på». Navngi kilden eller stryk påstanden.
- **Falsk muntlighet** — uformell åpning som brått skifter til polert byråkratspråk. Hold konsekvent tone.
- **Rettferdiggjøringsavsnitt** — avsnitt som forklarer hvorfor noe er viktig uten ny informasjon. Leseren skjønner at cyberøvelser er nyttige — du trenger ikke si det.
- **Overdreven gardering** — «kan potensielt muligens tenkes å» → «kan».

### Strukturelle mønstre

- Fjern oppsummeringssetninger som bare gjentar det du nettopp skrev
- Ikke tving balanse når ett alternativ er bedre («begge har sine fordeler»)
- Varier grammatisk struktur i kulepunkter — identisk form er et AI-tegn
- Ikke gjenta et poeng med andre ord rett etterpå
- Dropp «Derfor er X så viktig»-setninger som rettferdiggjør forrige setning uten å tilføre noe
- **Perfekt mal-struktur** — krok → kontekst → helt → resultat → det store bildet → konklusjon. Følger teksten dette slavisk, bryt det opp. Start med nyheten.
- **Chatbot-fraser** — «Håper dette hjelper!», «Si fra om du lurer på noe!», «Selvfølgelig!», «Godt spørsmål!». Fjern.

### Overgangsord

- «Videre», «Dessuten», «I tillegg» som avsnittåpning → bruk sjelden
- «I lys av dette», «Når det gjelder» → gå rett på sak
- «Furthermore», «Moreover», «Additionally» → aldri i norsk tekst

### Engelske AI-ord som siver inn i norsk

Skann etter de norske formene i venstre kolonne. De er langt vanligere i KI-generert norsk enn i vanlig norsk.

| Skann etter (engelsk opphav) | Gjør i stedet |
|------------------------------|---------------|
| «fordype seg i» (delve into) | Skriv bare innholdet |
| «utnytte», «leverere» (leverage) | «bruke» |
| «rike», «sfære» (realm) | «område», «felt» |
| «understreke» (underscore) | Si poenget direkte |
| «avgjørende» (crucial) | Si hvorfor det er viktig |
| «landskap» som metafor (landscape) | «markedet», «feltet», «situasjonen» |
| «fremme» (foster) | Si hva du gjør konkret |
| «navigere» (navigate) | «håndtere», «forholde seg til» |
| «effektivisere» (streamline) | Si hva som blir enklere |
| «vev», «samspill» (tapestry, interplay) | Dropp metaforen, si saken |

Abstrakte metafor-substantiv: «substrat», «vektor», «paradigme», «nordstjerne», «svinghjul», «fundament» (som metafor) → velg det konkrete ordet.

## Tegnsetting og formatering

- Ikke bruk tankestrek (—) i prosa. Bruk kolon, komma, parentes eller en ny setning.
- Overskrifter slutter aldri med kolon. Ellers er kolon greit før liste eller eksempel, ikke som setningslim midt i setninger. Kolon i hvert eneste kulepunkt er et AI-tegn.
- Ikke bruk semikolon unaturlig ofte.
- Dropp utropstegn i teknisk tekst.
- Ikke fet skrift på hvert egennavn eller forkortelse.
- **Innledningsord-lister**: Fet etikett + kolon som gjentar linja («**Ytelse:** Ytelsen ble bedre...») → skriv om til prosa. En fet innledning som navngir punktet og følges av genuint ny informasjon er grei.
- Fjern pynteemojis fra overskrifter og kulepunkter.
- **Store Bokstaver Midt i Setninger og Titler** er engelsk title case som lekker inn — et tydelig KI-tegn på norsk («Banebrytende Teknologi for Havvind Lansert»). På norsk har bare første ord og egennavn stor bokstav, også i titler og produktomtaler.
- Bruk norske anførselstegn «» i løpende tekst, ikke engelske krøllfnutter “”.
- Norsk tallformat: mellomrom som tusenskilletegn («151 354»), mellomrom før prosenttegn («20 %»).

## Fagtermer

### Alltid engelsk

Ikke oversett engelske tekniske termer som er etablert i norsk fagspråk:

- image (ikke «avbilde»), cluster (ikke «klynge»), node, container (ikke «beholder»)
- deployment (men «deploy» som verb og «rulle ut» er OK), release, plugin
- backup, failover, rollback, upstream, downstream, overhead
- secret, namespace, pod, CRD, PVC, PDB — aldri oversett Kubernetes-termer
- edge case, bug, bugfix, hotfix, patch
- roadmap, governance, community (i open source-kontekst)
- pipeline, workflow, runtime, framework, middleware
- pull request, merge, commit, branch, rebase
- endpoint, payload, token, scope

### Norsk er OK for

feilsøking, oppgradering, sikkerhetskrav, vedlikehold, bidragsytere, brukervennlighet, tilgjengelighet, kodegjennomgang, avhengighet. (Engelsk parallellform som «debugging» og «code review» er også OK.)

### Sammensatte ord med engelske termer

Bruk bindestrek — særskriving er feil:

```
✅ image-bygg, CI-pipeline, deploy-steg, CLI-brukere, PR-er
✅ Postgres-operatoren, Kafka-topicet, GitHub-repoet
❌ Postgres operatoren, Kafka topicet, GitHub repoet
```

## Anglisismer

Skill mellom etablerte fagtermer (behold engelsk) og unødvendige anglisismer (bruk norsk).

| Anglisisme | Norsk alternativ |
|----------|-----------------|
| «tok et øyeblikk» | «ventet litt», «nølte» |
| «i person» | «personlig», «ansikt til ansikt» |
| «adressere et problem» | «løse», «fikse», «ta tak i» |
| «på slutten av dagen» | «til syvende og sist» eller dropp |
| «å være på samme side» | «å være enige» |
| «ta eierskap til» | «ha ansvar for» |
| «delivere» | «levere» |
| «har du noen input?» | «har du innspill?» |
| «involvere» (overbrukt) | «ta med», «inkludere» |
| «ha en god dialog» | «snakke med», «samarbeide med» |
| «per dags dato» | «nå», «i dag» |
| «basert på» (overbrukt) | «ut fra», «med utgangspunkt i» |
| «i henhold til» (overbrukt) | «etter», «ifølge» |

## Norsk språkkvalitet

### Nav — ikke NAV

Nav skrives med stor forbokstav og små bokstaver ellers. Rett opp «NAV» og «nav» konsekvent.

### Formvalg

- Konsekvent bokmål, ikke bland inn nynorsk
- Moderne, ledig bokmål i interne tekster: «elva» og «sida» er gode valg
- Ikke veksle mellom gyldige former (stein/sten, framtid/fremtid) i samme tekst. Konsekvens er regelen, ikke ett bestemt valg
- «vi» og «du», ikke «man» og «bruker»
- A-endelser («sida», «fila», «endra») er gyldig ledig bokmål — behold dem hvis teksten er konsekvent

### Nynorsk og svensk som siver inn

Språkmodeller blander bokmål, nynorsk og svensk. Skann alltid etter disse — de er de vanligste feilene i KI-generert bokmål.

**Nynorsk → bokmål:**

| ❌ Nynorsk | ✅ Bokmål |
|-----------|----------|
| oppgåve | oppgave |
| eigenskap / eigentleg | egenskap / egentlig |
| handtere / handtering | håndtere / håndtering |
| tilgjengeleg / tydeleg / mogleg | tilgjengelig / tydelig / mulig |
| moglegheit | mulighet |
| viktigaste | viktigste |
| løysing | løsning |
| brukaren / brukarane | brukeren / brukerne |
| teneste / tenester | tjeneste / tjenester |
| endringar / innstillingar / oppdateringar | endringer / innstillinger / oppdateringer |
| naudsynt | nødvendig |
| kjeldekode | kildekode |
| sjølv | selv |
| nokon / kvar / kvifor / korleis | noen / hver / hvorfor / hvordan |
| fleire / meir | flere / mer |
| framleis | fremdeles / fortsatt |
| mellom anna / til dømes | blant annet / for eksempel |
| ikkje / medan / mykje / berre / difor | ikke / mens / mye / bare / derfor |
| vart / vorte | ble / blitt |
| dei | de |
| -ane (filane) | -ene (filene) |

**Svensk → bokmål:** engångs- → engangs-, användare → bruker, verktyg → verktøy, tillgänglig → tilgjengelig, nödvändig → nødvendig, möjlig → mulig, ändring → endring.

**Mønstre å skanne etter:**

- **-ingar** → **-inger** (oppdateringer, endringer)
- **-leg/-lege** → **-lig/-lige** (tydelig, mulig)
- **-aste** → **-ste** (viktigste)
- **ei-/eig-** i starten → **e-/eg-** (egenskap, egentlig)
- **kv-** → **hv-** (hver, hvorfor)
- **-ar** i ubestemt flertall → **-er** (brukere, tjenester), **-ane** i bestemt flertall → **-ene** (brukerne, tjenestene)
- **å** der bokmål har **a** → sjekk om det er svensk (engangs-, ikke engångs-)

**Obs:** «oppdaga» er valgfri bokmålsform og skal beholdes ved konsekvent bruk. «oppdateringar» er alltid nynorsk og skal alltid rettes.

## Teksttyper

### ADR (Architecture Decision Record)

- Kontekst kort og faktabasert
- Beslutning i presens, aktiv form: «Vi bruker X» ikke «Det ble besluttet å benytte X»
- Konsekvenser konkrete, ikke vage

### README og onboarding

- Start med hva prosjektet gjør (én setning), deretter hvordan komme i gang
- Ikke selg eller rettferdiggjør prosjektet — vis hva det gjør

### Blogginnlegg og artikler

- Start med det som er nytt, ikke historisk kontekst eller «definere temaet»-innledning
- Aktiv form, gjerne med «vi»

### UI-tekst og mikrotekst

Følg Designsystemets retningslinjer for tekst i digitale tjenester:

- **Knapper**: korte og handlingsorienterte. «Lagre», «Send inn» — ikke «Klikk her for å lagre»
- **Feilmeldinger**: si hva som gikk galt og hva brukeren kan gjøre. «Du må fylle ut fødselsnummer» ikke «Feltet er påkrevd»
- **Hjelpetekst**: forklar hva feltet betyr, ikke hvilke API-felt det kommer fra
- **Bekreftelser**: «Endringene er lagret» ikke «Operasjonen ble gjennomført»
- **Lenketekst**: beskrivende, ikke «klikk her» eller «les mer»

## Før og etter

```
❌ Det er viktig å påpeke at Kubernetes representerer et betydelig skritt
   fremover innen container-orkestrering, og spiller en avgjørende rolle
   i moderne skyinfrastruktur.

✅ Kubernetes orkestrerer containere. Vi bruker det til å kjøre og
   skalere appene våre i clusteret.
```

```
❌ Vi må rulle tilbake avbildet og opprette en ny hemmelighet
   i navnerommet.

✅ Vi må gjøre rollback på imaget og opprette en ny secret
   i namespacet.
```

```
❌ Denne PR-en adresserer behovet for å implementere en mer robust og
   helhetlig løsning for autentisering som tilrettelegger for en sømløs
   brukeropplevelse.

✅ Bytter fra manuell token-validering til @navikt/oasis. Forenkler
   auth-flyten og fikser bug der utløpte tokens ikke ble refreshet.
```

```
❌ Vi har nå gjennomgått de ulike aspektene ved migrasjonen. Oppsummert
   kan man si at en vellykket migrering krever grundig planlegging.

✅ (Kutt hele avsnittet. Leseren har allerede lest det du oppsummerer.)
```

## Grenser

### ✅ Alltid

- Det viktigste først, aktiv form, konkret språk
- Behold etablerte engelske fagtermer og bindestrek i sammensatte ord
- Konsekvent formvalg gjennom hele teksten
- Behold kode-literals, API-felter, IDer, enum-verdier og testforventninger uendret

### ⚠️ Spør først

- Endringer som kan påvirke faglig innhold
- Omstrukturering av hele dokumenter
- Fjerning av hele avsnitt (ikke bare setninger)

### 🚫 Aldri

- Endre programlogikk, funksjoner, API-er eller konfigurasjon
- Endre faglig innhold eller tekniske beslutninger
- Oversette etablerte engelske fagtermer til norsk
- Innføre nynorsk i bokmålstekster
- Legge til innhold som ikke finnes i originalen

## Kilder

- [Språkrådets KI-rapport](https://sprakradet.no/aktuelt/ki-sprakets-fallgruver/) (januar 2025) — 2,6 feil/side på bokmål, konservativt formvalg, engelsk som skinner gjennom
- [Språkrådets framtidsutsikter for norsk språk](https://sprakradet.no/aktuelt/framtidsutsikter-2/) (mars 2026) — KI-produserte tekster har alvorlige språklige feil på både bokmål og nynorsk, og KI-robotene har normautoritet
- [Kommunikasjonsforeningen om «ChatGPTsk»](https://www.kommunikasjon.no/fagstoff/guider-og-maler/2025/skriver-du-chatgptsk-slik-unngar-du-a-hore-robotisk-ut) — kjennetegn på robotisk norsk og overbrukte KI-fraser
- [Språkrådets klarspråk-prinsipper](https://sprakradet.no/Klarsprak/) — det viktigste først, aktiv form, skriv for leseren
- [ISO 24495-1](https://sprakradet.no/klarsprak/kunnskap-om-klarsprak/iso-standard-for-klarsprak/) — internasjonal klarspråk-standard
- [Digdirs klarspråk-veileder](https://www.digdir.no/klart-sprak/ny-veileder-om-klart-sprak-i-utvikling-av-digitale-tjenester/3603) — klarspråk i digitale tjenester
- [Designsystemets tekstpraksis](https://designsystemet.no/no/blog/shared-guidelines-for-text/) — retningslinjer for tekst i UI-komponenter
- [Termportalen](https://www.termportalen.no/) — nasjonal portal for norske faguttrykk
