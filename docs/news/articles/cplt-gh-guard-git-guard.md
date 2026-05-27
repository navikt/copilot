---
title: "gh guard og git guard: blokkér destruktive kommandoer i sandbox"
date: 2026-05-27
author: starefosen
category: praksis
excerpt: "cplt kan nå blokkere destruktive GitHub- og git-operasjoner når AI-agenter kjører i sandbox. Én kommando skrur det på."
tags:
  - copilot-cli
  - security
  - sandbox
  - cplt
---

cplt kan nå blokkere destruktive GitHub- og git-operasjoner når AI-agenter kjører i sandbox. Én kommando skrur det på — ingen manuell config-redigering.

## Problemet

Agenter kan gjøre uventede ting. Ett eksempel: en agent som fikk i oppgave å rydde opp i stale branches, kjørte `gh pr merge` som del av en «cleanup-rutine» ingen ba om.

## Kom i gang

```sh
cplt config set gh_guard.enabled true
cplt config set git_guard.enabled true
```

Det er alt. Neste gang du kjører `cplt` blokkeres destruktive operasjoner automatisk.

### Vil du teste forsiktig først?

Start i audit-modus for å se hva agenten prøver å gjøre, uten å blokkere noe:

```sh
cplt config set gh_guard.mode audit
cplt config set git_guard.mode audit
```

Gå til `warn` når du vil se advarsler, og `block` når du er klar til å håndheve:

```sh
cplt config set gh_guard.mode block
cplt config set git_guard.mode block
```

### Engangskjøring uten å endre config

CLI-flagg overstyrer config for én enkelt kjøring:

```sh
cplt --gh-guard --git-guard
```

## Hva blokkeres?

### gh guard — trelagsmodell

gh guard er en default-deny policy engine som klassifiserer over 150 `gh`-kommandoer i tre nivåer:

| Nivå           | Eksempler                         | Tilgang             |
| -------------- | --------------------------------- | ------------------- |
| **Read**       | `gh issue list`, `gh pr view`     | Fungerer fritt      |
| **Write**      | `gh pr create`, `gh issue edit`   | Bare i eget repo    |
| **Destruktiv** | `gh repo delete`, `gh pr merge`   | Alltid blokkert     |

`gh api`-kall begrenses til `/repos/{current-repo}/...`. Org-level og cross-repo API-tilgang blokkeres.

### git guard — push-beskyttelse

git guard blokkerer `git push`, `request-pull` og `send-pack`. Alt annet — commit, branch, rebase, stash — fungerer som normalt.

Trenger agenten å pushe til en fork?

```sh
cplt config set git_guard.protect_default_branch_only true
```

Eller legg til et strukturert unntak i `~/.config/cplt/config.toml`:

```toml
[[git_guard.allow_push]]
remote = "fork"
branches = ["agent/*"]
force = false
```

## Hva ser agenten?

```
⛔ sandbox restriction: `gh pr merge` is not allowed.
This command is classified as destructive and blocked by gh guard.
Please report this to the user and suggest an alternative approach.
```

Agenten får beskjed om å rapportere tilbake til deg — ingen retry-loops.

## Under panseret

- **Token-isolasjon**: Tokenet slettes fra filsystemet etter første lesing. Subprosesser kan ikke nå det.
- **API-scoping**: `gh api`-kall begrensa til `/repos/{current-repo}/...`. Org-level og cross-repo blokkeres.
- **Sikkerhetsmodell**: Policy bakes inn i wrapper-scriptet ved sandbox-oppstart. Agenten kan ikke endre reglene innenfra.

## Anbefalt oppsett

For de fleste utviklere anbefaler vi:

```sh
cplt config set gh_guard.enabled true
cplt config set gh_guard.mode block
cplt config set git_guard.enabled true
cplt config set git_guard.mode block
```

Fire kommandoer, full beskyttelse.

---

Se [PR #67](https://github.com/navikt/cplt/pull/67) for implementasjonsdetaljer og full testdekning.
