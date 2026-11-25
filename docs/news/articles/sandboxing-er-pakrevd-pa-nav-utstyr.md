---
title: "Sandboxing og isolasjon er påkrevd på Nav-utstyr"
date: 2026-08-10
author: starefossen
category: praksis
excerpt: "AI-agenter skal alltid kjøre isolert på Nav-utstyr. Bruk cplt som standard, også når agentarbeidet er personlig."
tags:
  - cplt
  - security
  - sandbox
  - coding-agents
---

Når du bruker en AI-agent på Nav-utstyr, skal agenten kjøre i en sandbox eller
tilsvarende isolasjon. Dette er et krav, ikke et valgfritt sikkerhetstiltak.

Kravet gjelder alt agentarbeid på utstyret:

- Nav-relaterte oppgaver
- arbeid i private repoer
- personlig utforsking og sideprosjekter

Det er utstyret og tilgangene som må beskyttes. At oppgaven er personlig, reduserer
ikke risikoen for at agenten får tilgang til filer, hemmeligheter,
tilgangsinformasjon eller interne tjenester på maskinen.

## Oppdatert 25.11.2026: Agenter i VS Code og IntelliJ

GitHub Copilot i VS Code og JetBrains (IntelliJ) har unntak fra kravet om
sandbox. Du kan fortsatt bruke Agent mode og nav-pilots agentpakke der, så lenge
du godkjenner terminalkommandoene selv. Unntaket gjelder lett og moderat
agentbruk.

Full auto er ikke tillatt: ingen «Allow all», global auto-godkjenning eller
Autopilot. Agenter i terminalen, som Copilot CLI, opencode og pi, skal fortsatt
kjøre i cplt.

**VS Code**

- Skal være av: `chat.tools.global.autoApprove`.
- Ikke velg **Allow all** eller **Autopilot** i velgeren for tillatelser.
- Ikke legg inn egne regler med `true` i `chat.tools.terminal.autoApprove`
  som godkjenner alle eller brede kommandomønstre.
- Anbefalt på: `chat.agent.sandbox.enabled` satt til `"on"` på macOS 15 eller
  nyere og Linux. Innstillingen sandboxer terminalkommandoer og MCP-servere,
  men ikke redigering av filer, og den stopper ikke nettverk som standard.

**JetBrains (IntelliJ)**

- Skal være av: **Settings > GitHub Copilot > Chat > Auto Approve > Global
  Auto Approve**.
- Ikke slå på auto-godkjenning for kommandoer og filendringer som ingen regel
  dekker, i samme seksjon.
- Bruker du Copilot CLI-agenten i IDE-en: ikke velg **Bypass** eller
  **Autopilot**.
- Slå på lokal sandboxing hvis IDE-en din har det.

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

Aliasene gjelder bare i terminalen. For VS Code og JetBrains gjelder unntaket
over. Andre klienter må bruke klientens sandboxing eller en annen
isolasjonsmekanisme.

## Hvis du ikke bruker cplt

Du må selv sette deg inn i hvordan agentklienten isolerer agenten, aktivere denne
funksjonen og forstå hva den faktisk beskytter. Hvis klienten ikke gir
tilstrekkelig beskyttelse, må du bruke en annen mekanisme, for eksempel en VM
eller container.

Vi går ikke nærmere inn på oppsett av alternative løsninger her. Hovedregelen er
enkel: Ikke kjør en AI-agent med ubegrenset tilgang til Nav-utstyret.

## Hvorfor godkjenning ikke er nok

En agent kan lese innhold den ikke bør stole på, kjøre kommandoer og installere
avhengigheter. Prompt injection og kompromitterte pakker kan påvirke hva agenten
gjør. Bekreftelsesdialoger og gode instruksjoner reduserer risiko, men erstatter
ikke en teknisk sikkerhetsgrense.

Sandboxing begrenser konsekvensene når agenten, et verktøy eller en avhengighet
gjør noe uventet. Derfor skal isolasjonen være på plass før agenten starter.

## Kortversjonen

**På Nav-utstyr skal AI-agenter alltid kjøre isolert. Bruk cplt. Velger du noe
annet, er du ansvarlig for å sikre tilsvarende isolasjon. Kravet gjelder både
Nav-relatert og personlig agentarbeid. Unntak: Copilot i VS Code og IntelliJ,
med godkjenning av terminalkommandoer og uten full auto.**

**Kilder:**

- [cplt-dokumentasjonen](/cplt) (Nav)
- [cplt threat model](https://github.com/navikt/cplt/blob/main/SECURITY.md) (GitHub)
- [Agent sandboxing](https://code.visualstudio.com/docs/agents/run/agent-sandboxing) (VS Code, 18. november 2026)
- [Approvals and permissions](https://code.visualstudio.com/docs/agents/run/approvals) (VS Code)
- [Auto-approve in Copilot for JetBrains](https://github.blog/changelog/2026-04-24-inline-agent-mode-in-preview-and-more-in-github-copilot-for-jetbrains-ides/) (GitHub, 24. april 2026)
- [Cloud and local sandboxes for GitHub Copilot](https://github.blog/changelog/2026-06-02-cloud-and-local-sandboxes-for-github-copilot-now-in-public-preview) (GitHub, 2. juni 2026)
