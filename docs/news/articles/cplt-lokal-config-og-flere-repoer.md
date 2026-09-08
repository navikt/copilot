---
title: "cplt: global, local eller repo, og flere repoer i samme sesjon"
date: 2026-09-08
author: starefossen
category: praksis
excerpt: "Én regel for å velge mellom repo, local og global i cplt, og hvorfor det meste hører hjemme i .cplt.toml. Pluss sandbox.repo_dirs, som lar agenten jobbe mot et nested repo i samme sesjon."
tags:
  - cplt
  - config
  - sandbox
  - nav-pilot
---

Du starter agenten med [nav-pilot](/nav-pilot), og nav-pilot kjører den trygt i en sandbox. Første gang du merker [cplt](/cplt), er som regel når noe blir stoppet: Gradle får ikke lese `~/.gradle/gradle.properties`, Testcontainers finner ikke docker, `npm install` nekter å kjøre postinstall. Da må du justere sandboxen, og spørsmålet blir: skal innstillingen settes globalt, lokalt eller i repoet?

Dette innlegget svarer på det, og viser hvordan du lar agenten jobbe mot to repoer i samme sesjon. Alt gjøres med `cplt config` uten at du trenger å redigere config-filer for hånd.

Det du setter med `cplt config`, plukker nav-pilot opp neste gang du starter en agent.

---

## Fire lag, og repoet

cplt slår opp en innstilling i denne rekkefølgen. Første treff vinner for enkeltverdier. Lister (`allow.read`, `allow.write`, `allow.ports`, `allow.localhost`, `deny.paths`) slås sammen på tvers av lagene.

| Lag | Gjelder for | Settes med | Ligger i |
| --- | --- | --- | --- |
| Flagg | denne ene kjøringen | `cplt --allow-write …` | ingenting lagres |
| **Local** | én checkout | `cplt config set --local …` | `~/.config/cplt/local/<hash>.toml` |
| **Global** | hele maskinen din | `cplt config set …` | `~/.config/cplt/config.toml` |
| Defaults | alle | preset `standard` | innebygd |

`.cplt.toml` i repoet er et femte sted, og det er stedet du bør begynne. Fila står utenfor rekkefølgen over og har sin egen tillitsmodell, fordi den er prosjektets innstillinger og ikke dine: den er sporet i git, går gjennom review i en PR, og en ny på teamet får riktig sandbox ved å klone.

- `[deny]` gjelder med en gang. Repoet kan alltid stramme inn.
- `[propose]` er forslag om å utvide, og hver utvikler må godkjenne dem på sin maskin med `cplt trust`. Godkjenningen er bundet til innholdet i fila. Endres fila, må du godkjenne på nytt.
- cplt leser fila fra git HEAD, ikke fra working tree. Agenten kan altså ikke redigere sin egen sandbox-config og få det til å gjelde.

Regelen for å velge lag er ett spørsmål: **hvem gjelder dette for?**

- Prosjektet, altså alle som sjekker det ut → repo-config (`.cplt.toml`)
- Meg, i denne checkouten, og ikke nødvendigvis resten av teamet → local
- Meg, uansett hvilket repo jeg står i → global
- Ingen spesielle → la default stå

Det meste prosjektet trenger, hører hjemme i repoet. Det gjelder også om du er alene på prosjektet: behovet er fortsatt prosjektets, og det følger med til neste maskin du kloner på. Local er for det som er annerledes på *din* maskin: en sti som bare finnes hos deg, et verktøy ikke alle på teamet bruker.

---

## Hvorfor ikke bare global

Det er fristende å sette alt globalt, så slipper du å gjøre det igjen. Ikke gjør det. En grant i global følger deg inn i hver eneste sesjon, også i repoer der stien betyr noe helt annet. Gir du `~/.gradle/gradle.properties` globalt, kan en agent i et Node-prosjekt lese GitHub Packages-tokenet ditt uten å ha noen grunn til det. Setter du det lokalt, gjelder det bare den Kotlin-tjenesten som faktisk bygger med Gradle.

Global skal være liten: ting som er sanne for maskinen din uansett prosjekt. Signerer du commits med GPG, er det et godt eksempel. Det følger deg, ikke prosjektet:

```sh
cplt config set sandbox.allow_gpg_signing true --force
```

Da kan du lure på hvorfor local får utvide sandboxen i det hele tatt, når repo-config må be om godkjenning for det samme. Svaret er at local-fila ligger utenfor repoet, i `~/.config/cplt/`, og sandboxen nekter skriving dit. Agenten kan ikke skrive sine egne grants. En fil i working tree kan ikke love det samme, og derfor må alt som utvider i `.cplt.toml` gjennom `cplt trust`.

---

## Skillet er håndhevet

Grensen mellom repo og local er ikke en konvensjon du må huske. `.cplt.toml` nekter nøkler som handler om maskinen din, og sier hvorfor:

```
$ cplt config set --repo sandbox.pass_env FOO
[cplt] sandbox.pass_env is not valid in repo config.
  Reason: environment variables are machine-specific, not project policy.
  Use: cplt config set sandbox.pass_env FOO
```

