---
title: "Nais MCP, GitHub og Figma under cplt, uten at innloggingen kommer inn i sandkassen"
date: 2026-10-23
author: starefossen
category: nav-pilot
cli: true
excerpt: "nav-pilot mcp enable slår på Nais MCP med én kommando. cplt kjører serveren på maskinen din, utenfor sandkassen, så agenten får verktøyene uten å se innloggingen. GitHub-serveren virker i OpenCode uten egen innlogging. Det virker likt i Copilot CLI og OpenCode."
tags:
  - nav-pilot
  - cplt
  - mcp
  - opencode
---

<!-- TODO før publisering: sett datoen og cplt-utgaven for Linux når navikt/cplt#838 og #841 er ute. -->

Kom i gang med tre kommandoer:

```bash
brew upgrade navikt/tap/nav-pilot navikt/tap/cplt nais   # skriptinstallasjon: nav-pilot upgrade && cplt update
nav-pilot mcp enable io.github.nais/nais-mcp
nav-pilot mcp list                                        # LOGIN og PATH viser at det virker
```

Start en ny økt etterpå. Detaljene står i [MCP gjennom cplt](/nav-pilot/guider/mcp-gjennom-cplt).

## Hva er nytt

**Nais MCP med én kommando.** Nais MCP leser innloggingen din fra nøkkelringen. Inne i sandkassen fantes ikke nøkkelringen, så serveren svarte «unauthenticated» selv om du var logget inn. Nå kjører cplt serveren på maskinen din, utenfor sandkassen, og agenten når verktøyene gjennom cplt. Er du ikke logget inn, spør nav-pilot om den skal kjøre `nais login -n`. Har du en Nais CLI eldre enn 7.0.2, ber nav-pilot deg oppgradere, men slår på serveren likevel.

**GitHub i OpenCode uten egen innlogging.** `opencode mcp auth` feilet mot GitHub, fordi GitHubs innlogging mangler dynamisk klientregistrering. Nå kobler cplt OpenCode til GitHub-serveren gjennom en proxy på maskinen din, med `gh`-innloggingen din. Sjekk `gh auth status` og start en ny økt.

**Figma** logger du inn på i klienten, ikke gjennom cplt. I OpenCode gjør du det én gang med `opencode mcp auth com.figma/figma-mcp`.

**Likt i begge klienter.** `nav-pilot mcp enable` skriver serveren til Copilot CLI og OpenCode, og `nav-pilot mcp list` viser begge. To nye kolonner: LOGIN sier om du er logget inn, og PATH viser `cplt broker` når cplt kjører serveren.

## Hvorfor det er trygt

- **Innloggingen blir på maskinen din.** Nais-tokenet og `gh`-tokenet kommer aldri inn i sandkassen. Agenten ser verktøyene, ikke tokenet.
- **Bare lesing som standard.** Nais MCP får bare lesende verktøy. Vil du ha verktøy som skriver, ber du om dem med `--tools`, og nav-pilot ber deg godkjenne det. GitHub-proxyen i OpenCode bruker alltid det skrivebeskyttede endepunktet.
- **cplt avviser GraphQL-mutasjoner.** GraphQL-verktøyet i Nais MCP slipper bare gjennom spørringer. cplt avviser mutasjoner og dokumenter med mer enn én operasjon. Fra Nais CLI 7.0.2 sjekker også Nais CLI hver operasjon.
- **cplt kjører ikke en server agenten kan endre.** Ligger oppsettet til serveren i en mappe agenten kan skrive til, nekter cplt å starte den.

Én kjent risiko: kjører du Nais MCP i flere økter samtidig, kan flere prosesser fornye en utløpt innlogging på en gang og i sjeldne tilfeller logge deg ut. Kjør da `nais login -n` på nytt. Vi fant risikoen ved å lese koden til Nais CLI og har ikke sett den i bruk. Den følges opp i [nais/cli#816](https://github.com/nais/cli/issues/816).

## Plattformer

- **macOS** virker med cplt-utgaven fra 23. oktober 2026.
- **Linux** krever bubblewrap (`bwrap`) og cplt-utgaven fra TODO(dato). Uten bubblewrap nekter cplt å kjøre serveren, fordi det er bubblewrap som skjuler nøkkelringen for agenten. `cplt doctor --fix` installerer bubblewrap for deg.
- **Ubuntu 24.04** stopper bubblewrap med AppArmor. Kjør `cplt doctor --fix` én gang. cplt viser kommandoene, spør før den kjører dem med `sudo`, og sjekker på nytt. Kommandoene for hånd står i [guiden](/nav-pilot/guider/mcp-gjennom-cplt#ubuntu-apparmor).
- **WSL2** skal virke med bubblewrap, men er ikke testet på en ekte maskin ennå. Nais CLI trenger en nøkkelring, og WSL2 uten skrivebord har ingen. Guiden viser [hvordan du setter opp gnome-keyring](/nav-pilot/guider/mcp-gjennom-cplt#wsl2-nokkelring).

Virker det ikke, se [feilmeldingene i guiden](/nav-pilot/guider/mcp-gjennom-cplt#feilsoking).

**Kilder:**

- navikt/copilot: [#1939](https://github.com/navikt/copilot/pull/1939), [#1961](https://github.com/navikt/copilot/pull/1961)
- navikt/cplt: [#834](https://github.com/navikt/cplt/issues/834), [#838](https://github.com/navikt/cplt/pull/838), [#841](https://github.com/navikt/cplt/pull/841)
- nais/cli: [#816](https://github.com/nais/cli/issues/816)
