---
title: "nav-pilot installerer MCP-servere, og cplt init foreslår det repoet trenger"
date: 2026-09-29
author: starefossen
category: nav-pilot
cli: true
excerpt: "nav-pilot mcp enable installerer MCP-servere fra Navs register og åpner sandkassen for dem når du sier ja. cplt init foreslår riktige tillatelser for Gradle, Go-tester og lokale databaser. Nye guider for oppsett, feilmeldinger, Kotlin og worktrees. opencode er standardklient for nye installasjoner."
tags:
  - nav-pilot
  - cplt
  - mcp
  - opencode
---

Oppgrader begge, og sjekk repoet ditt på nytt:

```bash
brew upgrade navikt/tap/nav-pilot navikt/tap/cplt   # skriptinstallasjon: nav-pilot upgrade && cplt update
cplt init                          # i repoet: viser hva cplt foreslår nå, skriver ingenting
nav-pilot mcp list                 # MCP-serverne dine, og hva som stopper dem
nav-pilot mcp enable figma-mcp     # installer en server fra Navs MCP-register
```

Det som står under, gjelder nav-pilot og cplt fra 29. september 2026.

## MCP-servere med nav-pilot

**`nav-pilot mcp enable <navn>...`** installerer én eller flere servere fra Navs MCP-register. Navnet er det fulle registernavnet eller delen etter siste skråstrek (`figma-mcp`). nav-pilot skriver serveren til hver klient du har installert, eller bare én med `--client copilot|opencode`. Andre servere og innstillinger i fila blir stående, og forrige versjon lagres som `<fil>.bak`. Så spør nav-pilot om sandkassen kan slippe gjennom vertene serveren trenger, og sier hva som gjenstår.

**`nav-pilot mcp disable <navn>...`** fjerner serveren og stenger verter som ingen annen server trenger.

**`nav-pilot mcp list`** viser serverne i registeret og hvilke du har satt opp i Copilot CLI og opencode. Problemer står først, hver med kommandoen som løser dem: et navn organisasjonen blokkerer, en vert eller localhost-port cplt stopper, en pakkekjører som mangler, eller en server som er tatt ut av registeret. Kommandoen endrer ikke oppsettet ditt.

Når du starter under cplt, slår nav-pilot opp MCP-serverne dine i registeret og spør én gang om sandkassen kan nå vertene deres. Vertene kommer bare fra registeret, aldri fra MCP-oppsettet ditt eller fra en `opencode.json` i repoet. Enter betyr nei. Kommer det nye verter til senere, spør nav-pilot igjen. Servere på localhost, som IntelliJ og aksel-arcade, slipper aldri gjennom automatisk; `nav-pilot doctor` viser `cplt config set allow.localhost <port>` for dem. Vil du slippe spørsmålet: `nav-pilot config set mcp_hosts off`.

[Verktøy](/verktoy) viser nå `nav-pilot mcp enable <navn>` som første vei for hver MCP-server. Oppsettet for VS Code, IntelliJ og Copilot CLI ligger under «Manuelt oppsett».

## nav-pilot og cplt

`cplt init` foreslår nå det de fleste repoer trenger:

- **Gradle** får `allow_localhost_any`. Gradle snakker med daemonen sin over localhost, og uten den feilet hvert bygg med «Could not connect to the Gradle daemon», også med `--no-daemon`.
- **Go-tester med `httptest`** får `allow_localhost_any`, fordi testserveren lytter på en tilfeldig port.
- **Porter fra compose og Postgres** blir `allow.localhost`, ikke `allow.ports`. `allow.ports` åpnet porten mot alle eksterne verter, mens tjenesten på din egen maskin fortsatt var stengt.
- **En Dockerfile alene** gir ikke lenger forslag om Docker. Det gjør bare en compose-fil eller Testcontainers, som nå også oppdages i `pom.xml`, `package.json`, `go.mod` og Python-prosjekter.
- **`@navikt`-pakker fra GitHub Packages** gir et hint: tokenet ligger i `~/.npmrc`, som sandkassen stenger. Hintet viser de to kommandoene som åpner for det.
- **mise** gir beskjed om å kjøre `mise install` utenfor sandkassen før du starter agenten.

