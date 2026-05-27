---
title: "cplt — sandbox for Copilot CLI"
date: 2026-04-10
author: starefossen
category: praksis
excerpt: "cplt kjører Copilot CLI inne i Apples Seatbelt-sandbox på macOS. Copilot kan jobbe med prosjektet ditt, men kjernen blokkerer tilgang til credentials, secrets og .env-filer."
tags:
  - copilot-cli
  - security
  - supply-chain
  - sandbox
---

`cplt` kjører GitHub Copilot CLI inne i Apples Seatbelt-sandbox på macOS. Copilot kan jobbe med prosjektet ditt, men kjernen blokkerer tilgang til credentials, secrets og `.env`-filer.

## Hvorfor

Copilot CLI kjører vilkårlige kommandoer på maskina di. En ondsinnet `postinstall`-hook eller et prompt injection-angrep kan lese `~/.ssh`, `~/.config/gcloud` og `.env`-filer uten at du merker det.

Supply chain-angrep det siste året viser at dette skjer i praksis:

- **Shai-Hulud** — npm-orm som spredde seg til 700+ pakker via stjålne tokens
- **axios-trojaneren** — kapra npm-pakke som installerte en RAT når en AI-agent kjørte `npm install`
- **CamoLeak** — prompt injection i PR-kommentarer fikk Copilot til å eksfiltrere kode (CVSS 9.6)
- **MCP Poisoning** — skjulte instruksjoner i npm-metadata lurte agenter til å hente ut SSH-nøkler

## Slik stopper cplt angrepskjeden

Et typisk supply chain-angrep har fire steg. cplt bryter kjeden på de tre viktigste:

**Infeksjon** via `postinstall`-hook → cplt injiserer `npm_config_ignore_scripts=true`, så hooks aldri kjører.

**Credential-høsting** fra `~/.ssh`, `~/.config/gcloud`, `.env` → kjernen nekter lesing. Agenten får ikke åpna filene.

**Persistens** via git-hooks eller native moduler → `.git/hooks` og `~/.copilot/pkg` er skrivebeskyttet.

## Kom i gang

```bash
brew install navikt/tap/cplt

cplt --doctor              # sjekk miljøet
cplt -- -p "fix the tests" # kjør Copilot i sandbox
```

Tre kommandoer, så er du i gang. `--doctor` verifiserer at Copilot er installert, at auth fungerer, og at sandboxen er aktiv.

For prosjekter som trenger `.env`-filer eller localhost-tilgang:

```bash
# Next.js med .env-filer
cplt --allow-env-files -- -p "start dev-serveren"

# MCP-server eller dev-server på localhost
cplt --allow-localhost 3000 -- -p "bruk MCP-serveren"

# Next.js/Turbopack (tilfeldige porter)
cplt --allow-localhost-any -- -p "fiks builden"
```

Kjør `cplt --init-config` for å lagre innstillingene permanent.

[github.com/navikt/cplt](https://github.com/navikt/cplt) · [Trusselmodell (SECURITY.md)](https://github.com/navikt/cplt/blob/main/SECURITY.md)
