# 📦 Agentpakke

En **agentpakke** er et innholdsrepo som beskriver seg selv for nav-pilot: agenter, skills, instruksjoner og prompts, pluss et manifest som sier hvilke klienter pakka støtter, hvilke agenter som er primære personaer, og hvor innholdet ligger i repoet. Et team kan dermed distribuere sitt eget sett med tilpasninger uten å forke nav-pilot eller sende PR til `navikt/copilot` — brukerne installerer pakka med `nav-pilot install --source <owner>/<repo>`. Manifestet ligger på `.nav-pilot/agentpakke.json` i agentpakke-repoet og er hele kontrakten mellom repoet og binæren.

Kontrakten er publisert som JSON Schema i [`cli/nav-pilot/schemas/agentpakke-v1.json`](../cli/nav-pilot/schemas/agentpakke-v1.json). Nøyaktig samme fil er kompilert inn i nav-pilot-binæren, så repoets egen CI-lint og nav-pilot validerer mot identiske bytes.

Dette dokumentet er for team som lager en agentpakke. Interndesignet (hvordan manifestet trådes gjennom install og sync) står i [cli/nav-pilot/DESIGN.md](../cli/nav-pilot/DESIGN.md).

## Repoform

```
<agentpakke-repo>/
├── .nav-pilot/
│   └── agentpakke.json      # manifestet
├── agents/                  # layout.agents        — <navn>.agent.md
├── skills/                  # layout.skills        — <navn>/SKILL.md
├── instructions/            # layout.instructions  — <navn>.instructions.md
└── prompts/                 # layout.prompts       — <navn>.prompt.md eller <navn>/
```

