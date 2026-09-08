---
title: "cplt: global, local eller repo, og flere repoer i samme sesjon"
date: 2026-09-08
author: starefossen
category: praksis
excerpt: "Fire config-lag i cplt, én regel for å velge riktig. Pluss sandbox.repo_dirs, som lar agenten jobbe mot et nested repo i samme sesjon."
tags:
  - cplt
  - config
  - sandbox
  - nav-pilot
---

Du starter agenter gjennom [nav-pilot](/nav-pilot), og nav-pilot kjører dem i [cplt](/cplt). Første gang du merker cplt er som regel når noe blir stoppet: Gradle får ikke lese `~/.gradle/gradle.properties`, Testcontainers finner ikke docker, `npm install` nekter å kjøre postinstall. Da må du justere sandboxen, og da dukker spørsmålet opp: skal innstillingen settes globalt, lokalt eller i repoet?

Dette innlegget svarer på det, og viser hvordan du lar agenten jobbe mot to repoer i samme sesjon. Alt gjøres med `cplt config`. Du trenger ikke åpne en fil.

Én ting er verdt å ta med seg med én gang: nav-pilot starter cplt for deg, så du har ingen kommandolinje å henge flagg på. Derfor er det lagret config som gjelder. Setter du det med `cplt config set --local`, plukker nav-pilot det opp neste gang du starter en agent, uten at du gjør noe mer.

---

## Fire lag

cplt slår opp en innstilling i denne rekkefølgen. Første treff vinner for enkeltverdier; lister (`allow.read`, `allow.write`, `allow.ports`, `allow.localhost`, `deny.paths`) slås sammen på tvers av lagene.

| Lag | Gjelder for | Settes med | Ligger i |
| --- | --- | --- | --- |
| Flagg | denne ene kjøringen | `cplt --allow-write …` | ingenting lagres |
| **Local** | én checkout | `cplt config set --local …` | `~/.config/cplt/local/<hash>.toml` |
| **Global** | hele maskinen din | `cplt config set …` | `~/.config/cplt/config.toml` |
| Defaults | alle | preset `standard` | innebygd |

`.cplt.toml` i repoet er et femte sted, men det står utenfor rekkefølgen over og har sin egen tillitsmodell. Det er teamets fil, ikke din:

- `[deny]` slår inn med en gang. Repoet kan alltid stramme inn.
- `[propose]` er forslag om å utvide, og krever at hver utvikler godkjenner med `cplt trust`. Godkjenningen er bundet til innholdet i fila; endres fila, må du godkjenne på nytt.
- cplt leser fila fra git HEAD, ikke fra working tree. Agenten kan altså ikke redigere sin egen sandbox-config og få det til å gjelde.

Regelen for å velge lag er ett spørsmål: **hvem gjelder dette for?**

- Hele teamet, i dette repoet → repo-config (`.cplt.toml`)
- Meg, i denne checkouten → local
- Meg, uansett hvilket repo jeg står i → global
- Ingen spesielle → la default stå

Nesten alt du trenger å justere havner i **local**.

---

## Hvorfor local, og ikke global

Det fristende er å sette alt globalt, så slipper du å gjøre det igjen. Ikke gjør det. En grant i global følger deg inn i hver eneste sesjon, også i repoer der stien betyr noe helt annet. Gir du `~/.gradle/gradle.properties` globalt, kan en agent i et Node-prosjekt lese GitHub Packages-tokenet ditt, uten å ha noen grunn til det. Setter du det lokalt, gjelder det bare den Kotlin-tjenesten som faktisk bygger med Gradle.

Global skal være liten: ting som er sanne for maskinen din uansett prosjekt. Signerer du commits med GPG, er det et godt eksempel — det følger deg, ikke prosjektet:

```sh
cplt config set sandbox.allow_gpg_signing true --force
```

Det leder til et rimelig spørsmål: hvorfor får local lov til å utvide sandboxen i det hele tatt, når repo-config må be om godkjenning for det samme? Fordi local-fila ligger utenfor repoet, i `~/.config/cplt/`, og sandboxen nekter skriving dit. Agenten kan ikke skrive sine egne grants. En fil i working tree kunne ikke lovet det, og derfor må alt som utvider i `.cplt.toml` gjennom `cplt trust`.

---

## Hvor hører dette hjemme?

**Gradle trenger GitHub Packages-credentials** fra `~/.gradle/gradle.properties`. Det er ett prosjekt som trenger det, altså local:

```sh
cplt config set --local allow.read ~/.gradle/gradle.properties
```

Finnes ikke fila ennå, får du en advarsel, men verdien lagres likevel.

**Testcontainers trenger docker.** Docker-tilgang svekker sandboxen, så cplt nekter uten `--force` og forteller deg nøyaktig hvilken kommando som trengs:

```sh
cplt config set --local sandbox.allow_docker true --force
```

**Node-appen kjører på en localhost-port.** Localhost er blokkert som standard. Trenger bare du det, er det local:

```sh
cplt config set --local allow.localhost 3000
```

Trenger hele teamet det, hører det hjemme i `.cplt.toml`. Den skriver du også med en kommando:

```sh
cplt config set --repo allow.localhost 3000
```

Verdien havner under `[propose]`, og cplt minner deg på begge stegene som gjenstår: `cplt trust accept --all` for å godkjenne på din egen maskin, og en commit av `.cplt.toml` så resten av teamet får den.

Skal du sette opp et repo fra bunnen, skanner `cplt init` prosjektet og skriver fila for deg:

```sh
cplt init            # se hva som detekteres
cplt init --write    # skriv .cplt.toml, commit den
```

**npm lifecycle scripts** (`postinstall` og venner) er blokkert som standard, fordi de er vilkårlig kodekjøring. Trenger prosjektet det, er det local, med `--force`, og ikke noe du committer for teamet:

