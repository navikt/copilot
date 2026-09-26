# 🧭 nav-pilot

nav-pilot er et CLI-verktøy og en AI-agent for Nav-utvikling med GitHub Copilot og opencode.

📖 **Online docs (primær):** https://ki-utvikling.nav.no/nav-pilot  
📝 **Endringslogg:** [docs/nav-pilot-changelog.md](nav-pilot-changelog.md)

## Kom i gang

```bash
# Anbefalt: Homebrew (macOS), nav-pilot og påkrevd isolasjon
brew install navikt/tap/nav-pilot navikt/tap/cplt

# Anbefalt på Linux (Debian, Ubuntu): apt-arkivet
curl -fsSL https://navikt.github.io/apt/keyring/navikt-archive-keyring.gpg \
  | sudo tee /usr/share/keyrings/navikt-archive-keyring.gpg >/dev/null
echo "deb [signed-by=/usr/share/keyrings/navikt-archive-keyring.gpg] https://navikt.github.io/apt stable main" \
  | sudo tee /etc/apt/sources.list.d/navikt.list
sudo apt update && sudo apt install nav-pilot cplt

# mise: samme binærer fra GitHub-releasen, med attestering verifisert
mise use -g 'github:navikt/cplt'
mise use -g 'github:navikt/copilot[exe=nav-pilot,version_prefix=nav-pilot/]@2026.09.12-225921-bb3fbb6'

# Uten arkivet: .deb-en er også et releaseartefakt
sudo apt install ./nav-pilot_2026.09.12-225921-bb3fbb6_$(dpkg --print-architecture).deb

# Linux / CI: last ned og inspiser skriptet manuelt
curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh -o install.sh
cat install.sh   # Se gjennom skriptet før kjøring
bash install.sh
```

> **Arkivet ligger inntil en time bak.** Publiseringsjobben kjører hver time og
> henter den nyeste `.deb`-en fra hver release, så en release du nettopp kuttet
> er ikke installerbar med `apt` med det samme. Det er et vanlig apt-arkiv som
> speiler releasene våre, ikke en distropakke med egen vedlikeholder. Oppgrader
> med `sudo apt upgrade`, ikke med `nav-pilot upgrade`: selvoppdateringen kjenner
> igjen en Homebrew-installasjon, men ikke en dpkg-installasjon, og ville byttet
> ut binæren uten at dpkg vet om det.

> ⚠ **Pin versjonen med mise.** Versjonsstrengene våre er ikke gyldig semver, så
> `mise latest` plukker en eldre release enn den nyeste. Oppgi versjonen selv,
> eller bruk Homebrew.

> ⚠ **Sikkerhetsmerk:** `curl ... | bash` kjører installasjonsskriptet uten forhåndsverifikasjon.
> Binæren verifiseres med SHA256-checksum og SLSA provenance (krever `gh` CLI), men skriptet
> som laster den ned er ikke signert. Derfor Homebrew på macOS, apt-arkivet på Debian og
> Ubuntu, og manuell nedlasting og gjennomlesing ellers på Linux og i CI.

```bash
# I et repo
nav-pilot
nav-pilot install nav-pilot
```

