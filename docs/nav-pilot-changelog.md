# Nav-Pilot Changelog

Endringslogg for nav-pilot agent harness — agenter, skills, instruksjoner, prompts og samlinger.

## 2026-09-03

### cplt-oppdateringer etter oppstrømsgjennomgang

- **`doctor` verifiserer nå håndheving, ikke bare oppsett**: Sjekken leste `cplt --version` og `cplt config show`, som svarer på om cplt er konfigurert, aldri på om sandboxen faktisk håndhever. `cplt check --json` ([navikt/cplt#145](https://github.com/navikt/cplt/pull/145)) kjører prober inne i den ekte resolverte sandboxen og eksiterer ikke-null når den ikke kan bekrefte håndheving. En eldre cplt uten subkommandoen, eller en cplt som ikke finnes, gir «vet ikke» — aldri en grønn hake og aldri en falsk feil.

## 2026-09-01

### opencode så aldri det teamet la inn for hånd

- **Scopet materialiseres nå sammen med kilden**: Opencode-artefaktene ble laget kun fra kildesjekkouten, aldri fra `.github/`. Copilot leser scopet direkte og så alt, opencode fikk bare det nav-pilot selv hadde levert. Et hub-repo med 25 installerte skills og 3 egne fikk 25, uten varsel og uten spor i tilstandsfila. Skills, prompts og agenter fra scopet blir nå med, både ved oppstart, `nav-pilot sync` og `nav-pilot export opencode` (#579).
- **Instruksjoner slås bevisst ikke sammen ved oppstart**: Utmappa er den globale opencode-konfigurasjonen og `AGENTS.md` er alltid-på kontekst, så ett repos instruksjoner ville fulgt brukeren inn i alle andre. `export opencode` skriver `<repo>/.opencode/` og tar dem med, siden den innvendingen ikke gjelder der (#579).
- **Slettelogikken sjekker nå hash**: Den fjernet alt som ikke lenger var i filsettet, uten å se på innholdet. Med scopet som input krymper settet hver gang du bytter repo, og en fil du selv hadde redigert i opencode-konfigurasjonen ville forsvunnet. Nav-pilot sletter nå bare det den selv har skrevet og som fortsatt er uendret. En fil som blir stående fordi du har redigert den, forblir sporet, slik at neste sync i repoet den kom fra melder konflikt framfor å overskrive den (#579).

## 2026-08-31

### Instruksjoner: én alltid-på output-style, tre agenter pensjonert

- **`output-style.instructions.md` med `applyTo: "**"`**: Reglene for lengde, tetthet og anti-slop er samlet i den ene laget som faktisk alltid er i kontekst. De lå tidligere i tre divergerende kopier (`agents/forfatter.agent.md`, `instructions/norwegian-text.instructions.md` og `.github/copilot-review-instructions.md`), og em-strek var styrt kun ett av stedene. `skills/terse-mode` åpnet med «ACTIVE EVERY RESPONSE», men en skill fyrer etter modellens eget skjønn, noe som forklarer hvorfor output holdt seg langt selv om skillen var installert overalt. Presedensregelen mot `deliberate-ai-use.instructions.md` er skrevet eksplisitt, siden to alltid-på-instruksjoner som drar hver sin vei gir dårligere output, ikke kortere (#481).
- **`terse-mode` beholdt, men slanket**: Navn, fem manifestoppføringer og intensitetsnivåene består. Reglene flyttes til instruksjonen, og skillen blir en peker. Ingenting brytes for installerte brukere (#481).
- **Anti-slop-dekning i alle samlinger**: `kotlin-backend` og `platform` hadde ingen dekning i det hele tatt før dette (#481).
- **Tre deprecerte agenter pensjonert**: `auth`, `nais` og `observability` (til sammen 1 707 linjer) er fjernet. Innholdet ble lest gjennom mot erstatningsskillene og migrert inn i `nav-auth` og `observability-setup` i samme PR, ikke bare slettet. Netto 1 470 linjer fjernet over 63 filer (#481).

### nav-pilot: konsistent modellvalg på tvers av klienter

- **`pi` forkastet pass-through-argumenter stille**: `nav-pilot --client pi -- run "…"` startet pi uten forespørselen. Varselfunksjonen dekket to av sju innstillinger som blir droppet, så effort, konteksttier, allow-all-tools, ask-user og loggnivå forsvant uten et ord (#490).
- **Materialiserte opencode-agenter bærer nå `model:`**: `transformAgent` bygde frontmatter fra bunnen med kun `description` og `mode`, så modellinjen på 12 av 14 agenter nådde aldri klienten. Verifisert mot opencode 1.18.25: klienten honorerer `model:` i agent-frontmatter, men avviser copilots `tools:`-liste, og derfor er gjennomslippet holdt allowlist-formet (#490).
- **Tier 1 copilot fikk en default-modellerklæring**: Den stagede Tier 2-stien falt allerede tilbake på en pakke-erklært modell for begge klienter. Tier 1 copilot, som er standardoppsettet, var den eneste stien uten noen erklæring. Verdien er `inherit`, ikke en modell-id og ikke `auto`, slik at argv er byte-identisk på alle stier og valget av faktisk verdi forblir en egen beslutning (#490).
- **Brukeren får nå vite hvilken modell hen kjører på**.

### nav-pilot: golden-harnesset måler outputstørrelse

- **Instruksjoner materialiseres i kjøringen**: Harnesset kopierte bare personaen inn i arbeidsmappa, så en `applyTo: "**"`-regel var aldri i kontekst under en kjøring. Det ville målt uendret persona og rapportert null effekt, noe som ser identisk ut med en regel som ikke virker. Kopieringen følger nå `nav-pilot install --repo` framfor å reimplementere transformen (#482).
- **Gjentak, baseline og sammenligning**: `--repeat N` kjører hver prompt N ganger og rapporterer median med min/maks-spredning. `--save-baseline` og `--compare` gjør før/etter til en diff mot en registrert kjøring. Størrelser blir aldri assertert og påvirker aldri exit-koden (#482).
- **Bestått-semantikk over gjentak**: En test feiler dersom én av kjøringene feilet, rapportert som `2/3 passed, 1 failed`. Flertallsvotering ville gjort TokenX-kanarifuglen stillere akkurat idet den begynte å ryke (#482).
- **Fortsatt bevisst utenfor CI**: Ekte modellkall, ekte penger, ikke-deterministisk. Ingen mise-oppgave heller, siden alle mise-oppgaver i repoet er CI-trygge (#482).

### Måling: fire modeller på personvern-blindsonen

Første benchmark av personaen mot levende modeller, rundt 195 kjøringer på én påstand fra golden-test 3: at personaen reiser blindsonene personvern og tilgangskontroll for prompten «ny tjeneste som leser fnr fra ID-porten».

| Modell | Feil | Kjøringer |
|---|---|---|
| Claude Sonnet 4.6 (sittende) | 2 | 50 |
| GPT-5.6 Sol | 1 | 50 |
| GPT-5.6 Luna | 1 | 50 |
| GPT-5.6 Terra | 5 | 45 |

- **Ingen av forskjellene er signifikante**. Fisher eksakt mot den sittende modellen gir p = 1,00 for Sol og Luna og p = 0,25 for Terra. Punktestimatene ser ut som en rangering, men konfidensintervallene overlapper alle. Målingen viste ikke at noen modell er tryggere, og ikke at noen er mindre trygg.
- **Funnet som betyr mer enn modellvalget**: Den påkrevde personvern-blindsonen blir oversett på alle modeller som ble testet. Feilen ligger i personaen, ikke i modellen, og ingen modellbytte fikser den.
- **Metodisk lærdom**: Ved n = 5 er én observert feil forenlig med en sann feilrate mellom 3,6 og 62,4 prosent. En tidlig n = 5-runde fikk Terra til å se utrygg ut og Luna til å se ren ut, og begge deler falt bort ved høyere n.

Protokollen ligger i `docs/nav-pilot-benchmark-og-beslutninger-2026-08.md`, foreløpig kun på gren i #496, som ikke er merget.

### Kjent feil: personaen sender ikke fase-checkpoint på full tier (#484)

- **Symptom**: `agents/nav-pilot.agent.md` krever at full-tier-forespørsler stopper etter hver fase og sender en checkpoint-blokk. Modellen sender den ikke. Alt annet er riktig: arketypen kjennes igjen, tieren klassifiseres riktig, Fase 1 kjøres, blindsone #1 og #2 reises, intervjuspørsmålene stilles. Det er checkpoint-blokka som mangler, i 11 av 11 kjøringer på tvers av tre oppsett.
- **Ikke en regresjon**: Golden-harnesset ble laget i #442 og kjørt mot ekte modellkall for første gang nå. Test 2 har aldri bestått. Feilen har ligget der siden #273 (3. juni), som fjernet imperativen festet til selve malen.
- **Årsak, slik den er forstått**: Checkpointen påstår «Fase N ferdig» i det øyeblikket fila selv sier at fasen ikke er ferdig, siden utgangskriteriet krever at blindsonene er adressert og brukeren har bekreftet. Klausulen «Output ONLY the checkpoint block» står dessuten mot flere steder som krever at Fase 1 stiller spørsmål.
- **Ikke fikset**: Første forsøk (#491) er verifisert med harness-kjøring og virker ikke. Test 2 står uendret på 0 av 5, og outputstørrelsen på prompten gikk opp 56 prosent uten at blokka kom. Fasegrensa finnes altså i personaen, men ikke i praksis: brukeren får ingenting å bekrefte.

### my-copilot: modellpriser og prisside

- **Prisdata synket etter drift**: `mise run pricing:check` fantes, men sto i ingen workflow, så `model-pricing.ts` drev fra GitHubs publiserte tall uten at noen oppdaget det. GPT-5.6 Sol gikk fra $5,00/$30,00 til $2,00/$10,00, Gemini 3.6 Flash halverte seg, og MAI-Code-1.1-Flash og Gemini 3.7 Flash manglet. Korreksjonen velter to konklusjoner i `docs/modellvalg.md`, som ikke er rettet i samme PR (#486).
- **Daglig drift-workflow**: Kjører daglig og åpner PR med regenerert fil når tallene flytter seg. Den gater bevisst ikke PR-er: priser er tredjepartsdata på GitHubs tidsplan, og en påkrevd sjekk ville gjort deres prisendring til rødt bygg på `main` for den som pushet neste gang (#486).
- **Prissiden åpnet**: `/priser` var lukket ved et uhell. Ruta står ikke i `PRIVATE_PAGE_PATHS`, men manglet også i `autoLoginIgnorePaths` og fikk dermed Wonderwalls auto-login-redirect. Siden er en server-komponent uten datahenting og viser kun GitHubs offentlige modellpriser. Lagt til i navigasjon og `sitemap.ts` med etiketten «Modellpriser» (#495).

## 2026-08-30

### CI: gitleaks blokkerte alle PR-er mot main

- **Modellmanifestet allowlistet på main**: Gitleaks-jobben henter hele historikken, men bruker `.gitleaks.toml` fra grenen som er sjekket ut. Regelen som dekker `cli/nav-pilot/internal/local/models.json` fantes bare på `local-inference` (#483), så alle grener ut fra `main` så commitene i historikken, manglet regelen og feilet. Regelen er kopiert uendret til `main`. Skanneomfanget er ikke rørt (#489).

## 2026-08-28

### nav-pilot: Tier 2-revisjonen pinnes, per-launch-staging pensjoneres

**Breaking change.** En agentpakke i Tier 2 kunne launches, men ikke installeres, og hver launch klonet den bevegelige default-grenen på nytt.

- **Install materialiserer og pinner**: Hver deklarert kontekst av hver payload-bærende klient skrives til `~/.nav-pilot/pakker/<owner>-<repo>/<sha>/`, publiseres med én `os.Rename`, og SHA-en lagres som pin. `sync` flytter pinnen, `uninstall` fjerner revisjonene, og maks to beholdes slik at en økt overlever én oppdatering (#475).
- **Tillitsgrensa flyttes bevisst**: Treet klienten leser bygges nå ved install og blir stående i ukevis, ikke mikrosekunder før launch. `VerifyPayloadExact` ved launch fanger fortsatt sha256 og eksakte rettighetsbiter per manifestert fil, umanifesterte ekstrafiler, symlenker og bytte mellom lstat og open. Begrunnelsen er skrevet ned i `docs/agentpakke-beslutninger.md` §3.1 (#475).
- **Per-launch-staging slettet**: `GCStaged`, `CleanupStaged`, `stagedRoot`, `stagedMaxAge`, navnevakta og 24-timers feiingen er borte. Begge grunnene til at modellen fantes dør med pinnen (#475).
- **Brytende detaljer**: En blandet pakke (både `layout` og `payloads`) nekter å launche Tier 2-klienten sin, framfor stille å degradere til Tier 1-innhold. En kilde med lokal sti kan ikke installeres (#475).

### nav-pilot: launch klienten i stedet for å spørre

**Breaking change.** To spørsmål sto mellom brukeren og en kjørende klient.

- **Å takke nei til synk stoppet launchen**: `interactiveSyncAndLaunch` returnerte på alt annet enn «ja», og returen lå over launchen. En installasjon med tilgjengelig oppdatering startet altså ingenting. Siden hver installasjon blir utdatert i det en release lander, gjorde én release «Nei» om til «nav-pilot gjør ingenting» for alle samtidig. Å takke nei hopper nå over synken, ikke launchen. Kun Ctrl-C stopper kjøringen (#476).
- **Sandbox-spørsmålet er blitt et varsel**: En bekreftelse der eneste fornuftige svar er «ja» er et tastetrykk, ikke en beslutning. `launchConfirmUnsandboxed` er blitt `launchWarnUnsandboxed`: samme deteksjon, samme melding, ingen prompt, deretter launch. `auto_launch = false` er uendret og er fortsatt måten å aldri launche på (#476).

### Dokumentasjon: språkvask og komprimering av hele settet

- **20 filer, 1 456 linjer inn og 1 057 ut**: `AGENTS.md`, `ARCHITECTURE.md`, `README.md`, `SECURITY.md`, klientstøttematrisen, alle `docs/README.*` og `docs/agent-harness.md`, `docs/ai-for-arkitekter.md`, `docs/modellvalg.md` og `docs/nav-planning-skills-rfc.md` (#479).
- **G4-baselinen flyttet**: Team eSyfo merget grillmester#63 og ba om at den vurderte baselinen flyttes. Arbeidet avdekket at to ulike ting delte én SHA: baselinen som begge prosjekter differansetester mot, og linjenummer-sitatene i kildekommentarer, som registrerer hva vi faktisk leste da koden ble skrevet og ikke blir usanne av at baselinen beveger seg (#480).

## 2026-08-27

### nav-pilot: Tier 2-agentpakker, verifisering, staging og launch

- **Payloads verifiseres mot manifestet sitt**: `VerifyPayload` feiler lukket på symlenker hvor som helst i treet, filer på disk manifestet ikke lister, manifesterte filer som mangler, digest-avvik, exec-bit-avvik, ødelagte poster og manifestnøkler som rømmer payload-mappa (`..`, absolutte stier, backslash). Koblet inn i `nav-pilot validate` slik at avvik meldes der pakkeforfatteren ser dem (#454).
- **Staging i et privat tre**: `StagePayload` verifiserer kilden, kopierer inn i en fersk mappe styrt av manifestets filliste framfor å gå kilden gjennom, re-hasher hver fil under skriving, oppretter alt med `O_EXCL` og eksakt deklarert modus, og re-verifiserer kopien med eksakt modus-sammenligning. Symlenker, hardlenker og spesialfiler kan ikke oppstå ved konstruksjon. Egen mappe per launch, slik at én launch ikke sletter treet en samtidig økt leser (#456).
- **Launch fra staget payload**: OpenCode får `OPENCODE_CONFIG_DIR` på staget sti videreført gjennom cplt med `--pass-env` og `--allow-read`; Copilot CLI får `--plugin-dir` og den plugin-kvalifiserte personaen pakka deklarerer. Staget Copilot krever cplt uten fallback, siden payloaden er uverifisert tredjeparts konfigurasjon. En staget økt arver ikke brukerens egne Copilot-instruksjoner. Kontekstflagget heter `--payload-context`, siden `--context` allerede navngir Copilots konteksttier (#458).
- **Personaer leses fra det aktive manifestet**: Persona-navn og OpenCode-allowlisten lå som konstanter flere steder. Atferden er uendret for alle, siden det aktive manifestet defaulter til `Default()`-literalen med nøyaktig de samme navnene. Gullstandard-testene ble skrevet først, mot uendret kode, og mutasjonssjekket (#455).
- **Agentrosteret tilhører payloaden, ikke klienten** (**breaking change i agentpakke-kontrakten**): `primaryAgents` var deklarert per klient, men en Tier 2-klient har flere payloads med ulike rostere, så fokusert launch kunne velge en agent payloaden ikke inneholder. Tier 1 beholder rosteret på klientnivå; Tier 2 deklarerer det per payload, påkrevd, `minItems: 1`, første element er kontekstens default-persona. Ingen fallback i koden. `contractVersion` forblir `"1"` siden Tier 2-install ennå er nektet og ingen bruker har en Tier 2-kilde (#461).
- **cplt-gulv og kompatibilitetsspenn håndheves ved launch**: Gulvet er `2026.08.17-062831`, releasen grillmester v0.3.0 er testet mot, som konstant i nav-pilot framfor kontraktsfelt. Et deklarert spenn (`">=1.18.20,<2"`) sjekkes mot klientens egen versjon; opencode rapporterer sin egen, copilot probes gjennom sandboxen. Begge portene kjører kun på den stagede stien, før noe bygges eller launches. Alt usikkert er fatalt, i motsetning til regelen ellers i nav-pilot (#462).

### Agentpakke: beslutninger skrevet ned

- **`docs/agentpakke-beslutninger.md`**: En dag med agentpakke-arbeid la igjen begrunnelsene sine i commit-meldinger og PR-beskrivelser. Dokumentet samler beslutningene der en leser vil lete, og er skrevet for oss, ikke for pakkeforfattere (#459).
- **Deprecation-vindu, eierskap og endringsprosess**: Vinduet er 90 dager fra kunngjøring til fjerning. Kontrakten eies av `@navikt/copilot` per CODEOWNERS, en pakke av repoet den klones fra, og `owner` i manifestet er attribusjon, ikke tilgangskontroll. Referansepinning flyttes ved enighet med eierne av det den måles mot (#460).
- **Tillitsgrensa sagt rett ut**: Payload-manifestet ligger i samme repo som payloaden og er skrevet av samme part, så kjeden er selvsignert. Digester beviser at treet er uendret, ikke at noen har vurdert det. Det som faktisk begrenser en fiendtlig pakkeforfatter er tre ting: at brukeren velger kilden, cplts sandbox, og at en staget økt ikke arver brukerens egen kontekst. Rekkefølgen er også pinnet: revisjonspinnen skal inn i samme endring som install-opplåsingen (#463).
- **Veien fra samlinger til én agentpakke kostnadsberegnet**: Kontrakten tilbyr nøyaktig ett installerbart navn per repo, mens Nav har fem navngitte samlinger. Målingen viser at de fem er nesten nøstede: `frontend` ⊆ `nextjs-frontend` ⊆ `fullstack`, `kotlin-backend` ⊆ `fullstack`, og `platform` skiller seg fra `fullstack` med nøyaktig ett artefakt (#465).

### nav-pilot og skills: øvrig

- **cplt strict-preset anbefales, og versjonsavvik rapporteres**: `doctor` leser effektiv `sandbox.preset` og foreslår `cplt config set sandbox.preset strict` når den ikke er `strict`. Det er en anbefaling, ikke en feil. `doctor` sammenligner også installert cplt mot siste release og utvider Homebrew-linja til `brew upgrade navikt/tap/nav-pilot navikt/tap/cplt` ved etterslep. nav-pilot skriver aldri nøkkelen stille (#452).
- **Utdatert begrensning fjernet fra DESIGN.md**: «Ingen passthrough av argumenter til Copilot CLI» stemte ikke lenger; `--` fanges opp og legges på i begge launch-stiene (#444).
- **`aksel-builder` peker ikke lenger på avviklet `Alert`**: «Inline page alert» viser til `LocalAlert`, `GlobalAlert` og `InfoCard`, og `Alert` er lagt i lista over avviklede komponenter (#448).
- **AI Coding News Excerpts-workflowen pensjonert**: 38 planlagte kjøringer på rad feilet siden 8. juni, og siden 17. august feilet den stille. Alle 149 artikler under `docs/news/articles/` er urørt og rendres fortsatt (#445).
- **copilot-metrics: scope-bevisst inntak**: Tre steder antok at data alltid ligger under enterprise-scope, mens uthentingen faller tilbake til org-scope når enterprise-endepunktet feiler. Det, ikke `upsertReport`, var grunnen til dupliserte dager (#446).

## 2026-08-26

### nav-pilot: interaktiv innstillingsside og scope-spørsmål ved install

- **`nav-pilot config` uten subkommando**: Skrev tidligere en usage-feil. På terminal åpner den nå en innstillingsside med hver brukerrettede nøkkel, gjeldende verdi, hvor verdien kommer fra (fil, miljø eller default) og hva nøkkelen gjør. Booleans og nøkler med faste verdier får plukker, resten prefylt input, og tom verdi fjerner nøkkelen. `config show` bygges nå fra samme nøkkeltabell, så merkingen ikke kan drive fra hverandre. Uten terminal er usage-feilen uendret (#450).
- **Install-scope spørres om**: `nav-pilot install kotlin-backend` og `nav-pilot add` skrev til repoets `.github/` uten å spørre, noe som overrasket folk som ventet en install på brukernivå. Spørsmålet hoppes over ved eksplisitt scope, `--json`, `--dry-run` og ikke-interaktive kjøringer (#450).
- **my-copilot: risting på budsjettlinja over 90 prosent**, bak `prefers-reduced-motion: no-preference` (#449).

## 2026-08-24

### nav-pilot: agentpakke-kontrakten (M1)

- **Kontrakt og skjema**: En pakke leverer `.nav-pilot/agentpakke.json`, og skjemaet `cli/nav-pilot/schemas/agentpakke-v1.json` er embeddet i binæren slik at pakkerepoer kan linte mot samme fil i sin egen CI. Tier utledes av form (`payloads` gir Tier 2, ellers Tier 1 via `layout`). Ukjente klienter, kontekster og felter ignoreres etter kontraktens additiv-regler; ødelagte kjente konstruksjoner feiler lukket (#436).
- **Én intern valuta**: `Manifest` er eneste interne representasjon. Kilder uten manifest adapteres av `SynthesizeLegacy`, så det finnes ingen legacy-vs-manifest-forgreninger nedstrøms (#436).
- **`source`-nøkkel og `nav-pilot validate`**: Presedens `--source` > config > default, lagret først etter vellykket eksplisitt install (#436).

### nav-pilot: trimmet alltid-lastet kontekst og nytt golden-harness

- **Netto 145 tokens (3 prosent) mindre alltid-lastet kontekst**, og det beskjedne tallet er poenget: den ærlige konklusjonen er at innholdet stort sett bærer sin egen vekt. To påstander i forarbeidet (#441) viste seg feil og er rettet der: `nav-pilot-opus.agent.md` gjør ikke samme jobb på 702 tokens (den er en leaf-only ledsager uten fasemaskin, scope-klassifisering eller blindsonesjekkliste), og `deliberate-ai-use.instructions.md` er ikke løs policy-prosa, men noe personaens rødsone-erklæring avhenger av (#442).
- **`scripts/nav-pilot-golden.sh`**: Testoppsett for innholdet, som ikke fantes fra før. Bevisst utenfor CI (#442).

### Dokumentasjon

- **rtk-anbefalingen trukket**: nav-pilot reklamerte for rtk med «60-90 % on token costs» og «Highly Recommended». Tallet er rtks egen selvrapport. Promoteringen er fjernet og rtk installeres ikke lenger automatisk, men er fortsatt tilgjengelig. Samtidig rettet en reell install-defekt (#438), med påfølgende språkvask av den norske teksten (#440).
- **Artikkel om Claude-tekstvannmerking**: Kolleger har reist bekymring for at vannmerket kan spore enkeltpersoner. Artikkelen argumenterer fra uavhengige kilder framfor leverandørforsikring, siden det siste ikke avgjør et personvernspørsmål for et offentlig organ (#439).

## 2026-08-23

- **`doctor` sluttet å melde en falsk feil**: Sjekken grep etter «nav-pilot» i `cplt config show` og foreslo `cplt config set copilot.agent_name nav-pilot`. Nøkkelen har aldri eksistert i cplt, så sjekken kunne aldri passere og hver `doctor`-kjøring endte i et spøkelsesvarsel. Personaen pinnes av nav-pilot selv ved hver launch, så doctor rapporterer den nå i stedet for å sjekke etter den (#432, lukker #406).
- **`jackson-3-migration`: `JsonNode.map()`-skyggelegging**: Skillen oppdager og retter nå de subtile bruddene Jackson 3.1 introduserte, som ikke nødvendigvis gir kompileringsfeil (#431).

## 2026-08-20

- **IntelliJ MCP-server oppdatert til 2026.2.1** i `apps/mcp-registry/allowlist.json` (#430).

## 2026-08-17

- **opencode defaulter til `github-copilot/auto`**: Kommandobyggerens output er normalisert til provider-prefiksede modell-id-er. Holder modellvalget fleksibelt og kostnadsbevisst framfor pinnet til én historisk modell (#421).

## 2026-08-14

- **Brukerens modellvalg respekteres igjen**: Modellpinnen i nav-pilot-agentens frontmatter overstyrte `--model` og config noen hundre millisekunder etter launch, og er fjernet. Modellstrategien i selve teksten består, og spesialistagentenes pinner er uendret. De kuraterte Copilot- og OpenCode-listene er oppfrisket fra models.dev med `claude-opus-5`, `claude-fable-5`, `gpt-5.6-sol/terra/luna`, `gemini-3.6-flash`, `kimi-k2.7-code` og `kimi-k3`, slik at GA-modeller ikke lenger utløser «not a recognized model» (#426, lukker #425).
- **Retningslinjene krever isolasjon**: AI-agenter skal kjøre sandboxet på Nav-utstyr, både for Nav-arbeid og privat arbeid. Å kjøre agenter uten isolasjon er lagt til i ikke-tillatt-lista (#427).

## 2026-08-10

- **Sandboxing-krav dokumentert for nav-pilot**: cplt er anbefalt isolasjonsmekanisme for alt agentarbeid på Nav-eid utstyr. Egen delbar artikkel, lenket fra både nav-pilot- og cplt-dokumentasjonen (#417).

## 2026-08-06

- **`nav-dekoratoren`-skillen dokumenterer `origin`**: Parameteren fra `@navikt/nav-dekoratoren-moduler` v4.3 identifiserer appen på automatiske `besøk`-hendelser. Lagt til i SSR-eksemplene og parameterreferansen, med fallback beskrevet (#413).

## 2026-07-31

- **Ny skill `jackson-3-migration`**: Migrerer Jackson 2.x (`com.fasterxml.jackson`) til 3.x (`tools.jackson`) i Kotlin- og Java-prosjekter. Pre-flight-sjekker, unntaket for `jackson-annotations`, automatisert OpenRewrite-pass, Kotlin-spesifikk opprydding og verifisering (#407).

## 2026-07-29

- **Feilsøkingsguiden krasjet**: `/praksis/guide/feilsoking` traff error-boundaryen. `@navikt/ds-react` har `"use client"` i pakkeroten, så `Accordion` er en klientmodul: en server-komponent kan rendre den, men ikke lese felter av den. Samlet med en rettelse av Linux-installasjonen (#402).
- **`v_repository_usage` opprettet og manglende repo-dager backfilt**: Viewet fantes ikke i noen av BigQuery-prosjektene, `/api/v1/copilot/usage/repositories` svarte 500, og Repositorier-fanen var nede i to døgn uten at noen alarm gikk (#401).
- **Lenker til aksel-mcp oppdatert** (#405).

## 2026-07-26

### copilot-metrics og my-copilot: bruk på repositoriumsnivå

- **Inntak av Copilot-bruksmetrikk per repositorium** (#388), **`v_repository_usage` med personvernundertrykking** (#389), **endepunktet `/api/v1/copilot/usage/repositories`** (#390) og **Repositorier-fanen i my-copilot** med sorterbar, paginert og søkbar tabell (#391). Designet er beskrevet i #387; designdokumentet ble senere fjernet til fordel for PRD-en (#394).
- **PR review-velocity**: `avg_pull_requests_minutes_to_review` og `avg_pull_requests_review_cycles` fra Copilot usage-API-et, som GitHub la til 7. juli, hentes fram per fase i `totals_by_ai_adoption_phase[]` (#369, oppfølging i #386).

## 2026-07-25

- **Juli-nyheter**: Claude Opus 5 (`claude-opus-5`, lansert 24. juli) med ny artikkel og oppdatert `modellvalg.md` og prisdata. Gemini 2.5 Pro og Gemini 3 Flash utfases 31. juli, med oppdaterte anbefalinger. Seks øvrige julisaker, og kostnadskalkulatoren er fjernet (#385).

## 2026-07-23

- **Fem nye modeller slått på for Nav i GitHub Copilot**: GPT-5.6 Luna, Sol og Terra, Kimi K2.7 Code (første open-weight-modell i Copilot) og Gemini 3.6 Flash. Én nyhetsartikkel per modell, pluss en utviklerguide for hele modellflåten (#379).

## 2026-07-14

- **lefthook regenererer manifester og docs**: En `pre-commit`-kommando kjører `mise run generate` og stager genererte app-manifester og docs-tabeller når en agent-, skill-, instruksjons-, prompt- eller samlingskilde er staget (#363).
- **`observability-debugging`: Loki-eksemplene bruker globalt endepunkt**: `loki.nav.cloud.nais.io` med eksakt `k8s_cluster_name="$CLUSTER"`-label, i tråd med Mimir-mønsteret, i stedet for regex per miljø (#362).

## 2026-07-05

- **cplt security-hardening i my-copilot**: Landingsside og config explorer gjenspeiler bubblewrap-isolasjon, tvunget proxy-modus og oppstrøms proxy-kjeding, med norsk nyhetsartikkel. Underveis ble det funnet at config explorer-parseren stille droppet 9 av 53 nøkler (#352).
- **`ci-ok`-port kompatibel med merge queue**: Ingen workflow hadde `merge_group:`-trigger og hovedrulesettet hadde ingen påkrevde sjekker, så køoppføringer ble merget etter fem minutters venting uten å ha kjørt CI mot den mergede kombinasjonen. Én påkrevd sjekk for alle PR-er (#353).
- **gitleaks-skanning innført**: Skanner full git-historikk på push og PR mot main, med pinnet binær og sha256-verifisering i stedet for actionen, som krever betalt lisens for organisasjoner. `.gitleaks.toml` allowlister tre bekreftede falske positive (#348).
- **`nav.repo` i OTel-ressursattributter**: Repo-slug utledes fra origin-remote ved launch og gjør spørretidsjoin mot repo-til-team-mapping mulig. Attributtet identifiserer en kodebase, aldri en person, og utelates helt utenfor navikt-organisasjonen (#347).
- **CodeQL-varsler og Dependabot npm-varsler ryddet** (#349).

## 2026-07-03

### my-copilot og copilot-api: herdingsrunde etter adversarial review

Runden fra 1. til 3. juli ligger som direktecommits uten PR-numre.

- **Tilgangskontroll**: Eierskap håndheves på per-bruker-leseendepunkter, og en router-nivå-test beviser at `/api/v1/` krever auth.
- **Datariktighet**: `GENERATE_DATE_ARRAY` off-by-one og delvis dag i prognosen, PR-median-fallback, `formatMinutes`-overflow, månedslogikk samlet i delte `month-utils`, faktureringsgrafer hentes fra daglig modelltabell, og hull i inntaket av supplerings- og budsjettdata er tettet.
- **Ytelse**: OBO-tokenveksling dedupliseres per request, SAML-identitetsoppslag caches i 10 minutter, `/statistikk` hentes parallelt med Suspense per fane, og bruksfordelingens caching er herdet.
- **Statistikk**: Ukedagsbevisst prognose i graf og backend, «Meg og team» omdøpt til «Team i Nav» med personlig statistikk fjernet, histogram for AI-kredittbruk lagt til, og redundant faktureringsbanner fjernet.
- **nav-pilot**: Uendelig re-exec-løkke ved auto-oppdatering rettet, Claude Sonnet 5 lagt til i modellkatalogene, og oppsettveiviseren spør om auto-launch.

## 2026-06-30

### nav-pilot CLI — robusthet, proxy og credential-varsling
- **Robust ferskhetssjekk & feilcooldown**: Lagt til 1-times cooldown på mislykkede API-søk mot GitHub for å hindre rate-limiting feilsirkler under ustabile nettverk eller offline-tilstand.
- **Proxy- og tokenstøtte**: Lagt til støtte for system-proxy (`http.ProxyFromEnvironment`) og bruk av `GITHUB_TOKEN` for ferskhetssjekk- og oppdateringskall. Økt sjekktimeout fra 2s til 5s for bedriftsnettverk.
- **Installasjons-fallback (rtk_setup)**: Implementert fallback-installasjon fra Brew til `curl` dersom Homebrew feiler. Lagt til hjelpetekster til Stderr ved mislykket hook-initialisering.
- **Feilsikker bakgrunnskloning**: Lagt til `GIT_TERMINAL_PROMPT=0` under kloningskall for å unngå henger i ikke-interaktive bakgrunnsjobber, samt mer presis parsing av git-feilmeldinger for manglende SSH-nøkler eller autentiseringstokens.
- **Atomisk skriving av cache**: Forbedret `WriteCache` til å skrive atomisk via midlertidig fil og rename for å forhindre korrupt JSON ved avbrudd.
- **Sikkerhetskonfigurasjon (.gitignore)**: Git-ignorerer lokal teststatus (`.local/`) for å forhindre innsending av testdata.

## 2026-06-26

### Refaktorering og struktur
- **Rotmappe-migrering**: Flyttet alle customization-artefakter (agents, skills, instructions, prompts) til prosjektets rotmappe for ryddigere struktur (#330).

### nav-pilot CLI — UX, robusthet og auto-oppdatering
- **nav-pilot doctor**: Erstattet den gamle `status`-kommandoen med en ny, handlingsrettet `doctor`-kommando som kjører systemhelsesjekk og gir proaktive, kopierbare løsninger på manglende kontekst, feil i konfigurasjon eller cplt sandbox-tilganger (#308, #231).
- **Sandbox-konfigurasjon**: Implementert konfigurasjon for `cplt` sandbox og synlighet i den interaktive oppsettveiviseren for å enklere sette riktig prosjektmodus (#309).
- **Auto-oppdatering og varsler**: CLI-en tilbyr nå en interaktiv oppgradering for utdaterte nav-pilot-installasjoner, samt støtte for `auto_update`-konfigurasjon (7-dagers terskel).
- **UX-løft**: Lagt til animerte spinnere under nettverkskall, `did-you-mean`-forslag ved skrivefeil i kommandoer/flagg, og tydeligere exit-koder dokumentert i `--help` (#331).
- **Konflikthåndtering i sync**: `sync --dry-run` evaluerer nå konflikter for å automatisk rydde opp dem som allerede er løst manuelt.
- **Kloning fra tilpasset `--source`**: Fikset en bug der `sync --source` feilet. CLI-en fanger nå opp og formaterer `git clone` feilmeldinger (stderr) slik at nettverks- og referansefeil blir tydelige.
- **Sikkerhetskontekst (Sandbox)**: Dokumentert `cplt` sandbox-restriksjoner eksplisitt i `nav-pilot.agent.md` og globale `AGENTS.md` for å forhindre filtilgang utenfor gjeldende workspace (#326).

### Standardisering av språk og innhold
- **Språkstandardisering**: Body-tekst i instruksjoner og skills er harmonisert til engelsk, mens metadata i YAML frontmatter forblir på norsk (#179).
- **Tilgjengelighet slanket**: Trimmet `accessibility.instructions.md` kraftig for å unngå dobbeltoppføring. Dype WCAG-remedieringer og ARIA-eksempler er samlet i `@accessibility`-agenten (#167).
- **Konsistente agentnavn**: Navngivning av flere agenter er strømlinjeformet (f.eks. ble `auth-agent` til `@auth` og `code-review-agent` til `@code-review`), inkludert manifest-oppdateringer og oppdaterte prompt-eksempler.

### Telemetri og test
- **Separasjon av bakgrunnssynk**: Telemetri skiller nå `auto_sync` fra manuelle `sync`-kall for å gi mer nøyaktig bruksstatistikk.
- **Test-robusthet (Bats)**: Bypasset macOS `noexec`-restriksjoner på `/tmp` ved å peke Bats tmp-katalog til workspace-mappen.
- Diverse opprydding etter grundige kodegjennomganger (Adversarial Review og Opus).

---

## 2026-06-09

### nav-pilot design — canonical spec og delegasjonsklarhet

- La til `docs/nav-pilot-design.md` som canonical design/spec for nav-pilot
- Festet at `@nav-pilot` er koordinator, mens spesialistagenter er leaf-only
- La inn matrise som skiller Copilot-CLI-tips fra nav-pilot-praksis
- Oppdaterte referanser fra README og agentprompt til å peke på design-docen

### nav-pilot CLI — `export opencode` token-optimalisering

`export opencode` genererte tidligere én AGENTS.md med alle instruksjoner (~4 600 linjer) som ble lastet inn av OpenCode på hvert prompt.

Ny oppførsel: instruksjoner med spesifikk `applyTo`-pattern (`.go`, `.kt`, `.tsx`, osv.) eksporteres som individuelle filer til `.opencode/instructions/<name>.md` og refereres lazily fra AGENTS.md. Globale instruksjoner (uten pattern eller `applyTo: "**"`) forblir inline i AGENTS.md.

Resultat: AGENTS.md er nå ~300 linjer i stedet for ~4 600 — ca. 85 % tokenreduksjon per prompt. Språk- og rammeverk-spesifikk kontekst lastes kun når relevant.

---

## 2026-06-05

### nav-pilot og my-copilot — sync, launch og hash-anchor

### nav-pilot CLI — launch og sync

- Launch sender ikke lenger tvungne `--mode plan` / `--effort high`; agent-default og bruker-overstyring gjelder
- `sync` viser konfliktfiler tydelig i output/JSON når de blir hoppet over
- `sync --apply` rydder `conflict`-status når filer faktisk matcher source
- Forbedret auto-sync feedback per scope (repo/user)
- Egen source-resolve-strategi for sync + utvidet testdekning

### my-copilot — navigasjon og prising

- La til hash-anchor scrolling ved hard reload (`HashAnchorScroll` i root layout)
- Robust håndtering av ugyldig URL-fragment (fallback når `decodeURIComponent` feiler)
- Synket model-pricing-data (inkludert oppdatert dato og modelliste)

---

## 2026-06-04

### nav-pilot web docs — README-audit og riktig integrasjon

Fjernet README-embedding fra `/nav-pilot/docs` og gjorde i stedet en målrettet innholdsjustering i web docs:

- La til lenke til primær dokumentasjon: `https://ki-utvikling.nav.no/nav-pilot`
- La til lenke til changelog i ressursseksjonen
- Beholdt docs-siden som kuratert dokumentasjon i stedet for å rendere README rått
- Fjernet duplikatinnhold i leseflyten:
  - «Første kommandoer» ble erstattet med pekere til «CLI-referanse»
  - «Livssyklus» ble fjernet fra TOC og erstattet med kort krysslenke til relevante seksjoner

### README — slanket for web docs-først

`docs/README.nav-pilot.md` er redusert til en kort inngangsside:

- kort beskrivelse + lenke til online docs
- minimale komme-i-gang-kommandoer
- korte bidragsyter-pekere

### my-copilot — nav-pilot README inn i web docs

Denne tilnærmingen ble testet og deretter erstattet samme dag med kuratert docs-side (se «README-audit og riktig integrasjon» over).

- Rå README-embedding i docs-side er fjernet

### nav-pilot — ekstra kosttiltak fra oppdatert research

La inn flere håndhevbare tiltak som ikke var eksplisitt dekket tidligere:

- **Ask-before-Agent gate**: små fakta-/syntaksoppgaver skal løses i Ask/chat før Agent Mode vurderes
- **Cache-hygiene**: unngå bytte av instruksjoner/verktøy midt i tråd; start ny tråd for stabil cache
- **Fasebudsjett**: grovt tokenbudsjett per fase i full-tier oppgaver
- **Governance hooks**: følg Opus-eskaleringer, Agent Mode-andel og kosttrend per oppgavetype

### Dokumentasjonsstruktur for kosteffektiv Copilot-bruk

Dokumentasjonen ble tydelig delt i fire lag for mindre sprik mellom policy og formidling:

- `.github/agents/nav-pilot.agent.md` er styrende policy (fasit)
- `docs/README.nav-pilot.md` er operativ playbook for bruk
- `docs/nav-pilot-changelog.md` er sporbar endringslogg
- `apps/my-copilot/src/app/praksis/sections/cost-optimization.tsx` er pedagogisk praksis-side

### nav-pilot — routing-policy for lavere tokenkost

La til en eksplisitt routing-policy i `nav-pilot.agent.md` for å redusere unødvendig kontekst og modellkost:

- Bruk `@research-agent` først til kartlegging og faktainnhenting
- Hold `@nav-pilot` til orkestrering, syntese og fasekontroll
- Eskaler kun smale høyrisiko-delproblemer til `@nav-pilot-opus`
- Deleger domenespørsmål til spesialistagenter i stedet for å laste alt i én kontekst

### nav-pilot — operasjonelle kostnadsvern på routing

For å dekke hele research-bildet (7 tiltak) ble policyen skjerpet med håndhevbare regler:

- **Model-gate for Opus**: Krever irreversibel/høyrisiko-beslutning + uløst tradeoff + smalt delproblem før eskalering
- **Eksplisitt «never escalate»** for rutineoppgaver (boilerplate, enkel wiring, lint/test-tolkning)
- **Konteksthygiene**: én oppgave per tråd, bruk `/compact` ved handoff, `/clear` ved problembytte
- **Tool-first** som standard: deterministiske kommandoer før bred LLM-tolkning
- **MCP/tool-pruning**: hold aktive verktøysett smale for aktuell oppgave
- **Output-disiplin**: kort output som standard, utvid bare ved reelle tradeoffs/sikkerhetsbehov

---

## 2026-06-03

### nav-pilot — sterkere fasedisiplin og rød-sone-håndhevelse

Analyse av agent-interaksjoner viste at nav-pilot for ofte hoppet over fasestopp og leverte kode uten å deklarere rød sone. Omskrevet fasemaskinen og rød/grønn-sone-systemet med 8 konkrete forbedringer. Fil: 492 → 336 linjer (−32 %).

**Fasedisiplin:**

- **PHASE INTEGRITY** — ny seksjon øverst i filen som eksplisitt forbyr fase N+1-innhold i samme svar som fase N-utput. `Phase gates override concise-by-default.`
- **Scope-klassifisering** — erstatter vage small/medium/large med eksplisitt tre-nivå-tabell (trivial/compressed/full) med entydige kriterier per nivå. Default til Full ved tvil, PII, auth, ny API-kontrakt eller nytt dataflyt
- **Kontekst-anker** — etter 5+ svar begynner nav-pilot med én linje som oppsummerer fase, nøkkelbeslutninger og åpne spørsmål. Kompenserer for LLM-konteksttap i lange samtaler
- **FORBIDDEN-regel** — eksplisitt klausulen «generating Phase N+1 content in the same response as Phase N output» fjernet tvetydighet

**Rød/grønn sone:**

- **🔴 Rød-sone-deklarasjon som punkt #10** — obligatorisk i alle Fase 2-planer. Inkluderer begrunnelse per element, ikke bare en liste. Grønn-sone-elementer er «les gjennom før merge», ikke «trygt»
- **Explain-back-regel** — etter at utvikleren implementerer rød-sone-kode ber nav-pilot dem forklare den tilbake. Mer effektivt enn stub-blokkering alene (basert på Anthropic-studie 2026)
- **Blindsoner #1/#2 alltid-obligatorisk** — personvern og tilgangskontroll merket ⚠️ uavhengig av scope-tier når endringen berører brukerdata eller nye endepunkter

**Filstruktur:**

- Fjernet «Slik bruker du meg»-seksjon (25 linjer, lav atferdsverdi)
- Kondensert HikariCP-kodeblokk og Nais YAML-eksempler til kompakte tabeller/bullets
- Forkortet Opus-eskaleringseksjon til kjernetriggere
- Vedlikeholder `<operating_loop>` XML-tag og 6 høykonsekvens-mønster inline

---

## 2026-05-28

### `$terse-mode` — native output-komprimering

Ny skill som kutter output-tokens med ~65 % uten å miste teknisk substans. Inspirert av Caveman (66k ⭐) men native i nav-pilot — ingen tredjepartsavhengighet.

- **Tre nivåer**: lett (profesjonell kort), normal (fragmenter), ultra (telegrafisk)
- **Auto-clarity**: slår seg av for sikkerhetsvarsler og destruktive handlinger
- **Persistens**: anti-drift-instruksjon hindrer modellen i å falle tilbake til verbose
- **Norsk ordliste**: dropper fyllord som «Selvfølgelig», «La meg», «Absolutt»
- Aktivér med `$terse-mode` i Copilot Chat

Tilgjengelig i alle 5 samlinger (kotlin-backend, frontend, nextjs-frontend, fullstack, platform).

### `$security-owasp` — OWASP 2025 med Java og Node.js

Oppdatert sikkerhetsskill med OWASP Top 10 2025, utvidet fra kun Go/Kotlin til også Java og Node.js/Next.js. Flyttet fra always-on instruksjon (21 KB per interaksjon) til on-demand skill.

### nav-pilot oppførsel — kortere svar og smartere kontekst

- **Concise by default**: nav-pilot gir nå korte, handlingsrettede svar som standard. Si «forklar» for detaljer.
- **Infer-and-confirm**: Infererer kontekst fra repo-filer i stedet for å stille mange spørsmål. Stiller maks 2–3 spørsmål ved store/uklare oppgaver.
- **Skill-routing**: Anvender automatisk riktig Nav-kunnskap (auth, Nais, Kafka, sikkerhet) basert på kontekst — brukeren trenger ikke huske skill-navn.

---

## 2026-05-19

### Agenter vs skills — deprecation og erstatning

Deprecerte 5 agenter som manglet verktøytilgang (ga kun råd, kunne ikke gjøre endringer). Erstattet med tilsvarende skills som fungerer som kunnskapspakker inne i agenter som *har* verktøy.

Refs: #255

### Bevisst AI-bruk — kompetansebevaringsrammeverk

Ny instruksjon (`deliberate-ai-use.instructions.md`) basert på Anthropic-, MIT- og Nav-forskning. Klassifiserer oppgaver i grønn sone (AI-egnet) og rød sone (lær manuelt først). Inkluderer «generer-så-forstå»-mønster.

Refs: #187

---

## 2026-05-14

### `nav-pilot init` — scaffold repo-lokal Copilot-konfig

Ny kommando som genererer `AGENTS.md`, `.github/copilot-instructions.md` og `.github/copilot-review-instructions.md` tilpasset repoet ditt.

### Code review-instruksjoner

Ny `code-review.instructions.md` som gir Copilot Code Review kontekst om Nav-konvensjoner (sikkerhet, Nais, auth, infrastruktur).

---

## 2026-05-07

### nav-pilot CLI forenklet til 4 kommandoer

Breaking change: CLI-en ble forenklet fra mange subcommands til `install`, `update`, `init` og `ignore`. Synk skjer nå automatisk ved install/update.

### `--sync`-flagg og default all-scopes

`nav-pilot install` synkroniserer nå alle scopes (agents, skills, instructions, prompts) som standard. Bruk `--sync=false` for å hoppe over.

---

## 2026-04-28

### `$readme-review` skill

Ny skill for strukturell gjennomgang og generering av README-er tilpasset prosjekttype (tjeneste, bibliotek, monorepo, naisjob).

### Norsk tekstkvalitets-instruksjon

Ny `norwegian-text.instructions.md` som aktiveres for alle `.md`-filer. Sikrer klart språk, riktige fagtermer og konsistent norsk.

### AI Credits-kalkulator

Ny side på ki-utvikling.nav.no som estimerer månedlig Copilot-kostnad basert på modellvalg og bruksmønster.

---

## 2026-04-22

### `nav-pilot ignore` — undertrykk påminnelser

Ny kommando for å undertrykke «nye elementer tilgjengelig»-påminnelser for spesifikke filer eller scopes.

### `/fleet` og Git worktrees-artikkel

Dokumentasjon om hvordan bruke Copilot `/fleet` med Git worktrees for parallell utvikling.

---

## 2026-04-20

### Språkstrategi — engelsk for maskininstruksjoner, norsk for brukersynlig output

Forskning (Multi-IF-benchmark) viser at norske instruksjoner gir 5–15 % lavere etterlevelse i LLM-er, og forverres per samtalesvng. nav-pilot hadde inkonsekvent språkblanding — det verste alternativet.

Refaktorert `nav-pilot.agent.md` med hybridstrategi:

- **Engelsk** (maskininstruksjoner): Fasemaskin-tabell, blindsoner, arketyper, beslutningstrær, review-perspektiver, leveransesjekkliste, vanlige mønstre, feilsøking, boundaries
- **Norsk** (brukersynlig output): Fasehoder, tilstandsfot, sjekkpunkt-mal, delegeringsmal, «Slik bruker du meg»-eksempler, @forfatter-delegering
- Eksplisitt språkdirektiv lagt til: «Respond to users in Norwegian. All internal instructions in this file are in English for optimal adherence.»
- Formalisert språkpolicy i AGENTS.md under «Customization Language»

Refs: #179

### Fasepersistens — nav-pilot husker hvem den er

Nav-pilot mistet fasebevissthet og persona under lange samtaler fordi instruksjonene ble erklært én gang og deretter begravd av konteksthistorikk. Omskrevet kjernemekanismen:

- **Operasjonsløkke** — erstatter engangs `<response_format>` med en 5-stegs løkke som kjøres på hvert svar: bestem fase → faseoverskrift → kun fase-tillatt arbeid → sjekkpunkt ved overgang → tilstandsfot
- **Tilstandsfot** — kompakt one-liner på slutten av hvert svar som sporer gjeldende fase, ferdige faser, nøkkelbeslutninger og åpne spørsmål. Fungerer som minneoppfrisking uten token-oppblåsing
- **Fasemaskin-tabell** — eksplisitte inn-/ut-kriterier per fase slik at modellen har et oppslagsverk for hva som er tillatt
- **Tilbakerullingsregel** — ny informasjon som konflikter med tidligere beslutninger tvinger eksplisitt retur til tidligste berørte fase
- **Utvidet Fase 3 (Review)** — fra 9 linjer til fullstendig 4-perspektiv-review med 16 konkrete spørsmål og strukturert output-mal med dom (Godkjent / Godkjent med endringer / Tilbake til Fase 2)
- **Delegeringskontrakt** — «deleger kun delproblemet, aldri hele samtalen. Gjenoppta alltid kontroll med oppsummering.» Forhindrer at spesialistagenter overtar
- **Nummererte blindsoner** — 10 punkt med krav om dekningsrapport i Fase 1-sjekkpunkt
- **Fasedisiplin i Boundaries** — nye ✅ Always og 🚫 Never-regler for faseoverskrift, tilstandsfot og fase-hopping

### Installasjonsskript — immunisert mot releasekaperng

Skills-release `v0.1.0` kapret GitHubs «Latest»-flagg og brakk `install.sh` (404 på nav-pilot-binærer):

- **Installasjonsskript** — byttet fra `/releases/latest` API til å filtrere `/releases` etter `nav-pilot/`-tag-prefiks. Nå immun mot andre release-strømmer i monorepoet
- **Skills-workflow** — lagt til `--latest=false` på `gh release edit` slik at skills-releaser aldri stjeler Latest-flagget
- **GitHub** — manuelt gjenopprettet nav-pilot-release som Latest

### Adopsjonssiden — 4 nye kategorier og verktøysammenligning

Surfacet 4 manglende skannerkategorier og lagt til verktøysammenligningsgraf:

- **BQ-views** — 4 nye kolonner i `v_adoption_summary`, `v_team_adoption`; 2 nye UNION ALL-seksjoner i `v_customization_details`
- **Nye kategorier**: copilot_setup_steps, agentic_workflows, agents_skills, nav_pilot_state
- **Gruppert CustomizationTypeChart** — delt i Copilot/Agentic/nav-pilot-seksjoner med filtrering av tomme grupper
- **Ny ToolComparisonChart** — Copilot vs Cursor vs Claude vs Windsurf sammenligning
- **TopCustomizationsChart** — 2 nye kategorier med automatisk filtrering av tomme kort

---

## 2026-04-17

### Eksport til OpenCode (`nav-pilot export opencode`)

- Ny `export`-kommando som transformerer `.github/`-artefakter til `.opencode/`-format for [OpenCode](https://github.com/anomalyco/opencode) og [oh-my-openagent](https://github.com/code-yeongyu/oh-my-openagent)
- Skills kopieres 1:1 (OpenCode støtter `name`, `description`, `license`, `metadata` nativt)
- Prompts → commands (fjerner `name` fra frontmatter, OpenCode utleder fra filnavn)
- Agenter → agents (erstatter frontmatter med `description` + `mode: subagent`, dropper VS Code-spesifikke `tools`)
- Instruksjoner + `copilot-instructions.md` → samlet `AGENTS.md` med seksjonsoverskrifter
- Støtter `--user` for global installasjon til `~/.config/opencode/`
- Gjenbruker eksisterende flagg: `--dry-run`, `--force`, `--target`, `--ref`, `--source`
- Blokkerer skriving til eksisterende `.opencode/` med mindre `--force` brukes
- YAML-safe sitering av beskrivelser med spesialtegn (`:`, `#`, etc.)

---

## 2026-04-14

### Bruker-hjemmappe-installasjon (`--user`)

- Nytt `InstallScope`-konsept (repo vs bruker) — `--user`-flagg installerer agenter, skills og instruksjoner til `~/.copilot/`
- Bruker-scope fungerer på tvers av alle repoer uten å modifisere hvert enkelt
- Instruksjoner installeres til `~/.copilot/.github/instructions/` og krever `COPILOT_CUSTOM_INSTRUCTIONS_DIRS` (kun Copilot CLI)
- nav-pilot setter env-variabelen automatisk ved lansering av cplt i interaktiv modus
- Ny `nav-pilot env`-kommando for shell-profilintegrasjon: `eval "$(nav-pilot env)"`
- Prompts støttes kun i repo-scope
- Scope-felt i state-fil for å forhindre kryssforurensning

### TUI-oppgradering

- Erstattet nummererte tekstvalg med TUI-velgere (opp/ned + enter)
- Bruker `charmbracelet/huh` for Select-komponenter
- Interaktiv modus spør om repo- eller bruker-installasjon

### Feilrettinger

- Fikset uendelig «update available»-loop forårsaket av foreldet manifest-versjon
- `cplt`-lansering bruker `-- --agent` passthrough, `copilot` bruker `--agent` direkte
- `--user`-flagg avvises for kommandoer som ikke støtter det
- `--user --target .` oppdages korrekt som ugyldig (mutually exclusive)
- Symlink-beskyttelse i state-skriving dekker nå hele mappekjeden
- Versjon lagres i à-la-carte-installasjoner (`nav-pilot add`)
- Korrupt bruker-state viser advarsel i stedet for å ignoreres stille

### Refaktorering

- `installSingleFile`, `countFileIntegrity`, `shortSHA` ekstrahert som gjenbrukbare hjelpere
- All state-validering går gjennom `InstallScope`
- Deduplisert installasjonslogikk

---

## 2026-04-13

### Nye artefakter

- **threat-model** (skill) — STRIDE-A trusselmodellering for NAIS-mikrotjenester med dataflytdiagram, tillitsgrenser og risikovurdering
- **java-to-kotlin** (skill) — Rammeverk-bevisst Java→Kotlin-migrering (Spring→Ktor, JPA→Kotliquery, JUnit→Kotest)
- **performance** (instruksjon) — Core Web Vitals-mål for Next.js/Aksel-apper med server components, datafetching og bundle-optimalisering
- **security-owasp** (instruksjon) — OWASP Top 10:2025 kodemønstre med ✅/❌-eksempler i både Kotlin og Go

### Integrasjonsaudit

Gjennomført kryssreferanseaudit av alle 4 samlinger. Lagt til `Related`-tabeller i 7 instruksjoner og 1 agent for bedre kobling mellom artefakter:

- `performance` → @aksel-agent, @observability-agent, aksel-spacing, playwright-testing
- `security-owasp` → security-review, @security-champion, @auth-agent, threat-model
- `database` → flyway-migration, @nais-agent, postgresql-review
- `kotlin-ktor` → kotlin-app-config, ktor-scaffold, @auth-agent, @nais-agent, @observability-agent
- `accessibility` → @accessibility-agent, @aksel-agent, playwright-testing
- `nextjs-aksel` → @aksel-agent, @accessibility-agent, performance, aksel-spacing
- `golang` → @nais-agent, @observability-agent, security-owasp, @security-champion
- `security-champion` (agent) → threat-model, security-review, security-owasp

### Forbedrede instruksjoner

- **performance** — utvidet med Core Web Vitals-mål, server components, bundle-optimalisering
- **nextjs-aksel** — utvidet med middleware, streaming, server actions
- **accessibility** — redusert overlapp med Aksel-instruksjoner, fokus på WCAG-regler
- **golang** — utvidet med pgx, sqlc, slog, Chainguard Docker
- **kotlin-ktor** — Spring Boot-deprekering og Ktor-migreringsråd, Koin/Arrow-kt

### @forfatter-integrasjon

- Lagt til språkvask som siste del-steg i nav-pilot Fase 4
- Delegerer til `@forfatter` for klartspråk, anglismer og mikrotekst

### Omdøping

- `go-nais` → `golang` (instruksjon)
- `go-service` → `golang-service` (prompt)

### Copilot CLI-integrasjon

- `nav-pilot` CLI finner nå både `cplt` og `copilot` i PATH
- Interaktiv agentvelger — velg blant installerte agenter
- Starter Copilot CLI med `--agent`-flagg

### Tre innganger til nav-pilot

Dokumentert tre måter å bruke nav-pilot på:
- **Terminal**: `copilot --agent nav-pilot`
- **VS Code / JetBrains**: `@nav-pilot` i chat
- **nav-pilot CLI**: interaktiv modus med agentvelger

### Feilrettinger

- Opprettet manglende `ktor-scaffold/metadata.json`
- Refaktorert `threat-model` SKILL.md fra 613→487 linjer (ekstrahert kodeeksempler til `references/`)
- Rettet metadata-skjema i 3 instruksjoner (`displayName`/`domain`/`tags`/`examples`)
- Rettet Nynorsk→Bokmål i docs-tabeller og metadata
- Rettet ugyldig import-syntaks i performance-instruksjon
- Fjernet ubrukt `launchCopilot()`-funksjon
- Skills lint: 0 feil

### Samlingsoversikt

| Kategori | Antall |
|----------|--------|
| Agenter | 12 |
| Skills | 22 |
| Instruksjoner | 13 |
| Prompts | 7 |
| Samlinger | 4 |