```sh
cplt config set --local sandbox.allow_lifecycle_scripts true --force
```

**Agenten skal pushe en feature branch.** Ingenting. Preset `standard` har git guard på i block-modus og beskytter bare default branch. Push til feature branch går, push til main stoppes.

**git-oppsettet ditt** i `~/.gitconfig` er sant for maskinen din, ikke ett prosjekt. Global, altså uten `--local`:

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

Merk at listeverdier vises sammenslått uten lag-merking, så en `allow.read` du satte lokalt får ikke `(local)` etter seg i `config show`.

Ved oppstart viser cplt hvilken local-fil som er i bruk. `Local:`-raden vises bare når en slik fil faktisk gjelder:

```
[cplt] Project:  ~/src/spleis
[cplt] Local:    /Users/x/.config/cplt/local/3291….toml
```

---

## Flere repoer i samme sesjon

Et typisk oppsett: `navikt/spleis` har `navikt/sykepenger-model` sjekket ut under `libs/` i samme tre. Agenten kan allerede lese og skrive filene i `libs/sykepenger-model`, fordi prosjektkatalogen er gitt som ett subtre. Det den ikke kan, er å behandle det nested repoet som *et repo*: gh guard slipper bare gjennom skriveoperasjoner mot repoet du startet i, så `gh pr create -R navikt/sykepenger-model` stoppes.

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

Nå kan agenten opprette PR-er mot `navikt/sykepenger-model`. Merk hva som *ikke* skjedde: det ble ikke gitt noen ny filtilgang. Å navngi et repo handler om identitet, ikke om tilgang.

Kjører du cplt direkte og bare vil prøve, finnes flagget. Det gjelder for den ene kjøringen og skriver ingenting til fila, og det kommer i tillegg til det som allerede er lagret, ikke i stedet for:

```sh
cplt --repo-dir ~/src/spleis/libs/sykepenger-model exec -- ./gradlew build
```

Går du via nav-pilot, er `--local` den eneste veien; det er ingen kommandolinje å sette flagget på.

### Bare nested, ikke sibling

Repoet må ligge inni prosjektkatalogen. Ligger `sykepenger-model` som `~/src/sykepenger-model`, ved siden av `spleis`, får du nei:

```
[cplt] --repo-dir /path/to/sibling is outside the project directory
  /path/to/launch-repo
  sibling repositories are not yet supported; use `--allow-write` for edit-only.
```

Dagens svar for sibling-repoer er `--allow-write`: agenten får redigere filene, men ikke gh-identitet mot repoet.

```sh
cplt --allow-write ~/src/sykepenger-model
```

Å starte fra `~/src` med `--project-dir` er ikke en omvei rundt dette. `--repo-dir` krever at du starter fra toppen av et git-repo, og en katalog som bare inneholder repoer er ikke det. Ordentlig sibling-støtte er [navikt/cplt#344](https://github.com/navikt/cplt/issues/344).

Å navngi repoet du startet i, nektes også; det er alltid i scope.

### Local, og bare local

`sandbox.repo_dirs` kan bare settes med `--local`. De to andre lagene nekter, med hver sin begrunnelse:

- Global: en repo-liste i global config ville hengt seg på hver eneste sesjon. Det er per prosjekt, ikke per maskin.
- Repo-config: å utnevne andre trær til prosjektnivå er en path grant, og repo-config kan ikke gi stier. Det er en per-checkout brukerinnstilling.

Det passer regelen fra toppen: dette gjelder deg, i denne checkouten.

### Sjekkes på nytt hver gang

Local-fila kan være uker gammel, og treet den peker på kan agenten skrive til. Derfor sjekker cplt hver lagrede oppføring ved *hver* oppstart, ikke bare når du skriver den. Oppstarten stopper, i stedet for å stille droppe oppføringen, hvis den

- ikke er toppen av et git-repo
- ikke ligger inni prosjektkatalogen
- har en symlink som siste ledd (en symlinket forelder er greit)
- inneholder `..`
- er hjemmekatalogen eller en annen utrygg rot

Feilmeldingen sier hvilken fil oppføringen kom fra, så du vet hva du skal rydde. Det finnes ingen `cplt config unset`; fjerning er `config set … --unset`:

```sh
cplt config set --local sandbox.repo_dirs ~/src/spleis/libs/sykepenger-model --unset   # fjern ett element
cplt config set --local sandbox.repo_dirs --unset                                      # fjern hele nøkkelen
```

---

## Flyttet eller byttet checkout

Local-fila husker hvilken `origin` den ble skrevet for. Står det et annet repo på samme sti nå, advarer cplt (også med `--quiet`) og bruker ingenting fra fila. Vil du ta den i bruk igjen, setter du en hvilken som helst nøkkel med `--local` på nytt. En checkout du har flyttet etterlater en foreldreløs fil; `cplt config local list` viser den som «path no longer exists».

---

## Kom i gang

Kotlin-tjeneste med nested bibliotek, Gradle mot GitHub Packages og Testcontainers:

```sh
cd ~/src/spleis

# Det som er ditt, i denne checkouten
cplt config set --local allow.read ~/.gradle/gradle.properties
cplt config set --local sandbox.allow_docker true --force
cplt config set --local sandbox.repo_dirs ~/src/spleis/libs/sykepenger-model

# Det teamet trenger, committet
cplt config set --repo allow.localhost 8080
cplt trust accept --all

# Sjekk resultatet
cplt config show
cplt config get sandbox.repo_dirs
```

Start så agenten gjennom nav-pilot som før. Oppstartssammendraget viser `Local:`-raden og begge repoene.

Full referanse i [docs/configuration.md](https://github.com/navikt/cplt/blob/main/docs/configuration.md).
