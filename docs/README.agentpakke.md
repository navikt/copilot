# 📦 Agentpakke

En **agentpakke** er et innholdsrepo som beskriver seg selv for nav-pilot: agenter, skills, instruksjoner og prompts, pluss et manifest som sier hvilke klienter pakka støtter, hvilke agenter som er primære personaer, og hvor innholdet ligger i repoet. Et team kan dermed distribuere sitt eget sett med tilpasninger uten å forke nav-pilot eller sende PR til `navikt/copilot`. Brukerne installerer pakka med `nav-pilot install --source <owner>/<repo>`.

Manifestet ligger på `.nav-pilot/agentpakke.json` i agentpakke-repoet og er hele kontrakten mellom repoet og binæren. Den er publisert som JSON Schema i [`cli/nav-pilot/schemas/agentpakke-v1.json`](../cli/nav-pilot/schemas/agentpakke-v1.json), og nøyaktig samme fil er kompilert inn i nav-pilot-binæren, så repoets egen CI-lint og nav-pilot validerer mot identiske bytes.

Dette dokumentet er for team som lager en agentpakke. Interndesignet, altså hvordan manifestet trådes gjennom install og sync, står i [cli/nav-pilot/DESIGN.md](../cli/nav-pilot/DESIGN.md).

## Repoform

```
<agentpakke-repo>/
├── .nav-pilot/
│   └── agentpakke.json      # manifestet
├── agents/                  # layout.agents:        <navn>.agent.md
├── skills/                  # layout.skills:        <navn>/SKILL.md
├── instructions/            # layout.instructions:  <navn>.instructions.md
└── prompts/                 # layout.prompts:       <navn>.prompt.md eller <navn>/
```

Katalognavnene er ikke låst. `layout` peker på hvor innholdet faktisk ligger, og nav-pilot leser kun der. Filnavnkonvensjonene inne i katalogene er derimot låst, fordi det er dem nav-pilot bruker til å finne og navngi artefaktene. Agentfiler må åpne med en `---`-avgrenset YAML-frontmatter som minst deklarerer `name` og `description`.

## Feltreferanse

Generert fra `cli/nav-pilot/schemas/agentpakke-v1.json`. Ukjente felt på alle nivåer er tillatt (`additionalProperties: true`) og ignoreres av konsumenten.

### Rotnivå

