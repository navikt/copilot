---
title: "cplt: global, local eller repo, og flere repoer i samme sesjon"
date: 2026-09-08
author: starefossen
category: praksis
excerpt: "Det prosjektet trenger, hører hjemme i .cplt.toml. Det som bare gjelder din maskin, settes med --local. Global skal være liten. Pluss sandbox.repo_dirs, som lar agenten jobbe mot et nested repo i samme sesjon."
tags:
  - cplt
  - config
  - sandbox
  - nav-pilot
---

Kortversjonen: **Det prosjektet trenger (porter, docker, hemmeligheter som skal holdes ute), setter du med `cplt config set --repo …`. Det havner i `.cplt.toml`, som du committer. Det som bare er sant på din maskin, setter du med `--local` — stier i hjemmekatalogen, og repoer du har sjekket ut inni prosjektet:**

```sh
cplt config set --local sandbox.repo_dirs ~/src/spleis/libs/sykepenger-model
```

**Global skal være liten. Start med `cplt init --write` i repoet, og sjekk resultatet med `cplt config show`.**

Du starter agenten med [nav-pilot](/nav-pilot), og nav-pilot kjører den trygt i en sandbox. Første gang du merker [cplt](/cplt), er som regel når noe blir stoppet: Gradle får ikke lese `~/.gradle/gradle.properties`, Testcontainers finner ikke docker, `npm install` nekter å kjøre postinstall. Da må du justere sandboxen, og spørsmålet blir: skal innstillingen settes globalt, lokalt eller i repoet?

Dette innlegget svarer på det, og viser hvordan du lar agenten jobbe mot to repoer i samme sesjon. Alt gjøres med `cplt config`, og nav-pilot plukker det opp neste gang du starter en agent.

---

## Fire lag, og repoet

cplt slår opp en innstilling i denne rekkefølgen. Første treff vinner for enkeltverdier. Lister (`allow.read`, `allow.write`, `allow.ports`, `allow.localhost`, `deny.paths`) slås sammen på tvers av lagene.

| Lag | Gjelder for | Settes med | Ligger i |
| --- | --- | --- | --- |
| Flagg | denne ene kjøringen | `cplt --allow-write …` | ingenting lagres |
| **Local** | én checkout | `cplt config set --local …` | `~/.config/cplt/local/<hash>.toml` |
| **Global** | hele maskinen din | `cplt config set …` | `~/.config/cplt/config.toml` |
| Defaults | alle | preset `standard` | innebygd |

`.cplt.toml` i repoet er et femte sted, og det er stedet du bør begynne. Fila står utenfor rekkefølgen over fordi den er prosjektets innstillinger og ikke dine: den er sporet i git, går gjennom review i en PR, og en ny på teamet får riktig sandbox ved å klone. Den har derfor sin egen tillitsmodell:

- `[deny]` gjelder med en gang. Repoet kan alltid stramme inn.
- `[propose]` er forslag om å utvide, og hver utvikler må godkjenne dem på sin maskin med `cplt trust`. Godkjenningen er bundet til innholdet i fila. Endres fila, må du godkjenne på nytt. Local slipper dette steget fordi fila ligger utenfor repoet, der agenten ikke kan skrive.
- cplt leser fila fra git HEAD, ikke fra working tree. Agenten kan altså ikke redigere sin egen sandbox-config og få det til å gjelde.

Regelen for å velge lag er ett spørsmål: **hvem gjelder dette for?**

- Prosjektet, altså alle som sjekker det ut → repo-config (`.cplt.toml`)
- Meg, i denne checkouten → local
- Meg, uansett hvilket repo jeg står i → global
- Ingen spesielle → la default stå

Det meste prosjektet trenger, hører hjemme i repoet, også om du er alene på det. Behovet følger med til neste maskin du kloner på. Skal du sette opp et repo fra bunnen, skanner `cplt init` prosjektet og skriver fila for deg:

```sh
cplt init            # se hva som detekteres
cplt init --write    # skriv .cplt.toml, commit den
```

---

## Hvorfor ikke bare global

Det er fristende å sette alt globalt, så slipper du å gjøre det igjen. Ikke gjør det. En grant i global følger deg inn i hver eneste sesjon, også i repoer der stien betyr noe helt annet. Gir du `~/.gradle/gradle.properties` globalt, kan en agent i et Node-prosjekt lese GitHub Packages-tokenet ditt uten å ha noen grunn til det.

Global skal være liten: ting som er sanne for maskinen din uansett prosjekt. Signerer du commits med GPG, er det et godt eksempel:

```sh
cplt config set sandbox.allow_gpg_signing true --force
```

---

## Repoet må be om én ting om gangen

Grensen mellom repo og local er ikke bare en konvensjon. Noen nøkler nekter `.cplt.toml` å ta imot, og sier hvorfor:

```
$ cplt config set --repo sandbox.preset permissive
[cplt] sandbox.preset is not valid in repo config.
  Reason: composes multiple dangerous permissions (docker, tmp exec, ...). A repo must request individual keys so each can be reviewed and trusted.
```