Rettelser i cplt:

- `cplt exec` virker med mise-shims. `cplt exec -- go version` kjørte mise i stedet for Go.
- `cplt exec` starter i katalogen du står i, når den er inne i prosjektet. Før startet den alltid i roten, så `go test ./...` testet hele repoet.
- `cplt check net` gir riktig råd for navn som peker til localhost, som `127.0.0.1.nip.io`.
- Dokumentasjonen forklarer hvorfor pnpm legger lageret sitt i repoet under cplt, og hvordan du flytter det.

nav-pilot og cplt fungerer bedre sammen:

- **Vertslisten kommer fra cplt.** nav-pilot spør cplt om vertene den slipper gjennom, i stedet for å holde sin egen kopi. Kopien manglet blant annet `plugins-artifacts.gradle.org`. Med en eldre cplt brukes kopien, og `nav-pilot doctor` og `nav-pilot upgrade` sier fra når cplt er utdatert.
- **Agenten gir riktige råd om sandkassen.** Den leser `$CPLT_BRIEF` når den finnes, og ber deg ellers kjøre `cplt config show` utenfor sandkassen. Den har en tabell over vanlige blokkeringer og nøkkelen som løser hver av dem.
- **`nav-pilot config sandbox` beholder innstillingene dine.** Før slettet et trykk på Enter alt du hadde slått på. Nå er innstillingene du har, valgt på forhånd, og bare endringer skrives. Innstillinger som svekker sandkassen, må du bekrefte, og veiviseren sier hva de åpner for.
- **`nav-pilot doctor`** foreslår `cplt init` når repoet mangler `.cplt.toml`. Den advarer ikke lenger om private verter du allerede har åpnet, eller om en merknad fra naisdevice. Filer du har endret selv, gir en advarsel, ikke en feil.
- **Private verter** skrives som én `cplt config set`-linje per vert. cplt avviser en kommaseparert liste.
- **Manglende filer forsvinner ikke lenger.** `nav-pilot sync` uten `--apply` merket en manglende fil som ignorert, så den ble borte fra oversikten. Nå viser `nav-pilot doctor` hvilke filer som mangler og `nav-pilot install`-kommandoen som legger dem tilbake. `nav-pilot sync --apply` bekrefter fortsatt at du slettet en fil med vilje.
- **Oppstarten feiler ikke på en treg første kjøring.** En versjonssjekk med to sekunders tidsgrense kunne stoppe oppstarten med «still running after 2s». Nå får klienten de 30 sekundene den skal ha.
- **nav-pilot tilbyr ikke lenger rtk.** rtk-hooken gjorde `pnpm exec tsc --version` om til en full typesjekk, og agenten kjørte den om igjen til løkkevakten stoppet den. `nav-pilot doctor` sier fra om rester av rtk og viser kommandoen som fjerner dem, men sletter ingenting selv.

## Nye guider på ki-utvikling