| Felt | Type | Påkrevd | Betydning |
| --- | --- | --- | --- |
| `contractVersion` | string, `^1(\.\d+)?$` | ja | Manifestets kontraktversjon. Denne binæren støtter major `1`. |
| `name` | identifikator, `^[a-z][a-z0-9-]*$` | ja | Pakkas identitet. Dette er navnet brukeren installerer (`nav-pilot install <name>`), og det som registreres som installasjonens `collection` i state. |
| `description` | string | ja | Én linje, vises i `nav-pilot list`. |
| `clients` | objekt, minst én nøkkel | ja | Én oppføring per klient. Se under. |
| `owner` | objekt: `repo` (`^[^/]+/[^/]+$`), `team` | nei | Kun attribusjon. Kilden til en installasjon er der manifestet ble klonet fra, ikke `owner.repo`. |
| `layout` | objekt: `agents`\*, `skills`\*, `instructions`, `prompts` | ja for Tier 1 | Repo-relative stier til innholdskatalogene. `agents` og `skills` er påkrevd når `layout` først er til stede. |
| `policies` | objekt: `opencodePermissions` | nei | Peker på policy-artefakter. Sti-sjekkes i dag, materialiseres ikke ennå. |
| `profiles` | objekt: `dir`, `default` | nei | Katalog med launch-profiler og navnet på standardprofilen (`<dir>/<default>.json`). Sti-sjekkes i dag, brukes ikke ennå. |
| `provenance` | objekt: `base` (`repo`\*, `digest`\*), `overlays[]` (`component`\*, `version`\*) | nei | Opphav for komponert innhold. Ren metadata, nav-pilot verifiserer ikke digest. |
| `minNavPilotVersion` | string, `YYYY.MM.DD-HHMMSS[-sha]` | nei | Minste nav-pilot-versjon. Se [Versjonsgate](#versjonsgate). |

### `clients.<klient>`

Nøkkelen er en identifikator (`^[a-z][a-z0-9-]*$`). Klientene denne binæren kan starte er `copilot`, `opencode` og `pi`. Andre nøkler er *ikke* feil. De ignoreres, og klienten er bare utilgjengelig her.

| Felt | Type | Påkrevd | Betydning |
| --- | --- | --- | --- |
| `primaryAgents` | array av string, minst ett element | påkrevd for Tier 1 | Agentene som er valgbare som primære personaer i klienten. Alt annet i `agents/` materialiseres som subagent. Har oppføringen `payloads`, ligger rosteret i stedet på hver payload ([`payloads.<kontekst>.primaryAgents`](#clientsklientpayloadskontekst)), og feltet her **leses ikke**. Blir det stående, valideres det fortsatt som et velformet ikke-tomt array, men fjern det heller, se [korreksjonen](#én-korreksjon-før-første-konsument-august-2026). |
| `compatibility` | string | nei | Støttet klientversjon som **range** (f.eks. `">=1.18.20,<2"`), ikke en eksakt pin. |
| `defaultModel` | string | nei | Modell-id, eller literalen `"inherit"` (ikke pin noe, arv provider- eller sesjonsvalget). |
| `defaultContext` | identifikator | nei | Hvilken payload-kontekst som startes som standard. Uten verdi: `"full"`. |
| `payloads` | objekt, minst én nøkkel | nei | Tier 2: én oppføring per kontekst (i dag `full`, `focused`). At dette feltet er til stede, er det som gjør oppføringen til Tier 2. |

### `clients.<klient>.payloads.<kontekst>`

Kontekstnøkkelen er en identifikator, og ukjente kontekstnøkler ignoreres på samme måte som ukjente klientnøkler.

| Felt | Type | Påkrevd | Betydning |
| --- | --- | --- | --- |
| `path` | repo-relativ sti | ja | Katalogen som holder det ferdigbygde payload-treet. |
| `primaryAgents` | array av string, minst ett element | ja | Agentene som kan startes i denne konteksten. **Første element er kontekstens standardpersona**, den launch velger. Rosteret hører til payloaden fordi det er payload-treet som bærer agentene: to kontekster for samme klient kan inneholde ulike agenter, og da kan ikke klientoppføringen svare for begge. Det finnes ingen fallback til klientnivået. |
| `manifest` | repo-relativ sti | nei | Overstyrer payload-manifestets plassering. Uten dette feltet er konvensjonen `<path>/manifest.json`. |

## Tier utledes av form

En klients konformanstier deklareres ikke. Den utledes av formen på oppføringen, slik at den ikke kan drifte fra innholdet:

- **Tier 1 (layout):** klientoppføringen har ingen `payloads`. nav-pilot materialiserer innholdet selv fra stiene i `layout`, som da må finnes.
- **Tier 2 (payloads):** klientoppføringen har `payloads`. nav-pilot verifiserer og stager ferdigbygde trær. Ingen `layout` kreves for slike klienter.

En pakke kan blande: `copilot` på Tier 2 og `opencode` på Tier 1 i samme manifest er gyldig, forutsatt at `layout` finnes for Tier 1-klienten. Men nav-pilot starter ikke Tier 2-halvdelen av en slik pakke i dag (se [Begrensninger](#begrensninger-i-dag)).

Mangler `layout` mens en *kjent* klient er Tier 1, avvises manifestet:

```
client(s) opencode declare no payloads, which makes them Tier 1, but the manifest has no "layout".
Add a layout with agents and skills paths, or declare payloads to make them Tier 2
```

## Ignorer-ukjent, og hva som feiler lukket

Regelen har to halvdeler, og de gjelder samtidig:

- **Ukjente konstruksjoner ignoreres.** Ukjente klientnøkler, ukjente kontekstnøkler i `payloads`, og ekstra felt på ethvert nivå gjør ikke manifestet ugyldig. En eldre binær tilbyr bare ikke det den ikke kjenner navnet på. Dermed kan økosystemet vokse uten å ugyldiggjøre manifester som allerede er ute.
- **Feilformede *kjente* konstruksjoner feiler lukket.** En `primaryAgents` som er tom eller mangler der den kreves (Tier 1-oppføring eller Tier 2-payload), en `layout` som mangler for en Tier 1-klient, en `contractVersion` med en major nav-pilot ikke implementerer, en `minNavPilotVersion` nav-pilot ikke kan sammenligne: alt stopper. Et repo som *har* et manifest må ha et gyldig et. Install avbrytes før første filoperasjon, og ingenting skrives delvis.

Et repo helt uten `.nav-pilot/agentpakke.json` er ikke en feil. Det behandles som en legacy-samlingskilde (`collections/<navn>/manifest.json`) akkurat som før.

## Stiregler

Alle repo-relative stier i manifestet (`layout.*`, `policies.*`, `profiles.dir`, `payloads.*.path`, `payloads.*.manifest`) valideres etter samme regler:

1. Ikke tom, ikke omgitt av whitespace.
2. Skråstrek forover (`/`). Backslash avvises, også på Linux.
3. Relativ. Absolutte stier og `~`-prefiks avvises.
4. Ingen escape: en sti som normaliseres til `..` eller starter med `../` avvises.
5. **Symlinker må holde seg inne i checkouten.** En sti som *er* en symlink avvises, og det gjør også en sti som når ut av repoet via en symlinket foreldrekatalog. Hele stikjeden løses opp, ikke bare siste ledd. Det samme gjelder enkeltfiler i `agents/`.

Regel 1 til 4 er tekstlige og kjøres når manifestet parses. Regel 5 krever checkouten på disk og kjøres av `nav-pilot validate` og av install før noe skrives.

## Innholdssjekker på disk

Utover manifestets form sjekker `nav-pilot validate` (og install) innholdet:

- Hver `layout`-katalog som er deklarert, finnes og er en katalog.
- `layout.agents` inneholder minst én `*.agent.md`, og hver av dem åpner med YAML-frontmatter.
- Hver Tier 2 `payloads.<kontekst>.path` finnes som katalog.
- Hver Tier 2-payload har et payload-manifest, enten `<path>/manifest.json` eller filen `manifest` peker på. nav-pilot nekter å stage en payload uten manifest.
- Hvert payload-tre stemmer eksakt med payload-manifestet sitt: `schemaVersion` er 1, hver oppføring i `files` har en gyldig sha256 (64 tegn, små bokstaver) og modus `0644` eller `0755`, hver deklarert fil finnes med riktig innhold, og treet inneholder ingen fil manifestet ikke lister. Symlinker og andre ikke-vanlige filer avvises. Modus sjekkes som kjørbit på kilden, slik at en streng `umask` i checkouten ikke gir falske avvik. Eksakt modus settes og verifiseres når payloaden stages.
- En payload kan ikke selv ha en fil som heter `manifest.json` i rota. Staging skriver payload-manifestet dit i det stagede treet, så navnet er opptatt. Dette kan bare oppstå når `manifest` peker på en fil utenfor payload-katalogen. Ligger manifestet på konvensjonsplassen, er det manifestet.
- `policies.opencodePermissions` finnes som fil hvis den er satt.
- `profiles.dir` finnes som katalog, og `<dir>/<default>.json` finnes hvis `profiles.default` er satt.

Alle funn rapporteres samlet, ikke bare det første.

**Hva verifiseringen gir, og ikke gir.** Payload-manifestet ligger i samme repo som payloaden og er skrevet av samme part, så kjeden er selvsignert: digestene sier ingenting om at innholdet er vurdert av noen andre. Det de gir, er integritet og konsistens. Treet nav-pilot stager er byte-identisk med det pakkerepoets egen CI så, og ingenting kan smugles inn mellom verifisering og kopiering. Mot en pakkeforfatter med vond vilje er det tre andre ting som er grensa: at brukeren selv har valgt kilden, at en staget launch kjører i cplts sandkasse (med `--no-audit`, som lukker parent-side audit), og at ingenting av brukerens eget klientinnhold blandes inn. Det er samme tillitsmodell som `brew install`: «verifisert» betyr uendret, ikke godkjent.

## Versjonsgate

`minNavPilotVersion` må skrives på nav-pilots releaseformat: `YYYY.MM.DD-HHMMSS`, eventuelt med build-sha (`2026.09.01-120000-a1b2c3d`). Andre formater avvises framfor å ignoreres. nav-pilot kan ikke sammenligne dem, og å godta dem ville stille av akkurat den gaten manifestet ba om.

Kjører brukeren en eldre release enn kravet:

```
agentpakke "grillmester" requires nav-pilot 2026.09.01-120000 or newer, but this binary is
2026.08.01-100000-abc1234. Run `nav-pilot update` (or reinstall via Homebrew) and try again
```

Utviklingsbygg (`dev`) er unntatt gaten, slik at lokalt arbeid på en agentpakke ikke blokkeres.

## Kompatibilitet

Kontrakten er bygget for at en agentpakke skal kunne vokse uten å knekke brukere på eldre binærer.

**Trygt å legge til uten bump av `contractVersion`:**

- Nye klientnøkler i `clients`, og nye kontekstnøkler i `payloads`. Eldre binærer ignorerer dem.
- Nye felt på ethvert objekt i manifestet.
- Flere `primaryAgents`, endret `defaultModel`, endret `compatibility`, ny `provenance`.
- Nytt innhold i `layout`-katalogene.

**Krever varsomhet, men ikke bump:**

- Å flytte innhold ved å endre `layout`-stier. Eksisterende brukere får det nye innholdet ved neste sync, men repoet må fortsatt validere. `nav-pilot export opencode` støtter foreløpig bare kanoniske stier (se [Begrensninger](#begrensninger-i-dag)).
- Å endre `name`. Eksisterende installasjoner er registrert under det gamle navnet i state, så nav-pilot kjenner dem ikke igjen som samme pakke. `nav-pilot install <nytt-navn>` blir en ny installasjon i samme scope.
- Å heve `minNavPilotVersion`. Eldre binærer blokkeres bevisst, med en melding som sier hva de skal gjøre.

**Krever bump av `contractVersion`:**

- Å endre betydningen av et eksisterende felt.
- Å gjøre et tidligere valgfritt felt påkrevd, eller å fjerne et felt konsumenter leser.
- Å endre en konvensjon konsumenten har innebygget, for eksempel hvor payload-manifestet ligger.

### Én korreksjon før første konsument (august 2026)

`primaryAgents` for Tier 2 ble **flyttet** fra klientoppføringen til hver payload. Etter reglene over krever det bump av `contractVersion`: et tidligere valgfritt felt ble påkrevd et nytt sted, og et felt konsumenter leste sluttet å gjelde. `contractVersion` står likevel på `"1"`, og deprekeringsvinduet under ble **ikke** brukt. Grunnen er verdt å skrive ned, slik at ingen senere leser dette som et brudd på reglene:

- Det fantes ikke én eneste konsument da endringen ble gjort. Tier 2 kunne ikke installeres, ingen bruker hadde en Tier 2-kilde, og det eneste manifestet som eksisterte var [navikt/grillmester](https://github.com/navikt/grillmester) sitt, hvis eiere var med på å utforme korreksjonen. Den var avtalt, ikke påtvunget.
- Med én konsument ville dette krevd bump til major 2 **og** de 90 dagene. Maskineriet finnes for å beskytte utrullede konsumenter. Med null konsumenter ville en bump bare tvunget binæren til å støtte to schemaer for ingen.
- Vinduet gjelder fra det øyeblikket det finnes en konsument til. Neste gang noe tilsvarende dukker opp, er svaret bump og vindu. Dette er ikke et presedens for at «kontrakten er ung nok».

Årsaken til selve flyttingen står i [beslutningene](agentpakke-beslutninger.md).

**Flytter du et eksisterende Tier 2-manifest over, gjør begge deler:** legg `primaryAgents` på hver payload, *og* fjern det fra klientoppføringen. Blir begge stående, validerer manifestet også på en nav-pilot som er eldre enn korreksjonen, og den leser rosteret fra klientoppføringen, altså feil agent for enhver kontekst som har sitt eget. Sett samtidig `minNavPilotVersion` til første versjon som har korreksjonen, så blir en for gammel binær en tydelig feilmelding i stedet for en stille feil launch.

### Deprekeringsvindu

Motsatt vei har nav-pilot en tilsvarende forpliktelse: en fjerning på kontrakten skjer med deprekeringsvindu, ikke i én release. **Vinduet er 90 dager**, fra deprekeringen er kunngjort til fjerningen er merget. nav-pilot slippes ved hver merge til `main`, så et antall releaser er ikke noe å planlegge mot. Kalendertid er det eneste målet en pakkeforfatter har.

Kunngjøringen står der kontrakten står. Den deprekerte konstruksjonen merkes i tabellene over og i [schemaet](../cli/nav-pilot/schemas/agentpakke-v1.json), i samme PR som starter klokka. Vinduet gjelder bare det som krever bump av `contractVersion`. De additive endringene over trenger ingen kunngjøring.

### Å endre kontrakten

Endringen *er* pull requesten som endrer [`agentpakke-v1.json`](../cli/nav-pilot/schemas/agentpakke-v1.json) og dette dokumentet sammen. Schemaet er kompilert inn i binæren, så de to kan ikke drifte fra hverandre. [CODEOWNERS](../CODEOWNERS) avgjør hvem som må godkjenne. Det finnes ingen forslagsmal og intet eget vedtakssteg.

### Når en pakke slutter å validere

nav-pilot feiler lukket, og gjør ikke noe utover det. Et manifest som ikke lenger laster, stopper enhver kommando som resolver kilden (`install`, `add`, `sync`) før første filoperasjon ([Ignorer-ukjent](#ignorer-ukjent-og-hva-som-feiler-lukket)). `nav-pilot validate` er unntaket: den resolver bevisst uten å feste manifestet, slik at et ugyldig manifest rapporteres som funn framfor som en resolve-feil. Innhold som allerede er installert står urørt og virker videre.

nav-pilot varsler ingen om det, deaktiverer ingenting automatisk, og har ingen forestilling om en forlatt pakke. Å oppdage det i tide er pakkerepoets egen CI-jobb ([Validering i CI](#validering-i-ci)).

## Eierskap og vedlikehold

| Hva | Eier | Forpliktelse |
| --- | --- | --- |
| Kontrakten, altså schemaet og dette dokumentet | nav-pilot-vedlikeholderne, `@navikt/copilot` per [CODEOWNERS](../CODEOWNERS) | Reglene over: additivt uten bump, fjerning med 90 dagers vindu. |
| En agentpakke | eierne av repoet manifestet klones fra | Å holde pakka validerende mot en støttet `contractVersion`, og å svare på det som meldes mot pakkerepoet. |

`owner` i manifestet er attribusjon, ikke tilgangsstyring. Å publisere en agentpakke er å si at innholdet er ment å kjøre på andres maskiner enn dine egne.

**Referansepinning.** Der nav-pilot pinner en referanseimplementasjon for differensialtesting, står SHA-en i kildekommentarene i `cli/nav-pilot/internal/agentpakke`. Å flytte pinnen er en bevisst og reviewbar endring avtalt med eierne av pakka det måles mot, ikke stille drift.

**Kompromittert innhold.** En byttet eller endret fil i et payload-tre verifiserer ikke, og stopper der ([Innholdssjekker på disk](#innholdssjekker-på-disk)). Tier 1-innhold er ikke digest-bundet, og der er git-historikken i pakkerepoet hele sporbarheten. Det finnes **ingen tilbakekalling**: nav-pilot har ingen liste over trukne pakker eller digester, og når ikke en installasjon som allerede står på en maskin. Meld fra i pakkerepoets eget issue-spor, og til `@navikt/copilot` for kontrakten eller nav-pilot selv. [SECURITY.md](../SECURITY.md) i rota beskriver sikkerhetsarkitekturen for copilot-tjenestene og har ingen varslingskanal for agentpakker.

## Validering i CI

```bash
nav-pilot validate [--source <owner/repo>|<absolutt sti>] [--ref <ref>] [--json]
```

Kommandoen kjører hele konformanssjekken mot en kilde: manifestet mot schemaet, semantikken (kontraktversjon, versjonsgate, tier og layout, stiregler) og innholdet på disk. Den er ment for agentpakke-repoets egen CI.

> Ikke å forveksle med `nav-pilot config validate`, som sjekker brukerens egen `~/.nav-pilot/config.toml`.

`--source` tar enten `owner/repo` (klones) eller en **absolutt** sti til en checkout. Relative stier og `.` avvises. Uten `--source` faller kommandoen tilbake på kildepresedensen (`--source` > `source`-nøkkelen i config > `navikt/copilot`), og lokal auto-deteksjon slår kun til for et repo som har en `collections/`-katalog, altså ikke for en vanlig agentpakke. I CI skal du derfor peke eksplisitt på checkouten:

```yaml
- uses: actions/checkout@v7
- run: nav-pilot validate --source "$GITHUB_WORKSPACE"
```

### Exit-koder

| Kode | Betydning |
| --- | --- |
| `0` | Kilden er konform (eller er en gyldig legacy-samlingskilde). |
| `1` | Ett eller flere funn, eller en feil (kilden kunne ikke resolves, manifestet er ikke gyldig JSON, …). |

Ved funn skriver kommandoen problemene og avslutter med `Error: agentpakke validation failed` på stderr. Det gjelder også med `--json`: JSON-en skrives til stdout først, exit-koden er fortsatt `1`.

### `--json`

| Felt | Type | Innhold |
| --- | --- | --- |
| `command` | string | Alltid `"validate"`. |
| `source` | string | Kildeetiketten (`owner/repo`, stien, eller `navikt/copilot`). |
| `sha` | string | Commit-sha for checkouten som ble validert. |
| `kind` | string | `"agentpakke"` hvis manifestet finnes, `"legacy"` hvis kilden ikke har noe manifest. |
| `valid` | bool | `true` når `problems` er tom. |
| `notes` | array av string | Informasjon, ikke funn: manifeststi, pakkenavn og kontraktversjon, klientliste med tier (og hvilke klientnøkler denne binæren ignorerer), `minNavPilotVersion`. |
| `problems` | array av string | Ett element per funn, samme tekst som i den menneskelige utskriften. |

```json
{
  "command": "validate",
  "kind": "agentpakke",
  "notes": [
    "manifest: .nav-pilot/agentpakke.json",
    "agentpakke: grillmester (contract version 1)",
    "clients: copilot (tier 2), opencode (tier 2), zed (unknown to this nav-pilot — ignored)",
    "minNavPilotVersion: 2026.09.01-000000-0000000"
  ],
  "problems": [
    "clients.copilot.payloads.full.path references \"plugin\", which does not exist in the agentpakke repo"
  ],
  "sha": "a1b2c3d",
  "source": "navikt/grillmester",
  "valid": false
}
```

## Slik installerer brukerne pakka

```bash
nav-pilot install --source <owner>/<repo> <pakkenavn>
```

Et repo med manifest **erstatter** samlingsmodellen: det tilbyr nøyaktig ett installerbart navn, nemlig manifestets `name`. `nav-pilot list --source <owner>/<repo>` viser det under overskriften `Agentpakke in <kilde>:`. Kjører brukeren `nav-pilot install --source <owner>/<repo>` uten navn i et repo, hopper den interaktive flyten over samlingsvelgeren og installerer pakka, siden det ikke er noe å velge mellom.

Er navnet på pakka også navnet på en agent i den (vanlig), vinner pakka. Agenten kan fortsatt installeres alene med `--type agent`.

**Kilden huskes.** Etter en vellykket install med eksplisitt `--source` lagres verdien som `source` i `~/.nav-pilot/config.toml`:

```
✓ Saved navikt/grillmester as your source in /Users/<deg>/.nav-pilot/config.toml.
  Future runs use it without --source; clear it with nav-pilot config set source "".
```

Presedensen er `--source` > `source` i config > `navikt/copilot`.

**Valget er per scope.** Repo-scope (`.github/` i et repo) og bruker-scope (`~/.copilot`, `--user`) huskes hver for seg, og hvert scope registrerer kilden sin i sin egen state. Ulike repoer kan derfor følge ulike agentpakker.

**Kryss-kilde-vakt.** nav-pilot blander ikke innhold fra to agentpakker i én installasjon. Er scopet installert fra én kilde mens config peker på en annen, stopper install med valgene skrevet ut:

```
the repo scope was installed from navikt/grillmester, but your configured source is navikt/copilot.
nav-pilot will not silently mix content from two agentpakker into one install.

  Keep this scope on its current source:  nav-pilot install --source navikt/grillmester <name>
  Switch this scope to the new source:    nav-pilot install --source navikt/copilot <name>
  Or clear the persisted source:          nav-pilot config set source ""
```

Et eksplisitt `--source` er overstyringen. Da har brukeren sagt hvilken kilde som skal inn i hvilket scope. Vakten gjelder også de interaktive flytene, så det å velge scope i en meny er ingen vei rundt den.

**Sync.** `nav-pilot sync` bruker kilden som er registrert for scopet, ikke den konfigurerte, og sier fra når de to er ulike. Et scope som ble installert *før* nav-pilot begynte å registrere kilder har ingen registrert kilde. Da synkes det fra den konfigurerte kilden, det skrives én informasjonslinje om det (ikke i `--json`), og kilden festes i state først **etter** en vellykket sync. En sync som feiler registrerer ingenting. Fra neste kjøring gjelder den vanlige vakten. Uten `source` i config, og med eksplisitt `--source`, adopteres ingenting.

```
ℹ The repo scope predates source tracking; syncing from navikt/copilot and recording it as this scope's source.
```

Stiformede kilder sammenlignes symlink-oppløst, så en symlink og checkouten bak den regnes som samme kilde.

## Slik starter brukerne klienten fra en Tier 2-pakke

Peker kilden på en agentpakke som deklarerer `payloads` for klienten, kjører nav-pilot klienten fra en verifisert revisjon av pakka som ligger på maskinen:

```bash
nav-pilot --client copilot                                # konteksten manifestet setter som standard
nav-pilot --client copilot --payload-context focused      # en annen deklarert kontekst
```

- **`--payload-context <id>`** velger kontekst. Uten flagget brukes `defaultContext` fra klientoppføringa (`full` når feltet mangler). En kontekst pakka ikke deklarerer gir en feil som lister opp de som finnes. Det er ingen config-nøkkel for dette: den varige standarden er manifestets eget felt. Flagget er ikke `--context`, som er Copilots long-context-nivå og er uendret. De to er ortogonale og kan stå på samme kommandolinje.
- **Revisjonen ligger på maskinen, er immutabel og valgt av et install-steg.** `nav-pilot install --user <navn>` verifiserer pakka, materialiserer hver deklarerte kontekst under `~/.nav-pilot/pakker/<eier>-<repo>/<sha>/` (repo-id-en småskrevet, slik nav-pilot ellers sammenligner den) og pinner SHA-en. Senere launcher leser den katalogen og kloner ingenting. Startes en payload-only kilde som ikke er installert, pinner første launch den på samme måte og sier fra med én linje.
- **Verifisering før hver launch.** Det pinnede treet re-verifiseres eksakt mot payload-manifestet (digest og modus) før klienten startes. En fil som er endret, fjernet eller lagt til etter at revisjonen ble materialisert, stopper launchen. Feiler noe av dette, starter ingenting, og en Tier 2-launch faller aldri tilbake på Tier 1-veien.
- **Pinnen flyttes av `nav-pilot sync`, ikke av å starte klienten på nytt.** Uten `--apply` rapporterer sync hvilken revisjon som er tilgjengelig. Med `--apply` verifiseres og materialiseres den nye revisjonen, og pinnen bytter. De to siste revisjonene beholdes, eldre fjernes. Er den pinnede revisjonen fjernet fra disk, sier sync det framfor å melde «up to date», og `--apply` bygger den opp igjen (`Restored …`). Sync oppdaterer den kilden scopet er pinnet til: peker du den mot et annet repo, nekter den og ber deg gjøre byttet med `install`.
- **Revisjonene fjernes av `nav-pilot uninstall`, som navngir hver katalog den sletter, også i tørrkjøring.** De frigjøres også, uten utskrift, av en vanlig Tier 1-install som skriver over pinnen i samme scope, siden pin-staten da er borte og ingenting senere ville funnet trærne igjen.
- **Utvikler du pakka lokalt, pinnes den ikke.** Er `source` en absolutt sti (`nav-pilot config set source /sti/til/pakke`), materialiseres og verifiseres payloadene på nytt ved hver launch, så en endring i arbeidstreet er med på neste start, og bare den revisjonen launchen bruker beholdes. `nav-pilot install` av en lokal Tier 2-kilde nektes: det finnes ingen immutabel revisjon å pinne, og meldingen navngir install fra repoet i stedet. Dette er flyten for å utvikle en pakke.
- **opencode** startes med `OPENCODE_CONFIG_DIR` mot den pinnede revisjonen. Den delte `~/.config/opencode/opencode.json` verken leses eller skrives på denne veien. OTel går fortsatt som miljøvariabler.
- **copilot** startes med `--plugin-dir <revisjon>` og personaen kvalifisert med pakkenavnet: `--agent <pakke>:<agent>`.
- **cplt er påkrevd, og må være minst den gjennomgåtte baselinen.** En staged launch gir klienten et verifisert tre inne i sandkassen. Uten cplt starter ingenting (`brew install navikt/tap/cplt`). En eldre cplt enn `2026.08.17-062831`, eller en cplt nav-pilot ikke får lest versjonen av, avvises også, med `brew upgrade cplt` i feilen.
- **`compatibility` håndheves før launch.** Deklarerer klientoppføringa et versjonsområde, prober nav-pilot klienten og avviser en versjon utenfor området. Både en mislykket probe og uleselig versjonsutdata er fatalt: et område som ikke kan håndheves, er ikke håndhevet.
- **Modell.** `defaultModel: "inherit"` sender ingen `--model` i det hele tatt. En konkret verdi sendes med. En modell brukeren har pinnet selv vinner over begge.

## Begrensninger i dag

Dette er statusen i milepæl 1. Alt under er kjent og planlagt, ikke feil:

- **Tier 2 installeres bare i brukerscope**, med `nav-pilot install --user <navn>` ([slik pinnes revisjonen](#slik-starter-brukerne-klienten-fra-en-tier-2-pakke)). Et forsøk i repo-scope avvises med en begrunnelse som navngir `--user`. Installasjonen skriver ingen filer brukeren ser: `nav-pilot list` viser 0 elementer for en payload-only pakke, men lister payload-kontekstene per klient, også for klienter denne nav-pilot ikke kan starte, siden install materialiserer dem alle. `nav-pilot sync` rapporterer revisjoner framfor fildiffer. Første launch pinner en payload-only kilde som ikke er installert, med mindre brukerscopet allerede har en installasjon som ville mistet filene eller revisjonene sine. Da nekter launchen og ber om en eksplisitt `install`.
- **En blandet pakke starter ikke Tier 2-klienten sin.** Har manifestet både `layout` og `payloads`, pinnes pakka ikke i denne releasen, og en launch av en klient som deklarerer `payloads` stopper med en feil som sier nettopp det. Alternativet ville vært å starte den fra `layout`-innholdet, altså stille gi brukeren noe annet enn payloaden manifestet deklarerer. Tier 1-klientene i samme pakke installeres og startes som før. Skal Tier 2-klienten kunne startes nå, må pakka være payload-only. Får en pakke som allerede er pinnet hos brukere en `layout` oppstrøms, nekter `sync` å oppdatere pinnen over den endringen og ber om en ny `install`, mens launcher fortsetter å lese den pinnede revisjonen.
- **Alle deklarerte kontekster materialiseres**, også de brukeren aldri starter, og kontekster som deler innhold lagrer det én gang hver.
- **En kilde som er en absolutt sti pinnes ikke, og kan ikke installeres.** En pinnet installasjon krever et repo med en immutabel revisjon.
- **`policies`, `profiles` og `provenance` er deklarasjoner uten virkning ennå.** Stiene sti-sjekkes, men nav-pilot skriver hverken opencode-permissions eller launch-profiler ut fra manifestet (M3), og sjekker ikke `provenance`-digesten mot innholdet.
- **`compatibility` håndheves bare på den stagede stien.** Legacy-stien, altså Tier 1-innhold materialisert inn i brukerens egen klient, leser ikke feltet.
- **`nav-pilot export opencode` støtter bare kanoniske stier.** Export leser `agents/`, `skills/`, `instructions/` og `prompts/` direkte. En agentpakke som legger innholdet et annet sted avvises av export med en forklaring, framfor å skrive et tomt `.opencode/`-tre.
- **Ferskhetssjekken i rot-TUI-en beskriver bare standardkilden.** Scope som ikke kommer fra `navikt/copilot` hoppes over der, fordi release-feeden til nav-pilot bare sier noe om standardkilden.

## Eksempler

### Minimal Tier 1

Alt en pakke trenger for å være installerbar i dag:

```json
{
  "contractVersion": "1",
  "name": "grillmester",
  "description": "Agentteam for tydelig leveranse, design og produktarbeid",
  "owner": { "repo": "navikt/grillmester", "team": "Team eSyfo" },
  "clients": {
    "copilot": {
      "primaryAgents": ["grillmester"]
    },
    "opencode": {
      "primaryAgents": ["grillmester", "barista"],
      "defaultModel": "inherit"
    }
  },
  "layout": {
    "agents": "agents",
    "skills": "skills",
    "instructions": "instructions",
    "prompts": "prompts"
  }
}
```

Repoet ser da ut som treet under [Repoform](#repoform), med `agents/grillmester.agent.md` og `agents/barista.agent.md` som de to agentfilene.

### Tier 2 med ferdigbygde payloads

Formen en komponert pakke har. Den installeres i brukerscope og pinnes til en revisjon (se [Begrensninger](#begrensninger-i-dag)):

```json
{
  "contractVersion": "1",
  "name": "grillmester",
  "description": "A Copilot agent team for clarified software delivery, design, and product work with portable progressive skills.",
  "owner": { "repo": "navikt/grillmester", "team": "Team eSyfo" },
  "clients": {
    "copilot": {
      "compatibility": ">=1.0.79,<2",
      "defaultModel": "gpt-5.6-sol",
      "defaultContext": "full",
      "payloads": {
        "full": {
          "path": "plugin",
          "primaryAgents": ["grillmester", "barista", "designer", "doctor-who"]
        },
        "focused": {
          "path": "targets/copilot-cli-focused-v1",
          "primaryAgents": ["barista", "grill-inspektor"]
        }
      }
    },
    "opencode": {
      "compatibility": ">=1.18.20,<2",
      "defaultModel": "inherit",
      "payloads": {
        "full": {
          "path": "targets/opencode-v1",
          "primaryAgents": ["grillmester", "barista", "designer", "doctor-who"]
        },
        "focused": {
          "path": "targets/opencode-v1-focused",
          "primaryAgents": ["barista", "grill-inspektor"]
        }
      }
    }
  },
  "provenance": {
    "base": { "repo": "navikt/copilot", "digest": "sha256:abc" },
    "overlays": [{ "component": "grillmester", "version": "0.3.0-rc.8" }]
  },
  "profiles": { "dir": "profiles/opencode", "default": "hybrid" },
  "minNavPilotVersion": "2026.09.01-000000-0000000"
}
```

Hver payload-katalog må ha et payload-manifest: `plugin/manifest.json`, `targets/opencode-v1/manifest.json`, og så videre, eller en `manifest`-overstyring i oppføringen.

Legg merke til at rosterne skiller seg per kontekst. `full` starter `grillmester`, mens `focused`-payloadene bare inneholder `barista` og `grill-inspektor` og derfor starter `barista`. Det er nettopp derfor `primaryAgents` ligger på payloaden og ikke på klientoppføringen.

## Se også

- [JSON Schema: `cli/nav-pilot/schemas/agentpakke-v1.json`](../cli/nav-pilot/schemas/agentpakke-v1.json), kontrakten selv
- [Beslutninger](agentpakke-beslutninger.md), hvorfor nav-pilot oppfører seg som den gjør: bevisste avvik, åpne spørsmål og aksepterte begrensninger
- [nav-pilot](README.nav-pilot.md), CLI-et som konsumerer agentpakker
- [Samlinger](README.collections.md), legacy-modellen en agentpakke erstatter
- [Sync](README.sync.md), hvordan installert innhold holdes oppdatert
- [cli/nav-pilot/DESIGN.md](../cli/nav-pilot/DESIGN.md), internt design, sømmer og migrasjonsplan