Det er tillitsmodellen i én setning: repoet ber om én og én tillatelse, så hver kan reviewes og godkjennes for seg. Skillet er ikke like skarpt for alle nøkler, men er du i tvil om hvor noe hører hjemme, prøv `--repo` og les svaret.

---

## Hvor hører dette hjemme?

**Appen lytter på en localhost-port.** Localhost er blokkert som standard. Lytter appen på 8080, gjør den det for alle på teamet:

```sh
cplt config set --repo allow.localhost 8080
```

Verdien havner under `[propose]`, og cplt minner deg på de to stegene som gjenstår: `cplt trust accept --all` for å godkjenne på din egen maskin, og en commit av `.cplt.toml` så resten av teamet får den. Kjører du noe på en port bare du bruker, er det local:

```sh
cplt config set --local allow.localhost 3000
```

**Testcontainers trenger docker.** Testene er prosjektets, og da er docker-tilgangen det også. Docker svekker sandboxen, så cplt nekter uten `--force` og forteller deg nøyaktig hvilken kommando du trenger. Bruker testene Postgres på 5432, er den porten prosjektets på samme måte:

```sh
cplt config set --repo sandbox.allow_docker true --force
cplt config set --repo allow.ports 5432
```

Den som reviewer PR-en ser at prosjektet ber om docker, og hver utvikler godkjenner på sin maskin. Det er poenget med å legge det i repoet, ikke en omvei.

**npm lifecycle scripts** (`postinstall` og venner) er blokkert som standard, fordi de er vilkårlig kodekjøring. Feiler `npm install` uten dem, feiler det for alle. Det er samtidig det tyngste du kan be teamet godkjenne, og nettopp derfor hører det hjemme i en PR og ikke i hver utviklers local-fil:

```sh
cplt config set --repo sandbox.allow_lifecycle_scripts true --force
```

`cplt init` foreslår aldri denne selv. Står den i `.cplt.toml`, har et menneske satt den inn, og da er det verdt et spørsmål i reviewen: trenger `npm install` den virkelig, eller holder det med `--ignore-scripts`?

**Hemmeligheter som aldri skal inn i en agent-sesjon.** `[deny]` er den billige halvdelen av repo-config. Den gjelder for alle som sjekker ut repoet, i det øyeblikket fila er committet:

```sh
cplt config set --repo deny.env VAULT_TOKEN
```

Ingen `cplt trust`, ingenting å vente på. Det er den ene linja i dette innlegget som ikke koster noen noe.

**Gradle trenger GitHub Packages-credentials** fra `~/.gradle/gradle.properties`. Prosjektet trenger tokenet, men fila ligger i hjemmekatalogen din, og det er din maskin som avgjør hvor credentials bor. Local:

```sh
cplt config set --local allow.read ~/.gradle/gradle.properties
```

Finnes ikke fila ennå, får du en advarsel, men verdien lagres likevel.

**Agenten skal pushe en feature branch.** Ingenting å gjøre. Preset `standard` har git guard på i block-modus og beskytter bare default branch. Push til feature branch går, push til main stoppes.

**git-oppsettet ditt** i `~/.gitconfig` gjelder maskinen din, ikke ett prosjekt. Global, altså uten `--local`:

```sh
cplt config set allow.read ~/.gitconfig
```

---

## Se hva som gjelder

```sh
cplt config show                    # alt som gjelder her; enkeltverdier fra local er merket (local)
cplt config get sandbox.repo_dirs   # én verdi, med lag
cplt config path --local            # hvor local-fila for denne checkouten ligger
cplt config local list              # alle prosjekter du har local-config for
```

Listeverdier vises sammenslått uten lag-merking. En `allow.read` du satte lokalt, får derfor ikke `(local)` etter seg i `config show`. Ved oppstart viser cplt en `Local:`-rad med fila som er i bruk, når en slik fil faktisk gjelder.

---

## Flere repoer i samme sesjon

Et typisk oppsett: `navikt/spleis` har `navikt/sykepenger-model` sjekket ut under `libs/` i samme tre. Agenten kan allerede lese og skrive filene i `libs/sykepenger-model`, fordi prosjektkatalogen er gitt som ett subtre. Det den ikke kan, er å behandle det nested repoet som *et repo*. gh guard slipper bare gjennom skriveoperasjoner mot repoet du startet i, så `gh pr create -R navikt/sykepenger-model` stoppes.

`sandbox.repo_dirs` fikser det. Du navngir repoet, og det får identitet i sesjonen:

```sh
cplt config set --local sandbox.repo_dirs ~/src/spleis/libs/sykepenger-model
```

Stien må være absolutt eller starte med `~/`. Ved neste oppstart viser cplt begge:

```
 Repositories:
   navikt/spleis            ~/src/spleis                        launch repository
   navikt/sykepenger-model  ~/src/spleis/libs/sykepenger-model   local config
```

Nå kan agenten opprette PR-er mot `navikt/sykepenger-model`. Legg merke til at agenten ikke fikk noen ny filtilgang. Å navngi et repo gir identitet, ikke tilgang.