Katalognavnene er ikke låst — `layout` peker på hvor innholdet faktisk ligger, og nav-pilot leser kun der. Filnavnkonvensjonene inne i katalogene er derimot låst, fordi det er dem nav-pilot bruker til å finne og navngi artefaktene. Agentfiler må åpne med en `---`-avgrenset YAML-frontmatter som minst deklarerer `name` og `description`.

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
| `provenance` | objekt: `base` (`repo`\*, `digest`\*), `overlays[]` (`component`\*, `version`\*) | nei | Opphav for komponert innhold. Ren metadata; nav-pilot verifiserer ikke digest. |
| `minNavPilotVersion` | string, `YYYY.MM.DD-HHMMSS[-sha]` | nei | Minste nav-pilot-versjon. Se [Versjonsgate](#versjonsgate). |

### `clients.<klient>`

Nøkkelen er en identifikator (`^[a-z][a-z0-9-]*$`). Klientene denne binæren kan starte er `copilot`, `opencode` og `pi`. Andre nøkler er *ikke* feil — de ignoreres, og klienten er bare utilgjengelig her.

| Felt | Type | Påkrevd | Betydning |
| --- | --- | --- | --- |
| `primaryAgents` | array av string, minst ett element | ja | Agentene som er valgbare som primære personaer i klienten. Alt annet i `agents/` materialiseres som subagent. |
| `compatibility` | string | nei | Støttet klientversjon som **range** (f.eks. `">=1.18.20,<2"`), ikke en eksakt pin. |
| `defaultModel` | string | nei | Modell-id, eller literalen `"inherit"` (ikke pin noe; arv provider-/sesjonsvalget). |
| `defaultContext` | identifikator | nei | Hvilken payload-kontekst som startes som standard. Uten verdi: `"full"`. |
| `payloads` | objekt, minst én nøkkel | nei | Tier 2: én oppføring per kontekst (i dag `full`, `focused`). Tilstedeværelsen av dette feltet er det som gjør oppføringen til Tier 2. |

### `clients.<klient>.payloads.<kontekst>`

Kontekstnøkkelen er en identifikator, og ukjente kontekstnøkler ignoreres på samme måte som ukjente klientnøkler.

| Felt | Type | Påkrevd | Betydning |
| --- | --- | --- | --- |
| `path` | repo-relativ sti | ja | Katalogen som holder det ferdigbygde payload-treet. |
| `manifest` | repo-relativ sti | nei | Overstyrer payload-manifestets plassering. Uten dette feltet er konvensjonen `<path>/manifest.json`. |

## Tier utledes av form

En klients konformanstier deklareres ikke — den utledes av formen på oppføringen, slik at den ikke kan drifte fra innholdet:

- **Tier 1 (layout):** klientoppføringen har ingen `payloads`. nav-pilot materialiserer innholdet selv fra stiene i `layout`. Da må `layout` finnes.
- **Tier 2 (payloads):** klientoppføringen har `payloads`. nav-pilot verifiserer og stager ferdigbygde trær. Ingen `layout` kreves for slike klienter.

En pakke kan blande: `copilot` på Tier 2 og `opencode` på Tier 1 i samme manifest er gyldig, forutsatt at `layout` finnes for Tier 1-klienten.

Mangler `layout` mens en *kjent* klient er Tier 1, avvises manifestet:

```
client(s) opencode declare no payloads, which makes them Tier 1, but the manifest has no "layout".
Add a layout with agents and skills paths, or declare payloads to make them Tier 2
```

## Ignorer-ukjent, og hva som feiler lukket

Regelen har to halvdeler, og de gjelder samtidig:

- **Ukjente konstruksjoner ignoreres.** Ukjente klientnøkler, ukjente kontekstnøkler i `payloads`, og ekstra felt på ethvert nivå gjør ikke manifestet ugyldig. En eldre binær tilbyr bare ikke det den ikke kjenner navnet på. Dermed kan økosystemet vokse uten å ugyldiggjøre manifester som allerede er ute.
- **Feilformede *kjente* konstruksjoner feiler lukket.** Er `primaryAgents` tom, er `layout` fraværende for en Tier 1-klient, er `contractVersion` en major nav-pilot ikke implementerer, eller er `minNavPilotVersion` skrevet på et format nav-pilot ikke kan sammenligne — så stopper det. Et repo som *har* et manifest må ha et gyldig et: install avbrytes før første filoperasjon, og ingenting skrives delvis.

Et repo helt uten `.nav-pilot/agentpakke.json` er ikke en feil. Det behandles som en legacy-samlingskilde (`collections/<navn>/manifest.json`) akkurat som før.

## Stiregler

Alle repo-relative stier i manifestet (`layout.*`, `policies.*`, `profiles.dir`, `payloads.*.path`, `payloads.*.manifest`) valideres etter samme regler:

1. Ikke tom, ikke omgitt av whitespace.
2. Skråstrek forover (`/`). Backslash avvises, også på Linux.
3. Relativ. Absolutte stier og `~`-prefiks avvises.
4. Ingen escape: en sti som normaliseres til `..` eller starter med `../` avvises.
5. **Symlinker må holde seg inne i checkouten.** En sti som *er* en symlink avvises. En sti som når ut av repoet via en symlinket foreldrekatalog avvises også — hele stikjeden løses opp, ikke bare siste ledd. Det samme gjelder enkeltfiler i `agents/`.

Regel 1–4 er tekstlige og kjøres når manifestet parses. Regel 5 krever checkouten på disk og kjøres av `nav-pilot validate` og av install før noe skrives.

## Innholdssjekker på disk

Utover manifestets form sjekker `nav-pilot validate` (og install) innholdet:

- Hver `layout`-katalog som er deklarert, finnes og er en katalog.
- `layout.agents` inneholder minst én `*.agent.md`, og hver av dem åpner med YAML-frontmatter.
- Hver Tier 2 `payloads.<kontekst>.path` finnes som katalog.
- Hver Tier 2-payload har et payload-manifest — `<path>/manifest.json`, eller filen `manifest` peker på. nav-pilot nekter å stage en payload uten manifest.
- Hvert payload-tre stemmer eksakt med payload-manifestet sitt: `schemaVersion` er 1, hver oppføring i `files` har en gyldig sha256 (64 tegn, små bokstaver) og modus `0644` eller `0755`, hver deklarert fil finnes med riktig innhold, og treet inneholder ingen fil manifestet ikke lister. Symlinker og andre ikke-vanlige filer avvises. Modus sjekkes som kjørbit på kilden, slik at en streng `umask` i checkouten ikke gir falske avvik — eksakt modus settes og verifiseres når payloaden stages.
- En payload kan ikke selv ha en fil som heter `manifest.json` i rota. Staging skriver payload-manifestet dit i det stagede treet, så navnet er opptatt. Dette kan bare oppstå når `manifest` peker på en fil utenfor payload-katalogen; ligger manifestet på konvensjonsplassen, er det manifestet.
- `policies.opencodePermissions` finnes som fil hvis den er satt.
- `profiles.dir` finnes som katalog, og `<dir>/<default>.json` finnes hvis `profiles.default` er satt.

Alle funn rapporteres samlet, ikke bare det første.

## Versjonsgate

`minNavPilotVersion` må skrives på nav-pilots releaseformat: `YYYY.MM.DD-HHMMSS`, eventuelt med build-sha (`2026.09.01-120000-a1b2c3d`). Andre formater avvises framfor å ignoreres — nav-pilot kan ikke sammenligne dem, og å godta dem ville stille av akkurat den gaten manifestet ba om.

Kjører brukeren en eldre release enn kravet:

```
agentpakke "grillmester" requires nav-pilot 2026.09.01-120000 or newer, but this binary is
2026.08.01-100000-abc1234. Run `nav-pilot update` (or reinstall via Homebrew) and try again
```

Utviklingsbygg (`dev`) er unntatt gaten, slik at lokalt arbeid på en agentpakke ikke blokkeres.

## Kompatibilitet

Kontrakten er bygget for at en agentpakke skal kunne vokse uten å knekke brukere på eldre binærer.

**Trygt å legge til uten bump av `contractVersion`:**

- Nye klientnøkler i `clients` (eldre binærer ignorerer dem).
- Nye kontekstnøkler i `payloads` (samme regel).
- Nye felt på ethvert objekt i manifestet.
- Flere `primaryAgents`, endret `defaultModel`, endret `compatibility`, ny `provenance`.
- Nytt innhold i `layout`-katalogene.

**Krever varsomhet, men ikke bump:**

- Å flytte innhold ved å endre `layout`-stier: eksisterende brukere får det nye innholdet ved neste sync, men repoet må fortsatt validere. `nav-pilot export opencode` støtter foreløpig bare kanoniske stier (se [Begrensninger](#begrensninger-i-dag)).
- Å endre `name`: eksisterende installasjoner er registrert under det gamle navnet i state, så nav-pilot kjenner dem ikke igjen som samme pakke. `nav-pilot install <nytt-navn>` blir en ny installasjon i samme scope.
- Å heve `minNavPilotVersion`: eldre binærer blokkeres bevisst, med en melding som sier hva de skal gjøre.

**Krever bump av `contractVersion`:**

- Å endre betydningen av et eksisterende felt.
- Å gjøre et tidligere valgfritt felt påkrevd, eller å fjerne et felt konsumenter leser.
- Å endre en konvensjon konsumenten har innebygget, for eksempel hvor payload-manifestet ligger.

Motsatt vei har nav-pilot en tilsvarende forpliktelse: en fjerning på kontrakten skjer med deprekeringsvindu, ikke i én release.

## Validering i CI

```bash
nav-pilot validate [--source <owner/repo>|<absolutt sti>] [--ref <ref>] [--json]
```

Kommandoen kjører hele konformanssjekken mot en kilde: manifestet mot schemaet, semantikken (kontraktversjon, versjonsgate, tier/layout, stiregler) og innholdet på disk. Den er ment for agentpakke-repoets egen CI.

> Ikke å forveksle med `nav-pilot config validate`, som sjekker brukerens egen `~/.nav-pilot/config.toml`.

`--source` tar enten `owner/repo` (klones) eller en **absolutt** sti til en checkout. Relative stier og `.` avvises. Uten `--source` faller kommandoen tilbake på kildepresedensen (`--source` > `source`-nøkkelen i config > `navikt/copilot`), og lokal auto-deteksjon slår kun til for et repo som har en `collections/`-katalog — altså ikke for en vanlig agentpakke. I CI skal du derfor peke eksplisitt på checkouten:

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

Et repo med manifest **erstatter** samlingsmodellen: det tilbyr nøyaktig ett installerbart navn, nemlig manifestets `name`. `nav-pilot list --source <owner>/<repo>` viser det under overskriften `Agentpakke in <kilde>:`. Kjører brukeren `nav-pilot install --source <owner>/<repo>` uten navn i et repo, hopper den interaktive flyten over samlingsvelgeren og installerer pakka — det er ingenting å velge mellom.

Er navnet på pakka også navnet på en agent i den (vanlig), vinner pakka. Agenten kan fortsatt installeres alene med `--type agent`.

**Kilden huskes.** Etter en vellykket install med eksplisitt `--source` lagres verdien som `source` i `~/.nav-pilot/config.toml`:

```
✓ Saved navikt/grillmester as your source in /Users/<deg>/.nav-pilot/config.toml.
  Future runs use it without --source; clear it with nav-pilot config set source "".
```

Presedensen er `--source` > `source` i config > `navikt/copilot`. Tøm nøkkelen med:

```bash
nav-pilot config set source ""
```

**Valget er per scope.** Repo-scope (`.github/` i et repo) og bruker-scope (`~/.copilot`, `--user`) huskes hver for seg, og hvert scope registrerer kilden sin i sin egen state. Ulike repoer kan derfor følge ulike agentpakker.

**Kryss-kilde-vakt.** nav-pilot blander ikke innhold fra to agentpakker i én installasjon. Er scopet installert fra én kilde mens config peker på en annen, stopper install med valgene skrevet ut:

```
the repo scope was installed from navikt/grillmester, but your configured source is navikt/copilot.
nav-pilot will not silently mix content from two agentpakker into one install.

  Keep this scope on its current source:  nav-pilot install --source navikt/grillmester <name>
  Switch this scope to the new source:    nav-pilot install --source navikt/copilot <name>
  Or clear the persisted source:          nav-pilot config set source ""
```

Et eksplisitt `--source` er overstyringen — da har brukeren sagt hvilken kilde som skal inn i hvilket scope. Vakten gjelder også de interaktive flytene, så det å velge scope i en meny er ingen vei rundt den.

**Sync.** `nav-pilot sync` bruker kilden som er registrert for scopet, ikke den konfigurerte, og sier fra når de to er ulike. Et scope som ble installert *før* nav-pilot begynte å registrere kilder har ingen registrert kilde: da synkes det fra den konfigurerte kilden, det skrives én informasjonslinje om det (ikke i `--json`), og kilden festes i state først **etter** en vellykket sync — en sync som feiler registrerer ingenting. Fra neste kjøring gjelder den vanlige vakten. Uten `source` i config, og med eksplisitt `--source`, adopteres ingenting.

```
ℹ The repo scope predates source tracking; syncing from navikt/copilot and recording it as this scope's source.
```

Stiformede kilder sammenlignes symlink-oppløst, så en symlink og checkouten bak den regnes som samme kilde.

## Begrensninger i dag

Dette er statusen i milepæl 1. Alt under er kjent og planlagt, ikke feil:

- **Tier 2 kan valideres, men ikke installeres.** En agentpakke uten `layout` der klientene har `payloads` validerer fint, men install avvises med sin egen begrunnelse: payload-staging kommer i en senere release (M2). En pakke som skal installeres i dag må ha Tier 1-innhold.
- **`policies` og `profiles` parses og sti-sjekkes, men materialiseres ikke.** nav-pilot skriver hverken opencode-permissions eller launch-profiler ut fra manifestet ennå (M3).
- **Persona, modell og launch leser fortsatt Nav-defaults.** `primaryAgents`, `defaultModel`, `defaultContext` og `compatibility` valideres, men kallstedene i `internal/provider` og `internal/source` er ennå hardkodet. De flyttes over til manifestet i M2. En agentpakke bør deklarere feltene riktig nå, slik at oppførselen blir riktig når koden begynner å lese dem.
- **`nav-pilot export opencode` støtter bare kanoniske stier.** Export leser `agents/`, `skills/`, `instructions/` og `prompts/` direkte. En agentpakke som legger innholdet et annet sted avvises av export med en forklaring, framfor å skrive et tomt `.opencode/`-tre.
- **Ferskhetssjekken i rot-TUI-en beskriver bare standardkilden.** Scope som ikke kommer fra `navikt/copilot` hoppes over der, fordi release-feeden til nav-pilot bare sier noe om standardkilden.
- **`provenance` verifiseres ikke.** Feltet er metadata; nav-pilot sjekker ikke digest mot innholdet.

## Eksempler

### Minimal Tier 1

Alt en pakke trenger for å kunne installeres i dag:

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

Med repoform:

```
grillmester/
├── .nav-pilot/agentpakke.json
├── agents/grillmester.agent.md      # --- name: grillmester / description: … ---
├── agents/barista.agent.md
├── skills/nav-plan/SKILL.md
├── instructions/kotlin.instructions.md
└── prompts/ny-tjeneste.prompt.md
```

### Tier 2 med ferdigbygde payloads

Formen en komponert pakke har. Denne validerer, men kan ennå ikke installeres av nav-pilot (se [Begrensninger](#begrensninger-i-dag)):

```json
{
  "contractVersion": "1",
  "name": "grillmester",
  "description": "A Copilot agent team for clarified software delivery, design, and product work with portable progressive skills.",
  "owner": { "repo": "navikt/grillmester", "team": "Team eSyfo" },
  "clients": {
    "copilot": {
      "primaryAgents": ["grillmester", "barista", "designer", "doctor-who"],
      "compatibility": ">=1.0.79,<2",
      "defaultModel": "gpt-5.6-sol",
      "defaultContext": "full",
      "payloads": {
        "full": { "path": "plugin" },
        "focused": { "path": "targets/copilot-cli-focused-v1" }
      }
    },
    "opencode": {
      "primaryAgents": ["grillmester", "barista", "designer", "doctor-who"],
      "compatibility": ">=1.18.20,<2",
      "defaultModel": "inherit",
      "payloads": {
        "full": { "path": "targets/opencode-v1" },
        "focused": { "path": "targets/opencode-v1-focused" }
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

Hver payload-katalog må ha et payload-manifest: `plugin/manifest.json`, `targets/opencode-v1/manifest.json`, og så videre — eller en `manifest`-overstyring i oppføringen.

## Se også

- [JSON Schema: `cli/nav-pilot/schemas/agentpakke-v1.json`](../cli/nav-pilot/schemas/agentpakke-v1.json) — kontrakten selv
- [nav-pilot](README.nav-pilot.md) — CLI-et som konsumerer agentpakker
- [Samlinger](README.collections.md) — legacy-modellen en agentpakke erstatter
- [Sync](README.sync.md) — hvordan installert innhold holdes oppdatert
- [cli/nav-pilot/DESIGN.md](../cli/nav-pilot/DESIGN.md) — internt design, sømmer og migrasjonsplan
