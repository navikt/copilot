---
title: "Sandboxing og isolasjon er påkrevd på Nav-utstyr"
date: 2026-08-10
author: starefossen
category: praksis
excerpt: "KI-agenter på Nav-utstyr: start dem med nav-pilot i terminalen. VS Code og IntelliJ er greit med editorens sandkasse på. Aldri Allow all, Autopilot eller global auto-godkjenning."
tags:
  - cplt
  - security
  - sandbox
  - coding-agents
---

Når du bruker en KI-agent på Nav-utstyr, skal agenten kjøre i en sandkasse eller
tilsvarende isolasjon. Dette er et krav, ikke et valgfritt sikkerhetstiltak.

Kravet gjelder alt agentarbeid på utstyret:

- Nav-relaterte oppgaver
- arbeid i private repoer
- personlig utforsking og sideprosjekter

Det er utstyret og tilgangene som må beskyttes. At oppgaven er personlig, reduserer
ikke risikoen for at agenten får tilgang til filer, hemmeligheter,
tilgangsinformasjon eller interne tjenester på maskinen.

## Oppdatert 25.11.2026: Agenter i VS Code og IntelliJ

- Terminal: start agenter med `nav-pilot` (cplt).
- VS Code og IntelliJ: greit. Slå på editorens sandkasse hvis den har en.
- Aldri bruk Allow all, Autopilot eller global auto-godkjenning.

[Slik gjør du det i VS Code og IntelliJ](/nav-pilot/forklaring/sandkassen#ide-klienter)

## Bruk cplt

[`cplt`](/cplt) er den anbefalte og enkleste måten å oppfylle kravet på. cplt
starter agenten med isolasjon som operativsystemet håndhever. Dette begrenser
blant annet tilgangen til filer, tilgangsinformasjon og nettverk.

```bash
brew install navikt/tap/cplt
cplt
```

nav-pilot er laget for å brukes sammen med cplt:

```bash
brew install navikt/tap/nav-pilot navikt/tap/cplt
nav-pilot
```

Bruk cplt som standard fremfor å bygge din egen løsning.

### Valgfrie aliaser

Bruker du zsh eller bash, kan du legge disse aliasene i `~/.zshrc` eller
`~/.bashrc`:

```bash
alias copilot='cplt --'
alias np='nav-pilot'
```

Det første aliaset sender vanlig bruk av `copilot` gjennom cplt. `--` gjør at
Copilot-flagg sendes videre til Copilot CLI. Det andre er bare en kortform for
`nav-pilot`, som allerede bruker cplt.

Aliasene gjelder bare i terminalen. For VS Code og IntelliJ gjelder reglene
over. Andre klienter må bruke klientens sandboxing eller en annen
isolasjonsmekanisme.

## Hvis du ikke bruker cplt

Du må selv sette deg inn i hvordan agentklienten isolerer agenten, aktivere denne
funksjonen og forstå hva den faktisk beskytter. Hvis klienten ikke gir
tilstrekkelig beskyttelse, må du bruke en annen mekanisme, for eksempel en VM
eller container.

Vi går ikke nærmere inn på oppsett av alternative løsninger her. Hovedregelen er
enkel: Ikke kjør en KI-agent med ubegrenset tilgang til Nav-utstyret.

## Hvorfor godkjenning ikke er nok

En agent kan lese innhold den ikke bør stole på, kjøre kommandoer og installere
avhengigheter. Prompt injection og kompromitterte pakker kan påvirke hva agenten
gjør. Bekreftelsesdialoger og gode instruksjoner reduserer risiko, men erstatter
ikke en teknisk sikkerhetsgrense.

Sandboxing begrenser konsekvensene når agenten, et verktøy eller en avhengighet
gjør noe uventet. Derfor skal isolasjonen være på plass før agenten starter.

## Kortversjonen

**På Nav-utstyr skal KI-agenter kjøre isolert, også ved personlig bruk.**

- Terminal: start agenter med `nav-pilot` (cplt).
- VS Code og IntelliJ: greit. Slå på editorens sandkasse hvis den har en.
- Aldri bruk Allow all, Autopilot eller global auto-godkjenning.

**Kilder:**

- [cplt-dokumentasjonen](/cplt) (Nav)
- [cplt threat model](https://github.com/navikt/cplt/blob/main/SECURITY.md) (GitHub)
- [Agent sandboxing](https://code.visualstudio.com/docs/agents/run/agent-sandboxing) (VS Code, 18. november 2026)
- [Approvals and permissions](https://code.visualstudio.com/docs/agents/run/approvals) (VS Code)
- [Auto-approve in Copilot for JetBrains](https://github.blog/changelog/2026-04-24-inline-agent-mode-in-preview-and-more-in-github-copilot-for-jetbrains-ides/) (GitHub, 24. april 2026)
- [Local sandboxes GA in JetBrains](https://github.blog/changelog/2026-10-28-github-copilot-in-jetbrains-universe-releases/) (GitHub, 28. oktober 2026)
- [Cloud and local sandboxes for GitHub Copilot](https://github.blog/changelog/2026-06-02-cloud-and-local-sandboxes-for-github-copilot-now-in-public-preview) (GitHub, 2. juni 2026)