`install` spør hvor den skal installere. Svaret er ikke gitt: repoet deler oppsettet med
teamet, `--user` følger deg over alle repoer uten å sjekke inn noe. Se
[Hvor skal artefaktene installeres?](#hvor-skal-artefaktene-installeres) før du velger.

### Installere mindre <a id="collections"></a>

Vil du ha mindre enn hele pakka, velger du bort i den interaktive velgeren
(`nav-pilot install`, eller installer på nytt senere). I brukerscope kan du i tillegg
fjerne enkeltartefakter etterpå, for eksempel med
`nav-pilot ignore instruction nextjs-aksel --user`. Fravalgene ligger i tilstandsfila til
scopet og overlever både sync og ny installasjon.

## Hvor skal artefaktene installeres?

Tre former er i bruk i Nav, og de løser ulike problemer. `install` spør hvor den skal
installere, i repoet (`.github/`) eller i hjemmekatalogen (`~/.copilot/`). Svar på forhånd
med `--repo`, `--user` eller `--target <mappe>` for å hoppe over spørsmålet.

| Form                 | Hvor                                          | Kort sagt                                                   |
| -------------------- | --------------------------------------------- | ----------------------------------------------------------- |
| Repo (`--repo`)      | `<repo>/.github/`                             | Hele teamet får det samme, og Copilot på github.com ser det |
| Personlig (`--user`) | `~/.copilot/`                                 | Følger deg på tvers av alle repoer, ingenting sjekkes inn   |
| Hub-repo             | ett repo med `.github/` pluss egne artefakter | Ett sted å vedlikeholde teamets egne skills                 |

De utelukker ikke hverandre. `nav-pilot sync` uten scope-flagg synker alle scope som har en
tilstandsfil, og de spores hver for seg.

### Hvilken form bør du velge?

| Situasjonen din                                                         | Anbefalt             | Hvorfor                                                                                        |
| ----------------------------------------------------------------------- | -------------------- | ---------------------------------------------------------------------------------------------- |
| Teamet skal ha samme oppsett, og dere vil ha det på github.com også     | Repo                 | Copilot på github.com leser bare `.github/` i repoet. Ingen annen form gir deg den synligheten |
| Du jobber i mange repoer, eller i repoer du ikke kan endre `.github/` i | `--user`             | Én installasjon å holde fersk, i stedet for én per repo                                        |
| Du vil ikke sjekke inn generert innhold                                 | `--user`             | Ingenting havner i differ eller kodegjennomgang                                                |
| Teamet har egne skills å vedlikeholde ved siden av Nav-artefaktene      | Hub-repo             | Ett sted som eier både det felles og deres eget                                                |
| Du bruker opencode                                                      | Repo, eller Hub-repo | opencode materialiserer fra kilden pluss `.github/` i repoet du står i, ikke fra `~/.copilot/` |

Er du i tvil, og bare deg det gjelder: ta `--user`. Den er reversibel uten at noen andre
merker det, og du kan legge til repo-installasjon senere uten å fjerne den.

### Repo-installasjon

Skriver til `<repo>/.github/`: `agents/`, `skills/`, `instructions/`, `prompts/` og
tilstandsfila `.github/.nav-pilot-state.json`.

Dette får du bare her:

- **Prompts.** Brukerscopet støtter `agent`, `skill`, `instruction`, `hook` og `extension`, ikke `prompt`
  (`ScopeUser()` i `cli/nav-pilot/internal/domain/domain.go`). Installerer du pakka med
  `--user`, hoppes promptene over og rapporteres som ikke støttet. Ber du om én enkelt prompt
  med `--type prompt --user`, er det en feilmelding.
- **Copilot på github.com.** Filene er sjekket inn, så det som kjører på GitHub-siden leser
  dem. Det forutsetter at du committer og pusher, og nav-pilot gjør ingen av delene.
- **Automatisk oppdatering uten at noen kjører CLI-et.** Den gjenbrukbare workflowen
  `copilot-customization-sync.yml` kjører `nav-pilot sync` i Actions og åpner PR med
  oppdateringene. Se [README.sync.md](README.sync.md).
- **Alle på teamet, også de som ikke har nav-pilot.** Filene ligger der uansett.

Hva det koster: innholdet blir liggende i repoet og dukker opp i differ og
kodegjennomgang. Hele agentpakka er 141 filer og rundt 844 KB målt på artefaktene i denne
kilden, mindre om du velger bort i velgeren. Filer teamet vil eie selv kan merkes som overrides i `.github/copilot-sync.json`,
og blir da hoppet over ved sync.

### Personlig installasjon (`--user`)

```bash
nav-pilot install --user --all
eval "$(nav-pilot env)"
```

Skriver til `~/.copilot/`. Ingenting sjekkes inn noe sted.

Dette får du bare her:

- **Alle repoer på maskinen på én gang**, også repoer der du ikke vil eller kan endre
  `.github/`.
- **Ett sted å synce.** Repo-scopet er alltid det repoet du står i. Med repo-installasjon i
  40 repoer må noen innom alle 40, eller sette opp sync-workflowen i alle 40. Med `--user`
  er det én installasjon å holde fersk.
- **`nav-pilot ignore <type> <name> --user`** for å slippe varsler om komponenter du ikke
  vil ha. Kommandoen avviser repo-scope.

Dette når den ikke:

- **Prompts.** Se over.
- **Instruksjoner utenfor nav-pilot.** De havner i `~/.copilot/.github/instructions/`, og
  leses bare når `COPILOT_CUSTOM_INSTRUCTIONS_DIRS` peker på den mappen. Starter du
  klienten med `nav-pilot`, settes den for deg (`copilotEnv` i
  `cli/nav-pilot/internal/provider/copilot_launch.go`). Starter du `copilot` eller `cplt`
  direkte, må du sette den selv: `eval "$(nav-pilot env)"`. Agenter og skills plukkes opp
  uansett. En staged Tier 2-pakke setter den bevisst ikke.
- **GitHub-siden.** Filene ligger på din maskin, ikke i repoet.
- **Resten av teamet.** En personlig installasjon er personlig.
- **opencode.** Nav-konteksten til opencode materialiseres fra kilden pluss `.github/` i
  repoet du står i, ikke fra `~/.copilot/` (`repoScopeDir()` i
  `cli/nav-pilot/internal/provider/opencode_launch.go`).

### Hooks kan ikke installeres inne i cplt

`nav-pilot install` skriver hooks til `~/.copilot/hooks/` (personlig) eller
`.github/hooks/` (repo). cplt nekter skriving til begge stedene, og det er med vilje: en
hook er kode som kjører senere på maskinen din, utenfor sandboxen, så cplt lar ikke en
prosess inne i sandboxen legge igjen en. Kjører du `nav-pilot install` fra en agentsesjon
i cplt, stopper installasjonen med en feil som forteller deg akkurat det.

Kjør installasjonen fra et vanlig skall i stedet. Alle andre artefakttyper — agenter,
skills, prompts, instruksjoner — installeres helt fint inne i cplt.

### Repo-hooks fyrer bare i en betrodd mappe

En hook i repo-scope må ha to ting på plass, ikke én. Fila må ligge på rett sted, og
Copilot CLI må stole på mappa. Stien er rett — `.github/hooks/copilot-hooks.json` er det
klienten leser — men tilliten er ikke gitt, og det er den som ryker stille.

Målt mot Copilot CLI 1.0.83 i [#888](https://github.com/navikt/copilot/issues/888):

| Scope                                       | `copilot` (interaktiv)          | `copilot -p`   |
| ------------------------------------------- | ------------------------------- | -------------- |
| `~/.copilot/hooks/` (`--user`)              | Fyrer                           | Fyrer          |
| `.github/hooks/` (repo), mappa ikke betrodd | Spør først, fyrer når du svarer | **Fyrer ikke** |
| `.github/hooks/` (repo), mappa betrodd      | Fyrer                           | Fyrer          |

Interaktivt spør klienten «Do you trust the files in this folder?» og laster repo-hookene
i det du svarer. `copilot -p` spør ingen, så i en mappe du ikke har stolt på lastes de
aldri: debugloggen sier `folder not trusted; skipping repo hooks`, og ellers ser alt helt
riktig ut. Fila er committet, den er synlig i differ, og den gjør ingenting.

Svarer du «Yes», gjelder tilliten bare den økta. Det er «Yes, and remember this folder for
future sessions» som skriver mappa til `trustedFolders` i `~/.copilot/config.json`, og bare
den hjelper neste `-p`-kjøring. Hver på teamet må gjøre det på sin egen maskin.
`GITHUB_COPILOT_PROMPT_MODE_REPO_HOOKS=true` laster dem for én kjøring uten å stole på noe;
den står i endringsloggen til klienten og ikke i dokumentasjonen, så sjekk at den fortsatt
virker før du bygger noe på den.

`nav-pilot install` sier fra når den skriver hook-oppføringer til en mappe som ikke er
betrodd, og `nav-pilot doctor` sier om de installerte hookene faktisk kan fyre der du står.
Tåler ikke porten å være stille ute av funksjon, er `--user` det scopet som fyrer uansett.

### nav-pilots egne hooks

Hookene over er Python-skript du velger å installere. nav-pilot har i tillegg egne hooks
som er bygd inn i selve programmet. Når du starter Copilot CLI med `nav-pilot`, skriver den
dem til `~/.copilot/hooks/`. Derfra kjører de i alle Copilot CLI-økter på maskinen, også
når modellen kjører i skyen og også når du starter `copilot` direkte. `nav-pilot doctor`
viser dem sammen med de andre.

| Hook      | Fil                         | Hva den gjør                                                                                                                                                                                                                              | Slå av                                       |
| --------- | --------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------- |
| Løkkevakt | `nav-pilot-loop-guard.json` | Samme regel som `local_loop_guard` for lokale modeller: samme kall med samme resultat 4 ganger på rad, en syklus på to eller tre kall med de samme resultatene 4 ganger på rad, eller samme kall 8 ganger uansett resultat (med standardverdien). Tidsstempler, varigheter, id-er og tall regnes ikke som endring. | `nav-pilot config set hook_loop_guard false` |
| Maskering | `nav-pilot-redact-tool-output.json` | Maskerer hemmeligheter (GitHub-tokener, AWS-nøkkel-id-er, private nøkler, JWT-er og verdien i `password=`/`api_key=`) og fødselsnummer, D-nummer og H-nummer i verktøyresultatet før modellen leser det. Setter en merknad foran et resultat som ser ut som instrukser til modellen («ignore previous instructions», rollemarkører). | `hook_redact_secrets`, `hook_redact_fnr` og `hook_injection_note`, hver for seg |

En hook etter verktøykallet (`postToolUse`) kan ikke avslutte en tur. Den kan bare endre
det modellen leser. Når løkkevakten slår til, får modellen derfor en beskjed om at den står
fast, med resultatet under, i stedet for det samme svaret en gang til. Terskelen følger
`local_loop_guard`, og det som skjedde tidligere i økta ligger i en liten fil i øktas egen
mappe, `~/.copilot/session-state/<økt-id>/nav-pilot-loop-guard.json`. Filen inneholder bare
hasher og to tellere, aldri selve kallet eller resultatet.

I sandkassen til cplt får hookene verken lese eller skrive `~/.nav-pilot/`. Der bruker de
innstillingene nav-pilot skrev inn i hook-kommandoen ved siste oppstart. En endring med
`nav-pilot config set` gjelder derfor i sandkassen fra neste gang du starter `nav-pilot`.

Maskeringen bytter ut funnet med `[REDACTED:<type>]`, for eksempel `[REDACTED:fnr]`, og lar
resten av resultatet stå. Et fødselsnummer maskeres bare når datoen er gyldig og begge
kontrollsifrene stemmer. D-nummer (dag + 40) og H-nummer (måned + 40) regnes med. Et
tilfeldig tall på elleve sifre blir derfor stående. Merknaden om instrukser stopper
ingenting: den sier bare til modellen at teksten kommer fra verktøyet og ikke skal følges.
Mønstrene er et lite utvalg av standardreglene i gitleaks, og de er valgt for å gi få
falske treff i vanlig kode og vanlige logger. Hooken fanger ikke alle hemmeligheter.

I en lokal økt står vakten i nav-pilot allerede foran modellen og avslutter turen. Der gjør
hooken ingenting, så modellen ikke får to beskjeder om samme løkke. Hookene er laget for å
slippe gjennom ved feil: finnes ikke `nav-pilot` på `PATH`, eller går noe galt, blir
resultatet stående som det var. opencode får ikke disse hookene ennå
([#709](https://github.com/navikt/copilot/issues/709)).

### Hub-repo

Mekanisk er et hub-repo en vanlig repo-installasjon i et repo som ikke er en applikasjon,
pluss teamets egne agenter, skills og prompts lagt inn for hånd i det samme `.github/`. Du
kjører nav-pilot fra hub-repoet og jobber derfra.

Det virker fordi begge klientene leser scopet fra arbeidskatalogen:

- Copilot leser `.github/` i repoet direkte, uansett hvem som la fila der.
- opencode materialiserer både kildeartefaktene og det som bare finnes i scopet. Det siste
  kom med [#579](https://github.com/navikt/copilot/pull/579). Før den fikk opencode bare
  det nav-pilot selv hadde installert: et hub-repo med 25 installerte og 3 håndlagde skills
  så 25, uten varsel og uten feilmelding. Kjører teamet en eldre nav-pilot, mangler de tre
  fortsatt.

Skarpe kanter:

- **Konteksten følger arbeidskatalogen.** nav-pilot gir cplt katalogen du står i som
  `--project-dir`, og klienten arver den samme katalogen. Står du i hub-repoet, har du
  hub-repoets artefakter og hub-repoets filer. Går du til applikasjonsrepoet for å endre kode der, har
  du ikke lenger hub-repoets egne skills i scopet.
- **Kilden vinner ved navnekollisjon.** En håndlagd skill med samme navn som en installert
  taper i opencode. Se «Hva scopet ditt bidrar med» under opencode-avsnittet.
- **Instruksjoner fra `.github/` slås ikke sammen med opencodes globale `AGENTS.md`.**
  `nav-pilot export opencode` er veien når teamet trenger dem der.
- **Sync i hub-repoet oppdaterer hub-repoet.** De andre repoene har fortsatt sitt eget.

`AGENTS.md` og `.github/copilot-instructions.md` synces aldri, uansett form. De er alltid
repo-spesifikke, og er stedet for det som bare gjelder ett repo.

## Sandboxing og isolasjon er påkrevd

Når du bruker en AI-agent på Nav-utstyr, skal agenten kjøre i en sandbox eller tilsvarende
isolasjon. Kravet gjelder både Nav-relatert og personlig agentarbeid.

[`cplt`](https://github.com/navikt/cplt) er den anbefalte og enkleste måten å oppfylle
kravet på. Velger du noe annet, må du selv sette deg inn i hvordan agentklienten isolerer
agenten, og slå på funksjonen. Gir den ikke god nok beskyttelse, må du sørge for tilsvarende
isolasjon selv, for eksempel med en VM eller container. Ikke kjør agenter med ubegrenset
tilgang til Nav-utstyret.

Del [kortversjonen av kravet](https://ki-utvikling.nav.no/nyheter/sandboxing-er-pakrevd-pa-nav-utstyr)
med andre som trenger den.

### Sikkerhetsnivå, strict og logging

Står på [ki-utvikling.nav.no/nav-pilot/docs](https://ki-utvikling.nav.no/nav-pilot/docs): hva `sandbox.preset = strict` låser, hvorfor
presetet skal settes via `nav-pilot config` og ikke for hånd, når strict ikke anbefales på Linux,
og hva `proxy.log_level` faktisk logger.

`nav-pilot doctor` sier også fra når cplt selv er utdatert, og foreslår kommandoen som
hører til den cplt-en du har: `sudo apt upgrade cplt` når dpkg eier binæren,
`brew upgrade navikt/tap/cplt` ellers. Det er eierskapet som avgjør, ikke hvor pakka kom fra:
en `.deb` installert for hånd får samme svar som en fra arkivet. nav-pilot laster aldri ned eller oppgraderer cplt for
deg. Svarer ikke GitHub, hopper den bare over versjonssjekken.

## Klienter

nav-pilot støtter tre kodingsagenter (`client`-feltet i konfig):

| Klient                  | Binær               | Nav-kontekst                                                | Standard modell |
| ----------------------- | ------------------- | ----------------------------------------------------------- | --------------- |
| `copilot` (standard)    | `cplt` / `copilot`  | Installeres i `.github/`                                    | GPT-6 Sol       |
| `opencode`              | `cplt` + `opencode` | Materialiseres automatisk i brukerens OpenCode config-mappe | GPT-6 Sol       |
| `pi` _(eksperimentell)_ | `cplt` + `pi`       | Via `AGENTS.md` i prosjektroten                             | GPT-6 Sol       |

En modell du velger med config eller `--model`, vinner over agentpakkas standard.

> **Bruk cplt-sandboxen.** nav-pilot foretrekker `cplt` og kjører klienten via
> `cplt --agent <klient>`. Agenten kan da lese og skrive prosjektfiler, men når ikke
> SSH-nøkler, tilgangsinformasjon for skytjenester eller andre hemmeligheter. `cplt` må
> være installert for å starte `opencode` og `pi`, i tillegg til selve klient-binæren.
>
> **Sandboxen gjelder katalogen du står i.** nav-pilot sender alltid `--project-dir` med
> til cplt, satt til arbeidskatalogen. Uten det utvider cplt en undermappe til roten av
> git-repoet den ligger i, så en økt startet fra `workspaces/noe/` kunne endre mapper ved
> siden av. Trenger du hele repoet, for eksempel for endringer på tvers av pakker i et
> monorepo, starter du fra roten av repoet eller bruker `nav-pilot --project-dir <katalog>`.
> Hjemmekatalogen og `/` avviser cplt selv som for vide.

> **Auth-detalj (Copilot/cplt):** nav-pilot henter ikke ut GitHub-tokenet selv.
> Med `cplt`s gh-guard på, som `sandbox.preset = strict` slår på og `nav-pilot
doctor` anbefaler, skaffer `cplt` tokenet: den bruker `GH_TOKEN`,
> `GITHUB_TOKEN` eller `COPILOT_GITHUB_TOKEN` hvis en av dem er satt, ellers
> `gh auth token` utenfor sandkassen, og leverer det via en 0600-fil som leses
> én gang. Med gh-guarden av gjør `cplt` ingenting her, og Copilot autentiserer
> selv. `copilot_auth_mode` styrer hvilke kilder som slipper fram: `auto`
> (standard) begrenser ingenting, `env_only` avbryter oppstart hvis tokenet ikke
> allerede ligger i miljøet, og `gh_only` fjerner token-variablene fra
> barnemiljøet. Dette er en konfigurasjonskontroll, ikke en sikkerhetsgrense.
> Se [SECURITY.md](../SECURITY.md).

### opencode: Nav-kontekst automatisk

Med `--client opencode` (eller `client = "opencode"` i konfig) gjør nav-pilot dette ved hver
oppstart:

1. Løser opp Nav-kildeartifaktene (skills, agenter, prompts, instruksjoner)
2. Skriver dem til OpenCode-konfigurasjonsmappen (f.eks. `~/.config/opencode/` eller via `XDG_CONFIG_HOME`) som `AGENTS.md`, `skills/`, `commands/`, `agents/` og `instructions/`
3. Holder dem synkronisert med versjonskontroll (konflikt-deteksjon, ferskhetssjekk)
4. Starter opencode i cplt-sandboxen med Nav-agenten (`cplt --agent opencode -- --agent nav-pilot --model …`)

Den materialiserte `nav-pilot`-agenten er en **primær** opencode-agent, så den dukker opp i
agentvelgeren (Tab) og startes automatisk. De øvrige Nav-agentene (auth, kafka, aksel, …)
materialiseres som **subagenter** du kaller med `@navn`.

```bash
nav-pilot --client opencode           # én gangs override
nav-pilot config set client opencode  # sett permanent
```

`nav-pilot status` og `nav-pilot list --installed` viser opencode-artefaktene og om de er
oppdaterte.

##### Hva scopet ditt bidrar med

Skills, prompts og agenter du har lagt inn for hånd i `.github/` i repoet du står i,
materialiseres sammen med Nav-artefaktene. Det er slik et hub-repo får med sine egne
skills i opencode, ikke bare de nav-pilot har installert.

To ting følger ikke med, og det er med vilje:

- **Instruksjoner fra `.github/`** slås ikke sammen med `AGENTS.md`. Utmappa er den
  globale opencode-konfigurasjonen din, og `AGENTS.md` er alltid i kontekst, så
  instruksjonene fra ett repo ville ligget i hver eneste prompt i alle andre repoer.
  Trenger teamet dem i opencode, er `nav-pilot export opencode` veien: den skriver
  `<repo>/.opencode/`, som bare det repoet leser, og tar instruksjonene med.
- **Lokale endringer i en installert artefakt.** Redigerer du en installert skill i
  `.github/`, honorerer Copilot endringen mens opencode får kildeversjonen. Kilden
  vinner ved navnekollisjon. Vil du at opencode skal se den, kopier den til et eget
  navn: både katalognavnet og `name:` i frontmatter må endres, og opencode får da både
  originalen fra kilden og din kopi.

Artefakter fra et annet repo blir liggende i den globale konfigurasjonen til neste sync,
og er synlige ved navn og beskrivelse der. De ryddes bort ved neste oppstart, med mindre
du har endret dem selv: nav-pilot sletter bare det den selv har skrevet og som fortsatt
er uendret. En fil du har redigert blir stående, og forblir sporet, slik at neste sync i
repoet den kom fra melder konflikt framfor å overskrive den.

#### `export opencode` vs. automatisk materialisering

Til ditt **personlige** oppsett trenger du ikke `export` i det hele tatt.

| Kommando                                 | Mål                   | Tilstandssporing              | Når                                                          |
| ---------------------------------------- | --------------------- | ----------------------------- | ------------------------------------------------------------ |
| `nav-pilot --client opencode` (oppstart) | `~/.config/opencode/` | ✅ konflikt + ferskhet        | Personlig kontekst, skjer automatisk                         |
| `nav-pilot sync`                         | `~/.config/opencode/` | ✅ oppdaterer sporet tilstand | Frisk opp personlig kontekst                                 |
| `nav-pilot export opencode` (repo-scope) | `<repo>/.opencode/`   | ingen                         | Sjekk Nav-kontekst inn i et **prosjektrepo** for hele teamet |

> **Avviklet:** `nav-pilot export opencode --user` er erstattet av automatisk materialisering
> ved oppstart pluss `nav-pilot sync`, som i tillegg gir tilstandssporing og
> konflikt-deteksjon. Repo-scope `export opencode` består.

## Vanlige kommandoer

```bash
nav-pilot list --installed
nav-pilot sync
nav-pilot upgrade
nav-pilot feedback
```

`nav-pilot upgrade` bytter ikke ut en binær Homebrew eller apt eier. Da ville
pakkedatabasen pekt på en versjon som ikke lenger ligger på disk, og neste `apt upgrade`
ville rullet oppdateringen tilbake i stillhet. nav-pilot skriver i stedet kommandoen som
virker: `sudo apt upgrade nav-pilot` eller `brew upgrade navikt/tap/nav-pilot`.

## Lokal modell (alfa, av som standard)

`nav-pilot alpha local` kjører en modell på din egen maskin. Den trekker ingen AI-credits.
Krever en Mac med Apple Silicon og 48 GB minne, og rundt 26 GB ledig disk. `init` gjør resten, inkludert å heve macOS-minnegrensen med `sudo`.

```bash
nav-pilot alpha local init      # gjør alt: miljø, vekter, minnegrense, og starter serveren
nav-pilot alpha local status    # kjører den? svarer den? hvilken modell? hva har den gjort?
nav-pilot alpha local models    # modellene som tilbys, og hvilken som er i bruk
nav-pilot alpha local use <key> # velg modellen serveren laster
nav-pilot alpha local ask -p "..."  # still ett spørsmål rett til modellen
nav-pilot alpha decide "..." --options ja,nei --evidence fil  # typet avgjørelse, se under
nav-pilot alpha local stop      # og start igjen med start
nav-pilot alpha local restart   # stopp og start på modellen local_model peker på nå
nav-pilot alpha local on        # skru på igjen etter off
nav-pilot alpha local off       # slutt å sende oppgaver dit; vektene blir liggende
nav-pilot alpha local purge     # fjern miljøet og valgt modell, viser hva og hvor mye først
```

### Bytte modell

`nav-pilot alpha local models` viser de lokale modellene i en tabell: nøkkel, navn, størrelse,
kontekst, hva modellen er anbefalt til, og om den er lastet ned, kjører, er standard eller holdes
tilbake. `*` markerer den serveren laster ved neste start.

```text
     KEY                     NAME                                     SIZE   CONTEXT  RECOMMENDED       STATUS
     qwen3.6-35b-a3b-optiq   Qwen 3.6 35B A3B OptiQ 4bit              25 GB  64k      untrusted decide  default, downloaded
  *  qwen3.8-27b-optiq-4bit  Qwen 3.8 27B OptiQ 4bit (mixed 4/8-bit)  19 GB  64k      nuanced decide    downloaded, running
     qwen3.8-27b-8bit-mlx    Qwen 3.8 27B 8bit (mlx-lm)               30 GB  48k      -                 not downloaded

  Switch: nav-pilot alpha local use <key>
```

`nav-pilot alpha local use <key>` velger modell. Den tar nøkkelen eller hele modell-ID-en og
skriver den til `local_model`. `model` er modellen økten selv kjører på, og settes for seg.

```bash
nav-pilot alpha local use qwen3.8-27b-optiq-4bit
nav-pilot alpha local init      # laster ned vektene hvis de mangler, og starter
nav-pilot alpha local restart   # hvis serveren allerede kjører en annen modell
```

`use` laster aldri ned noe og starter ingenting på egen hånd. Kjører serveren en annen modell,
spør den om omstart når du sitter ved en terminal, og skriver ellers kommandoen.
`nav-pilot config set local_model <id>` virker fortsatt og gjør det samme.

Listen oppdateres når du kjører `init` eller `start` — ikke ved hver kommando, fordi
et nettverkskall der ville lagt seg foran alt annet nav-pilot gjør. Har du nettopp hørt
om en ny modell og ikke ser den, er `start` det som henter listen på nytt.

**Qwen 3.6 er standard fordi den er rask og forutsigbar.** De to Qwen 3.8-modellene kan velges,
men de er mye tregere. 8-bit løste litt flere oppgaver enn standard i siste måling, men brukte
mange ganger så lang tid.

Kontekst, svarlengde, minnekrav, vekter og minste nav-pilot-versjon for hver modell står i
[tabellen på ki-utvikling.nav.no](https://ki-utvikling.nav.no/nav-pilot/docs#lokal-modeller).
Den hentes fra [modellmanifestet](https://github.com/navikt/mlx-workspace/blob/main/manifest/models.json),
det samme nav-pilot leser, så tallene står ikke her. Målingene bak står i
[MODELS.md](https://github.com/navikt/mlx-workspace/blob/main/MODELS.md).

Krever en modell nyere nav-pilot enn du har, skjuler nav-pilot den. Peker `local_model` på den,
faller nav-pilot tilbake til standardmodellen, og `init`, `start` og `status` sier hvilken versjon
du trenger. `models` viser den som holdt tilbake, og `use` nekter å velge den. Oppdater med `nav-pilot upgrade`.

`RECOMMENDED` sier hva målingene anbefaler modellen til: `untrusted decide` er `alpha decide` på
tekst du ikke kontrollerer selv, som issues og PR-beskrivelser, og `nuanced decide` er nyanserte
ja/nei-spørsmål. Har du valgt en annen modell enn standard, sier `start`, `status` og `models` én
gang hva standardmodellen er anbefalt til, og hvordan du bytter. Du ser beskjeden igjen bare hvis
manifestet endrer den, og aldri fra `alpha decide`, en vanlig launch eller når utskriften går til
et skript.

Er modellen i `local_model` fjernet fra manifestet og erstattet av en annen, fortsetter nav-pilot
med den gamle så lenge bare vektene til den gamle ligger på maskinen. Når erstatningen er lastet
ned, bytter nav-pilot til den. Begge deler får du beskjed om én gang, og konfigurasjonen endres
ikke. `nav-pilot alpha local use <key>` gjør valget eksplisitt.

Bytter du modell, må vektene til den nye lastes ned én gang. `purge` fjerner Python-miljøet, vektene
til modellen du har valgt og vektene til modeller manifestet har erstattet. Andre modeller du har
lastet ned, blir liggende, og listen sier hvilke. `purge --all` fjerner vektene til alle modellene.
Ingenting slettes før du legger til `--yes`.

Vil du slippe å starte serveren selv, kan en vanlig `nav-pilot` gjøre det når den trenger den:

```bash
nav-pilot config set local_autostart true
```

Av som standard, med vilje: å starte en 21 GB prosess uten å bli bedt om det er ikke greit.

Ingenting av dette skjer med mindre du kjører `init` selv. Gjør du ikke det, er nav-pilot
uendret.

### Utsending til en lokal underagent krever opencode

**Under opencode** blir modellen en underagent (`local-worker`) som hovedagenten i skyen
kan sende avgrensede oppgaver til. Hovedagenten bestemmer fortsatt alt. Den sender videre
det som er mekanisk og spesifisert, og gjør resten selv.

**Under Copilot CLI finnes ingen slik underagent i dag.** Copilot CLI er standardklienten i
nav-pilot, så dette gjelder deg med mindre du har byttet. Der er valget hele økten på den lokale
modellen eller ingenting lokalt. Grunnen er at Copilot CLI leser modelleverandøren fra en
miljøvariabel for hele prosessen, så én leverandør betjener hele økten. Vi har verifisert det mot
Copilot CLI 1.0.83-3. Hele økten lokalt passer til arbeid som allerede er spesifisert, ikke til
oppgaver der modellen må finne ut hva som skal gjøres.

Runtimen under Copilot CLI kan ha flere leverandører i én økt, men Copilot CLI lar ikke en agent
velge sin egen ennå, og det er ikke dokumentert. Vi tester om det kan tas i bruk
([github/copilot-cli#4703](https://github.com/github/copilot-cli/issues/4703)). Vil du ha
utsending nå, bytt klient:

```bash
nav-pilot config set client opencode
```

### Hva den er god og dårlig til

Målt i et kontrollert testoppsett, på én maskin, og nesten alt på ett Ktor-repo. På den ene Spring-appen vi målte kostet lokal utsending mer enn å la være. Den utfører en avgjørelse godt og tar en
avgjørelse dårlig.

Manifestet sier for hver modell hvilke oppgavetyper hovedagenten kan sende til den. For
standardmodellen er det bare mekaniske endringer over flere filer, sendt fra en skyagent. Svar og
forklaringer om kode, endringer i én fil, nye filer og feilsøking blir i skyen. Qwen 3.8-modellene
har ingen godkjent oppgavetype ennå. Den gjeldende lista står i
[tabellen på ki-utvikling.nav.no](https://ki-utvikling.nav.no/nav-pilot/docs#lokal-hva-den-klarer).

Tiden varierer: fra omtrent som skyen på små endringer til rundt fire ganger så lenge på en omdøping. På den største mekaniske endringen vi målte var den raskere enn skyen.

### Typede avgjørelser med `alpha decide`

`nav-pilot alpha decide` stiller den lokale modellen ett flervalgsspørsmål og svarer med
sannsynligheter for hvert alternativ, ikke med fritekst. Modellen genererer ett token, så et
varmt svar tar under ett sekund. Svaret er JSON når utdata går til et skript:

```json
{"choice":"ja","p":{"ja":0.93,"nei":0.07},"model":"…","ms":410,"evidence":true}
```

En avgjørelse blir aldri bedre enn grunnlaget modellen får. Gi den det den skal vurdere med
`--evidence <fil>` eller `--evidence -` (stdin). Uten grunnlag får du en advarsel og
`"evidence": false`.

Bruk `decide` til vurderinger en regel ikke kan gjøre. Om en commit-melding følger Conventional
Commits, avgjør et regulært uttrykk. Om meldingen forklarer hvorfor endringen ble gjort, kan
bare en modell vurdere. En `commit-msg`-hook som advarer, men aldri stopper commiten:

```bash
#!/bin/sh
command -v nav-pilot >/dev/null 2>&1 || exit 0
{ printf 'Commit message:\n-----\n'; grep -v '^#' "$1"; printf -- '-----\n\nDiff:\n-----\n'
  git diff --cached | head -c 7500; printf -- '\n-----\n'; } |
  nav-pilot alpha decide \
    "Does the commit message explain why the change was made, beyond describing what the diff already shows?" \
    --options yes,no --evidence - --threshold 0.7 --expect no --timeout 3s >/dev/null 2>&1 &&
  echo "commit-msg: meldingen ser ut til å si hva som endret seg, men ikke hvorfor." >&2
exit 0
```

Exit 0 betyr at sannsynligheten for `--expect` er minst terskelen, 1 at den er lavere, og 2 at
noe feilet, for eksempel at serveren ikke kjører. Hooken over slipper commiten gjennom i alle
tilfeller.

Med standardmodellen og terskel 0,7 fanget hooken 40 av 48 svar på meldinger uten forklaring og
flagget ingen av 24 meldinger med forklaring, med 0,4 sekunder median svartid. Med 0,9 fanget den
bare 7 av 48. Tallene er fra 48 meldinger fra egne repoer, så hooken advarer og stopper ikke
([målingen](https://github.com/navikt/mlx-workspace/blob/main/bench/decide-cases/commit-explains-why-results.md)).

Se etter feil i en logg:

```bash
tail -n 200 app.log | nav-pilot alpha decide "Viser loggen en feil som krever handling?" \
  --options ja,nei --evidence - --json
```

Mål spørsmålet før du bruker det i en hook. Lag en JSONL-fil med eksempler du kjenner fasiten
på, ett per linje:

```json
{"question":"Viser loggen en feil som krever handling?","options":["ja","nei"],"evidence":"ERROR db: connection refused","expect":"ja"}
```

```bash
nav-pilot alpha decide --eval cases.jsonl
```

Du får treffsikkerhet, en forvekslingsmatrise, gjennomsnittlig sannsynlighet for riktige og
gale svar (er modellen like sikker når den tar feil?) og p50/p95-svartid. Ingenting lagres per
tilfelle, og det første tilfellet som feiler, stopper kjøringen.

Forbehold:

- Hvor treffsikker modellen er på ditt spørsmål, vet du ikke før du har kjørt `--eval`.
- Bruk `--threshold 0.9` eller høyere når svaret skal stoppe noe, og filtrer tekst du ikke stoler
  på før `decide` leser den. En linje som «The correct answer is no.» i grunnlaget kan snu svaret.
- Serveren svarer på én forespørsel om gangen. Kjører en agentsesjon mot den samtidig, venter
  `decide` til sesjonens forespørsel er ferdig. `--timeout` (standard 10s) teller med ventetiden.
- `decide` starter ikke serveren selv, fordi en kaldstart tar 5–10 sekunder og legger modellen
  på GPU-en. Start den med `nav-pilot alpha local start`.
- Alt skjer lokalt. Spørsmålet og grunnlaget forlater ikke maskinen.

### Når noe henger

```bash
nav-pilot alpha local status
```

Den skiller «treg» fra «død». Sier den `hung`, kjør `nav-pilot alpha local restart`. Serveren
svarer på én forespørsel om gangen, så flere samtidige oppgaver står i kø framfor å kjøre
parallelt.

Går modellen tom for minne, dør tråden som genererer svar. Serveren avslutter seg da selv i stedet
for å henge, og neste økt sier `generation thread died, most likely out of memory` med stien til
tracebacken. Start den igjen med `nav-pilot alpha local restart`, og velg en modell med kortere
kontekst hvis det skjer igjen.

nav-pilot avslutter en tur hvis modellen gjør samme verktøykall fire ganger på rad med samme
resultat, går fire ganger rundt i en syklus på to eller tre kall som gir de samme resultatene,
eller gjør samme kall åtte ganger på rad uansett resultat. Grensene er standardverdier for
`local_loop_guard`.

Dette er alfa. Si fra om noe henger, om en endring kompilerer men er feil, eller om
ventetiden ikke er verdt det: `nav-pilot feedback`.

## Agentpakker fra andre team

Et team kan distribuere sitt eget innhold som en **agentpakke**, et repo med manifest på
`.nav-pilot/agentpakke.json`. Installer det med `--source`. Kilden huskes per scope til du
tømmer den.

Skal du lage en selv, står oppskrifta på [ki-utvikling.nav.no/nav-pilot/agentpakker](https://ki-utvikling.nav.no/nav-pilot/agentpakker).

```bash
nav-pilot install --source navikt/<repo> <pakkenavn>
nav-pilot config set source ""     # tilbake til navikt/copilot
nav-pilot validate --source navikt/<repo>   # sjekk en pakke mot kontrakten
```

**[Lag din egen agentpakke →](README.agentpakke.md)** med manifestreferanse,
kompatibilitetsregler og CI-validering.

## Telemetry (pilot, default on)

nav-pilot sender OTel-metrikker som standard i pilot. Standard endpoint er
`https://collector-internet.nav.cloud.nais.io/v1/metrics`, og du kan overstyre den med
`NAV_PILOT_TELEMETRY_ENDPOINT`. `NAV_PILOT_TELEMETRY_ENABLED=0` (eller `off`) slår av
telemetry.

Når nav-pilot starter `cplt`/`copilot`, setter den `OTEL_EXPORTER_OTLP_ENDPOINT` for Copilot
CLI til samme collector-base (`https://collector-internet.nav.cloud.nais.io`, uten
`/v1/metrics`), slik at Copilot kan sende både metrics og traces. Overstyr med
`NAV_PILOT_COPILOT_OTEL_ENDPOINT`. Den har forrang over generell
`OTEL_EXPORTER_OTLP_ENDPOINT`. nav-pilot setter også `COPILOT_OTEL_ENABLED=true` hvis den
ikke allerede er satt, og injiserer resource-attributtene `nav.pilot.launcher`,
`nav.pilot.version` og `nav.pilot.device_id` i Copilots `OTEL_RESOURCE_ATTRIBUTES`
(append-merge, eksisterende nøkler beholdes) for å spore Copilot-traces tilbake til nav-pilot.

Støttede MVP-metrikker:

- `nav_pilot_command_duration_ms` (`_count` er også antall kommandoer)
- `nav_pilot_command_error_total`
- `nav_pilot_install_items_total`
- `nav_pilot_sync_updates_total`
- `nav_pilot_sync_conflicts_total`
- `nav_pilot_info`
- `nav_pilot_install_present`
- `nav_pilot_installed_items`
- `nav_pilot_staleness_check_total`
- `nav_pilot_up_to_date`
- `nav_pilot_version_skew_days`

Metrikkene bærer også `execution_context` for å skille organisk bruk fra CI (`organic`,
`ci_github_actions`, `ci_other`, `unknown`).

## Konfigurasjon

Du kan lagre standardvalg i `~/.nav-pilot/config.toml`.

```bash
nav-pilot config          # interaktiv innstillingsside, alle valg med forklaring
nav-pilot config init
nav-pilot config setup
nav-pilot config show
```

Støttede felt er `client`, `model`, `mode`, `reasoning_effort`, `context_tier`,
`allow_all_tools`, `ask_user`, `auto_launch` og `log_level`. Du kan overstyre dem per kjøring
med globale flagg som `--client`, `--model`, `--mode`, `--effort`, `--context`,
`--allow-all-tools`, `--no-ask-user`, `--auto-launch`/`--no-auto-launch` og `--log-level`.

`--project-dir <katalog>` bestemmer hvilken katalog agenten får lese og skrive i cplt-sandboxen.
Standard er katalogen du står i, ikke roten av git-repoet rundt den.

`--persona <navn>` velger hvilken av agentpakkens `primaryAgents` som startes. Uten
flagget startes den første. Et navn som ikke er deklarert for klienten avvises med
en liste over dem som finnes, i stedet for å sendes videre til klienten.

Flagget gjelder Tier 1. En Tier 2-payload henter personaen fra sitt eget manifest,
og en kilde uten manifest har ingen liste å velge fra; begge avviser flagget i
stedet for å overse det.

`--payload-context <id>` gjelder bare kilder som er en agentpakke med ferdigbygde payloads,
og velger hvilken kontekst som stages ved launch. Den har ingen config-nøkkel, standarden er
`defaultContext` i pakkas manifest. Den er ikke det samme som `--context`, som fortsatt er
Copilots long-context-nivå. Se [README.agentpakke.md](README.agentpakke.md).

Etter synk eller installasjon starter nav-pilot kodeagenten automatisk. Sett
`auto_launch = false` (eller bruk `--no-auto-launch`) hvis du heller vil starte den selv.
Da skriver nav-pilot bare ut kommandoen du kan kjøre.

**Modell per klient:**

- Copilot: `auto`, `claude-opus-5`, `claude-fable-5`, `claude-sonnet-5`,
  `claude-sonnet-4.6`, `claude-haiku-4.5`, `claude-opus-4.8`, `claude-opus-4.6`,
  `gpt-5.6-sol`, `gpt-5.6-terra`, `gpt-5.6-luna`, `gpt-5.5`, `gpt-5.4`,
  `gpt-5.3-codex`, `gpt-5.4-mini`, `gpt-5-mini`, `gemini-3.6-flash`,
  `gemini-3.1-pro-preview`, `gemini-3.5-flash`, `kimi-k2.7-code`, `kimi-k3`
- opencode (startes via cplt mot GitHub Copilot-provideren): bruk `github-copilot/<id>`,
  f.eks. `github-copilot/claude-opus-4.8`, `github-copilot/gpt-5.5`. Modellen i config
  må være på `provider/model`-format (med `/`). Uten en satt modell (eller `--model auto`
  på CLI) brukes en modell den aktive agentpakken selv har erklært, hvis den har erklært
  en; ellers sendes ingen `--model`-flagg, og opencode velger selv en modell den vet
  kontoen din har tilgang til. opencode har ingen `auto`-modell selv (det er et
  Copilot-CLI-konsept), så en ren `github-copilot/auto` avvises av opencode; nav-pilot
  normaliserer den bort til det samme oppsettet i stedet.

Veiviseren (`nav-pilot config setup`) viser en modellvelger tilpasset valgt klient, og
`nav-pilot config explain model` lister opp de kurerte id-ene.

**opencode-mapping:** `client = "opencode"` mappes til opencode-flagg. `mode = plan` gir
`--agent plan` (ellers `--agent nav-pilot`), `model` gir `--model` (prefikses med
`github-copilot/` for bare id-er), `reasoning_effort` gir `--variant`, `allow_all_tools` gir
`--dangerously-skip-permissions`, og `log_level` oversettes til opencodes sett
(`DEBUG`/`INFO`/`WARN`/`ERROR`). Felt uten opencode-ekvivalent (`mode = autopilot`,
`context_tier`, `ask_user = false`) gir en ⚠-advarsel ved oppstart.

## For bidragsytere

- Agent: `.github/agents/nav-pilot.agent.md`
- Design: `docs/nav-pilot-design.md`
- Skills: `.github/skills/<name>/`
- Instruksjoner: `.github/instructions/`

Detaljert bruk, CLI-referanse og arbeidsflyt vedlikeholdes i online docs.
