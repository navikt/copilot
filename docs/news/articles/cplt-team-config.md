---
title: "cplt — team-config og auto-generert sandbox"
date: 2026-05-11
author: starefosen
category: praksis
excerpt: "Teams kan nå committe sandbox-innstillinger til repoet, og cplt init skanner prosjektet og genererer riktig config automatisk."
tags:
  - cplt
  - security
  - sandbox
  - config
---

Å sette opp riktig sandbox-config manuelt er kjedelig — og feil config betyr enten for lite tilgang (agenten feiler) eller for mye (sikkerheten svekkes). cplt har nå to funksjoner som løser dette: team-config du committer til repoet, og `cplt init` som genererer configen for deg.

---

## Team-config med `.cplt.toml`

Tidligere måtte hver utvikler sette opp sandbox-flagg selv, enten via kommandolinja eller personlig config. Det førte til at config driftet mellom maskiner og at nye teammedlemmer startet uten beskyttelse.

Nå kan du committe en `.cplt.toml` til rota av repoet. Alle som kjører cplt der får automatisk riktig sandbox-config:

```toml
[deny]
env = ["VAULT_TOKEN", "NPM_TOKEN"]

[propose]
allow_localhost_any = true

[propose.allow]
ports = [5432]
localhost = [3000]
```

Fila har to seksjoner med ulik tillitsmodell. `[deny]` strammes inn automatisk — repoet kan bare fjerne tilgang, aldri gi mer. `[propose]` foreslår utvidelser som krever eksplisitt godkjenning per maskin via `cplt trust`. Godkjenninga er bundet til et content-hash av fila. Endrer noen configen, må du godkjenne på nytt.

cplt leser `.cplt.toml` fra git HEAD, ikke working tree. Agenten kan ikke endre sin egen sandbox-config. I CI bruker du `--accept-repo-config` i stedet for interaktiv godkjenning.

---

## `cplt init` — la verktøyet gjøre jobben

Den vanligste innvendingen mot sandbox-config er at det tar tid å finne ut hva prosjektet faktisk trenger. `cplt init` løser dette ved å skanne prosjektet og generere en `.cplt.toml` med riktige tillatelser.

```bash
cplt init               # forhåndsvis hva som detekteres
cplt init --write       # skriv .cplt.toml til disk
```

15 detektorer gjenkjenner JVM (Gradle/Maven), Node.js, Docker, Python, Rust, Go, Spring Boot, Ktor, Next.js, Vite, Flyway, Playwright, Cypress, TestContainers og `.env`-filer. Hver detektor vet hvilke sandbox-tillatelser økosystemet trenger — et Spring Boot-prosjekt med Flyway får for eksempel localhost 8080 og PostgreSQL-port 5432, mens et Next.js-prosjekt får localhost 3000 og `allow_localhost_any` for Turbopack.

Farlige tillatelser som `allow_docker` får risikovarsel. `allow_lifecycle_scripts` foreslås aldri automatisk, siden det åpner for vilkårlig kodekjøring.

For personlig config kjører du `cplt init --global`, som skanner maskinen for Gradle-wrapper, Playwright-browsere, GPG-signering og alternative agenter. Resultatet skrives til `~/.config/cplt/config.toml`.

Hele init-flyten er dekket av 82 tester — deteksjon, TOML-generering og e2e.

---

## Kom i gang

```bash
# Generer config for prosjektet
cplt init --write

# Se hva repoet ber om
cplt trust

# Godkjenn og kjør
cplt trust accept --all
cplt -- -p "fix the tests"
```

Kjør `cplt config explain` for inline-hjelp, eller les [docs/configuration.md](https://github.com/navikt/cplt/blob/main/docs/configuration.md) for fullstendig referanse.

**Kilder:**

- [Team config med .cplt.toml — PR #32](https://github.com/navikt/cplt/pull/32) (cplt, mai 2026)
- [cplt init — PR #42](https://github.com/navikt/cplt/pull/42) (cplt, mai 2026)