- [Sett opp cplt i et repo](/nav-pilot/guider/cplt-oppsett#forste-gang): `cplt init`, gjennomgang, commit av `.cplt.toml` og `cplt trust accept`, med en [tabell over hva hver stakk trenger](/nav-pilot/guider/cplt-oppsett#stakker).
- [Feil i sandkassen](/nav-pilot/guider/cplt-feilmeldinger): feilmeldinger fra cplt med årsak og kommando. [«Could not connect to the Gradle daemon»](/nav-pilot/guider/cplt-feilmeldinger#gradle-connect) er lett å finne også når bare daemon-loggen vises.
- [Kotlin og Gradle i sandkassen](/nav-pilot/guider/cplt-gradle): daemonen, GitHub Packages, interne Nav-verter, JDK-er og Testcontainers.
- [Worktrees med nav-pilot og cplt](/nav-pilot/guider/worktrees): å starte i et worktree, worktrees for underagenter og `--bare`-repoer.
- [Feilsøking](/nav-pilot/guider/feilsoking#kjernen) har fått en del om blokkeringer i kjernen og `cplt check path|net|exec`.

Guidene og feilsøkingssiden har en boks øverst som viser hvordan du oppgraderer. Eksemplene for lokal Postgres bruker nå `localhost` i stedet for `ports`. [cplt](/cplt), [Kom i gang](/kom-i-gang) og [cplt på Windows](/cplt/windows) flyter ikke lenger over på mobil.

## opencode er standardklient for nye installasjoner

```bash
nav-pilot --client opencode           # én økt
nav-pilot config set client opencode  # for godt
nav-pilot config set client copilot   # tilbake til Copilot CLI
```

Installerer du nav-pilot for første gang, er opencode forhåndsvalgt. Mangler den og du har Homebrew, tilbyr nav-pilot å installere den med `brew install anomalyco/tap/opencode`. Ellers får du Copilot CLI og beskjed om hvordan du bytter.

Har du brukt nav-pilot før, beholder du klienten du har. En `~/.nav-pilot/config.toml` uten `client` betyr Copilot CLI, som fortsatt er fullt støttet. CI og kjøring uten terminal bruker Copilot CLI når `config.toml` mangler.

Hvorfor opencode:

- **Åpen kildekode.** Vi kan lese koden og melde feil der de hører hjemme.
- **Passer med resten.** opencode virker med mange modeller og leverandører, og med agentpakkene, hooks og Navs MCP-register. Maskering av hemmeligheter og fødselsnumre og løkkevakten gjelder som i Copilot CLI. Med `--pure` kjører ingen hooks.
- **Lokale modeller.** Bare i opencode kan hovedagenten kjøre på en skymodell og sende avgrensede jobber til en lokal modell (`local-worker`). Stoppet som griper inn når hovedagenten redigerer for mye selv (`local_dispatch` på `balanced` og `aggressive`), finnes også bare der.

Hva hver klient kan, og hva som mangler i opencode, står på [Klienter](/nav-pilot/klienter).

**Kilder:**

- navikt/copilot, MCP: [#1320](https://github.com/navikt/copilot/pull/1320), [#1322](https://github.com/navikt/copilot/pull/1322), [#1324](https://github.com/navikt/copilot/pull/1324)
- navikt/copilot, nav-pilot og cplt: [#1304](https://github.com/navikt/copilot/pull/1304), [#1311](https://github.com/navikt/copilot/pull/1311), [#1330](https://github.com/navikt/copilot/pull/1330), [#1333](https://github.com/navikt/copilot/pull/1333), [#1332](https://github.com/navikt/copilot/pull/1332), [#1336](https://github.com/navikt/copilot/pull/1336), [#1303](https://github.com/navikt/copilot/pull/1303)
- navikt/cplt: [#633](https://github.com/navikt/cplt/pull/633), [#639](https://github.com/navikt/cplt/pull/639), [#641](https://github.com/navikt/cplt/pull/641), [#632](https://github.com/navikt/cplt/pull/632), [#634](https://github.com/navikt/cplt/pull/634), [#636](https://github.com/navikt/cplt/pull/636), [#638](https://github.com/navikt/cplt/pull/638), [#647](https://github.com/navikt/cplt/pull/647), [#648](https://github.com/navikt/cplt/pull/648), [#650](https://github.com/navikt/cplt/pull/650)
- navikt/copilot, guider: [#1294](https://github.com/navikt/copilot/pull/1294), [#1302](https://github.com/navikt/copilot/pull/1302), [#1306](https://github.com/navikt/copilot/pull/1306), [#1315](https://github.com/navikt/copilot/pull/1315), [#1323](https://github.com/navikt/copilot/pull/1323), [#1313](https://github.com/navikt/copilot/pull/1313), [#1290](https://github.com/navikt/copilot/pull/1290)
- navikt/copilot, opencode: [#1159](https://github.com/navikt/copilot/pull/1159), [#1148](https://github.com/navikt/copilot/pull/1148)
