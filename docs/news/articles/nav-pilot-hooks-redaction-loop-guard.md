---
title: "Nav-pilot varsler tool-looper og maskerer sensitive tool-resultater"
date: 2026-09-24
author: starefossen
category: nav-pilot
excerpt: "To hooks for Copilot CLI varsler gjentatte tool-kall og maskerer utvalgte secrets og fødselsnumre i tool-resultater. De feiler åpent."
tags:
  - nav-pilot
  - hooks
  - security
  - copilot-cli
---

Nav-pilot installerer nå to hooks når du starter Copilot CLI. Den ene oppdager tool-looper. Den andre maskerer kjente secrets og gyldige fødsels-, D- og H-numre i tool-resultater før modellen mottar dem.

Hookene kjører lokalt etter tool-kallet. De endrer ikke filene eller kommandoresultatet på maskinen, bare teksten som sendes videre til modellen.

## Gjentatte kall utløser et varsel

Loop-guard ser på både tool-kallet og resultatet:

- Samme kall med samme resultat varsles etter fire gjentakelser som standard.
- Samme kall med skiftende resultat varsles etter åtte gjentakelser.

Forskjellen gjør at en poll som faktisk endrer seg kan fortsette lenger enn en agent som får samme svar om igjen. Ved treff får modellen resultatet sammen med en beskjed om å skifte tilnærming. En hook kan ikke avslutte turen selv.

Grensene styres av `local_loop_guard`. Hooken er slått på som standard og kan deaktiveres slik:

```bash
nav-pilot config set hook_loop_guard false
```

## Tool-resultater maskeres før modellen ser dem

Maskeringshooken erstatter blant annet disse verdiene:

- GitHub-tokens, AWS access key-ID-er, private nøkler og JWT-er
- verdier i vanlige secret- og passordfelter
- gyldige fødselsnumre, D-numre og H-numre

Tre innstillinger er slått på som standard:

- `hook_redact_secrets`
- `hook_redact_fnr`
- `hook_injection_note`

Den siste legger en advarsel foran tool-resultater som ligner prompt injection, for eksempel «ignore previous instructions» eller kjente rollemarkører. Resultatet blokkeres ikke.

## Mindre skjult kontekst

Nav-pilot pekte tidligere Copilot CLI mot hele `~/.copilot` som kilde for egne instruksjoner. Copilot søkte rekursivt og lastet også kopier fra gamle session-worktrees.

På én maskin økte dette antallet instruksjonskilder fra 18 til 82. Statisk kontekst vokste fra 18,9k til 45,2k tokens. Nav-pilot peker nå direkte på `~/.copilot/.github/instructions`.

Fiksen fjerner de ekstra instruksjonskopiene fra konteksten. På den målte maskinen kom den statiske konteksten under 32k tokens, men samtalen og tool-resultatene trenger også plass.

## Begrensninger

- Hookene installeres bare for Copilot CLI. Opencode støttes ikke ennå.
- De feiler åpent. Hvis payload, config eller state ikke kan leses, går resultatet videre.
- Maskeringen dekker et konservativt utvalg mønstre, ikke alle typer secrets.
- Bare tool-resultater kan omskrives. Brukerprompter dekkes ikke.
- Prompt-injection-sjekken kjenner bestemte fraser. Den erstatter ikke sandboxing eller vanlig kildekritikk.

Oppgrader nav-pilot og start en ny Copilot CLI-session for å få hook-filene.

**Kilder:**

- [Result-aware loop guard for every Copilot CLI session](https://github.com/navikt/copilot/pull/939) (navikt/copilot, 24. september 2026)
- [Redact secrets and fødselsnummer in tool results](https://github.com/navikt/copilot/pull/940) (navikt/copilot, 24. september 2026)
- [Scope custom instructions to nav-pilot's directory](https://github.com/navikt/copilot/pull/932) (navikt/copilot, 23. september 2026)