Nøkkelen kan bare settes med `--local`. Global nekter, fordi en repo-liste der ville hengt seg på hver eneste sesjon. Repo-config nekter, fordi «naming other trees as project-grade roots is a path grant, and repo config cannot grant paths». Hvor du har sjekket ut hva, er sant for din maskin, ikke for prosjektet.

Kjører du cplt direkte, gjør flagget `--repo-dir <sti>` det samme for én kjøring, i tillegg til det som er lagret. Går du via nav-pilot, er `--local` den eneste veien.

### Repoet må ligge inni prosjektkatalogen

Dette er begrensningen som svir, og vi skal være ærlige om den: et repo du vil navngi må være sjekket ut *inni* katalogen du starter fra. Ligger `sykepenger-model` på `~/src/sykepenger-model`, ved siden av `spleis`, får du nei.

De fleste av oss har repoene liggende side om side. Det betyr at funksjonen slik den står i dag treffer færre enn den burde.

Du kan få tilgang til filene, men ikke gh-identitet mot repoet:

```sh
cplt config set --local allow.write ~/src/sykepenger-model
```

Agenten kan da bygge og redigere der, men ikke opprette PR-er mot repoet.

**Det som faktisk virker i dag** er å legge repoene inni et felles repo og starte derfra. Lag en katalog, `git init` i den, og sjekk ut repoene du jobber med under den:

```sh
mkdir ~/src/min-arbeidsflate && cd ~/src/min-arbeidsflate
git init
git clone git@github.com:navikt/spleis.git
git clone git@github.com:navikt/sykepenger-model.git
cplt config set --local sandbox.repo_dirs ~/src/min-arbeidsflate/spleis
cplt config set --local sandbox.repo_dirs ~/src/min-arbeidsflate/sykepenger-model
```

Da får begge repoene identitet, og agenten kan opprette PR-er mot begge. Én ting må være på plass: den ytre katalogen må selv ha en GitHub-remote. Uten den klarer ikke cplt å fastslå hvilket repo sesjonen starter i, og da blokkeres alle gh-kommandoer som sjekkes mot scope — også mot de navngitte repoene.

Det er en omvei, ikke en løsning. Den krever at du legger om hvordan du har sjekket ut kode, og det er ikke noe vi mener du bør måtte gjøre.

**Vi er ikke i mål her.** Å la et repo utenfor prosjektkatalogen bli førsteklasses er ikke vanskelig i seg selv — sandkassa håndterer allerede den slags tilgang, og cplt kan allerede lese identiteten til en hvilken som helst katalog. Det som tar tid, er å få grensesnittet og sikkerhetsmodellen riktig samtidig: hvem som får peke ut hvilke repoer, hva som skjer når en katalog byttes ut under føttene på deg, og hvordan du ser hva agenten faktisk har lov til før du starter den. Vi har brukt uka på å tette hull der nettopp den slags var uklart, så vi tar den tiden.

Målet er en ordentlig workspace-modell, der repoer som ligger side om side kan kobles sammen uten omveier. Følg [navikt/cplt#344](https://github.com/navikt/cplt/issues/344).

Repoet du startet i, kan du forresten ikke navngi. Det er alltid i scope.

### Sjekkes på nytt hver gang

Local-fila kan være uker gammel, og treet den peker på kan agenten skrive til. Derfor sjekker cplt hver lagrede oppføring ved *hver* oppstart, og stopper hvis den ikke lenger er toppen av et git-repo inni prosjektkatalogen. Feilmeldingen sier hvilken fil oppføringen kom fra, så du vet hva du skal rydde. Du fjerner med `config set … --unset`:

```sh
cplt config set --local sandbox.repo_dirs ~/src/spleis/libs/sykepenger-model --unset   # fjern ett element
cplt config set --local sandbox.repo_dirs --unset                                      # fjern hele nøkkelen
```

---

## Flyttet eller byttet checkout

Local-fila husker hvilken `origin` den ble skrevet for. Står det et annet repo på samme sti nå, advarer cplt (også med `--quiet`) og bruker ingenting fra fila. Vil du ta den i bruk igjen, setter du en hvilken som helst nøkkel med `--local` på nytt. En checkout du har flyttet, etterlater en foreldreløs fil. `cplt config local list` viser den som «path no longer exists».

---

## Kom i gang

Kotlin-tjeneste med nested bibliotek, Gradle mot GitHub Packages og Testcontainers:

```sh
cd ~/src/spleis

# Det prosjektet trenger, i .cplt.toml
cplt config set --repo allow.localhost 8080
cplt config set --repo sandbox.allow_docker true --force
cplt config set --repo deny.env VAULT_TOKEN
cplt trust accept --all
# commit .cplt.toml og åpne en PR

# Det som er ditt, i denne checkouten
cplt config set --local allow.read ~/.gradle/gradle.properties
cplt config set --local sandbox.repo_dirs ~/src/spleis/libs/sykepenger-model

# Sjekk resultatet
cplt config show
cplt config get sandbox.repo_dirs
```

Start så agenten gjennom nav-pilot som før. Oppstartssammendraget viser `Local:`-raden og begge repoene.

Full referanse i [docs/configuration.md](https://github.com/navikt/cplt/blob/main/docs/configuration.md).