`sandbox.use_bubblewrap` får samme svar («depends on bwrap being installed locally, not project policy»), og det samme gjør `sandbox.repo_dirs`, som vi kommer til. Mønsteret er at repo-config tar imot det som er sant for prosjektet, og avviser det som er sant for en maskin. Er du i tvil om hvor noe hører hjemme, prøv `--repo` og les svaret.

---

## Hvor hører dette hjemme?

**Appen lytter på en localhost-port.** Localhost er blokkert som standard. Lytter appen på 8080, gjør den det for alle på teamet, så porten hører hjemme i repoet:

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

Begge havner under `[propose]`. Den som reviewer PR-en ser at prosjektet ber om docker, og hver utvikler godkjenner på sin maskin. Det er poenget med å legge det i repoet, ikke en omvei.

**npm lifecycle scripts** (`postinstall` og venner) er blokkert som standard, fordi de er vilkårlig kodekjøring. Feiler `npm install` uten dem, feiler det for alle, så også dette er prosjektets. Det er samtidig det tyngste du kan be teamet godkjenne, og nettopp derfor hører det hjemme i en PR og ikke i hver utviklers local-fil:

```sh
cplt config set --repo sandbox.allow_lifecycle_scripts true --force
```

**Hemmeligheter som aldri skal inn i en agent-sesjon.** `[deny]` er den billige halvdelen av repo-config. Den krever ingen godkjenning og gjelder for alle som sjekker ut repoet, i det øyeblikket fila er committet:

```sh
cplt config set --repo deny.env VAULT_TOKEN
```

Ingen `cplt trust`, ingenting å vente på. Det er den ene linja i dette innlegget som ikke koster noen noe.

Skal du sette opp et repo fra bunnen, skanner `cplt init` prosjektet og skriver fila for deg:

```sh
cplt init            # se hva som detekteres
cplt init --write    # skriv .cplt.toml, commit den
```

**Gradle trenger GitHub Packages-credentials** fra `~/.gradle/gradle.properties`. Prosjektet trenger tokenet, men fila ligger i hjemmekatalogen din, og det er din maskin som avgjør hvor credentials bor. Det er en sti som finnes hos deg, altså local:

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

Listeverdier vises sammenslått uten lag-merking. En `allow.read` du satte lokalt, får derfor ikke `(local)` etter seg i `config show`.

Ved oppstart viser cplt hvilken local-fil som er i bruk. `Local:`-raden vises bare når en slik fil faktisk gjelder:

```
[cplt] Project:  ~/src/spleis
[cplt] Local:    /Users/x/.config/cplt/local/3291….toml
```

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

Kjører du cplt direkte og bare vil prøve, finnes det et flagg. Det gjelder for den ene kjøringen og skriver ingenting til fila. Flagget kommer i tillegg til det som allerede er lagret, ikke i stedet for:

```sh
cplt --repo-dir ~/src/spleis/libs/sykepenger-model exec -- ./gradlew build
```

Går du via nav-pilot, er `--local` den eneste veien. Der er det ingen kommandolinje å sette flagget på.

### Bare nested, ikke sibling

Repoet må ligge inni prosjektkatalogen. Ligger `sykepenger-model` som `~/src/sykepenger-model`, ved siden av `spleis`, får du nei:

```
[cplt] --repo-dir /path/to/sibling is outside the project directory
  /path/to/launch-repo
  sibling repositories are not yet supported; use `--allow-write` for edit-only.
```

For sibling-repoer er svaret i dag `--allow-write`. Agenten får redigere filene, men ikke gh-identitet mot repoet:

```sh
cplt --allow-write ~/src/sykepenger-model
```

Du kommer ikke rundt dette ved å starte fra `~/src` med `--project-dir`. `--repo-dir` krever at du starter fra toppen av et git-repo, og en katalog som bare inneholder repoer er ikke det. Ordentlig sibling-støtte er [navikt/cplt#344](https://github.com/navikt/cplt/issues/344).

Du får heller ikke navngi repoet du startet i. Det er alltid i scope.

### Local, og bare local

`sandbox.repo_dirs` kan bare settes med `--local`. De to andre lagene nekter, med hver sin begrunnelse:

- Global: en repo-liste i global config ville hengt seg på hver eneste sesjon. Dette er per prosjekt, ikke per maskin.
- Repo-config: å løfte andre trær til prosjektnivå er en path grant, og repo-config kan ikke gi stier. Det er en per-checkout brukerinnstilling.

Det er det samme skillet som over. Hvor du har sjekket ut hva, er sant for din maskin, ikke for prosjektet.

### Sjekkes på nytt hver gang

Local-fila kan være uker gammel, og treet den peker på kan agenten skrive til. Derfor sjekker cplt hver lagrede oppføring ved *hver* oppstart, ikke bare når du skriver den. Er noe galt, stopper oppstarten i stedet for å droppe oppføringen i stillhet. Det skjer hvis oppføringen

- ikke er toppen av et git-repo
- ikke ligger inni prosjektkatalogen
- har en symlink som siste ledd (en symlinket forelder er greit)
- inneholder `..`
- er hjemmekatalogen eller en annen utrygg rot

Feilmeldingen sier hvilken fil oppføringen kom fra, så du vet hva du skal rydde. Det finnes ingen `cplt config unset`. Du fjerner med `config set … --unset`:

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
