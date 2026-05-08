---
title: "cplt — team-config med .cplt.toml"
date: 2026-05-08
category: praksis
excerpt: "Teams kan nå committe sandbox-innstillinger til repoet. Alle som kjører cplt i repoet får automatisk riktig config uten manuelle flagg."
tags:
  - cplt
  - security
  - sandbox
  - config
---

## Team-config i repoet

cplt støtter nå `.cplt.toml` — en konfigurasjonsfil du committer til repoet. Alle som kjører cplt i repoet får automatisk riktig sandbox-config uten manuelle flagg eller global config.

Legg fila i rota av repoet:

```toml
[deny]
env = ["VAULT_TOKEN", "NPM_TOKEN"]

[propose]
allow_localhost_any = true

[propose.allow]
read = ["~/.gradle/gradle.properties"]
ports = [8080]
```

Konfigen har to seksjoner med ulik tillitsmodell:

- **`[deny]`** strammes inn automatisk. Ingen godkjenning nødvendig — repoet kan bare fjerne tilgang agenten ellers ville hatt.
- **`[propose]`** foreslår utvidelser som krever eksplisitt godkjenning per maskin.

## Godkjenning med `cplt trust`

Når et repo har `[propose]`-regler, må du godkjenne dem lokalt:

```bash
cplt trust              # vis hva repoet ber om
cplt trust accept --all # godkjenn alt
```

Godkjenninga er bundet til innholdet i fila. Endrer noen `.cplt.toml`, må du godkjenne på nytt — cplt bruker en content-hash og invaliderer automatisk.

I CI trenger du ikke interaktiv godkjenning. Bruk `--accept-repo-config` per kjøring.

## Sikkerhetsmodell

`.cplt.toml` leses fra git HEAD, ikke fra working tree. Agenten kan ikke endre committed state, og `cplt trust` er blokkert inne i sandboxen.

Det betyr:

- Agenten kan ikke endre sin egen sandbox-config
- Endringer i fila krever en commit og ny godkjenning
- CI-kjøringer bruker eksplisitt opt-in per run

## Andre forbedringer

- **stdout for display-kommandoer** — `config show`, `trust` og `--doctor` skriver til stdout. Piping fungerer nå (`cplt trust | pbcopy`).
- **Oppdatert SECURITY.md** — trust-modell og seks nye sikkerhetseksjoner.
- **10 nye e2e-tester** for hele trust-flyten.
- **Linux sha256sum** bruker nå absolutte paths (sikkerhetsfiks).

## Dokumentasjon

Kjør `cplt config explain` for inline-hjelp, eller les [docs/configuration.md](https://github.com/navikt/cplt/blob/main/docs/configuration.md) for fullstendig referanse.
