---
title: "gh guard og git guard: blokkér destruktive kommandoer i sandbox"
date: 2026-05-27
author: starefosen
category: praksis
excerpt: "cplt kan nå blokkere destruktive GitHub- og git-operasjoner når AI-agenter kjører i sandbox. gh guard hindrer sletting og merging, git guard hindrer push."
tags:
  - copilot-cli
  - security
  - sandbox
  - cplt
---

cplt kan nå blokkere destruktive GitHub- og git-operasjoner når AI-agenter kjører i sandbox. gh guard og git guard hindrer agenten i å slette repoer, pushe til main eller merge PR-er på egen hånd.

## Hvorfor dette trengs

Under testing oppdaga vi at agenter kan gjøre uventede ting. Ett eksempel: en agent som fikk i oppgave å rydde opp i stale branches, kjørte `gh pr merge` som del av en «cleanup-rutine» ingen ba om. gh guard og git guard gir deg kontroll over hva agenten faktisk kan gjøre mot GitHub og git.

## Slik aktiverer du det

Legg til i `~/.config/cplt/config.toml`:

```toml
[gh_guard]
enabled = true
mode = "block"

[git_guard]
enabled = true
mode = "block"
```

Eller som CLI-flagg for én enkelt kjøring (overstyrer config):

```sh
cplt --gh-guard --git-guard
```

### Tre enforcement modes

| Mode    | Oppførsel                                  |
| ------- | ------------------------------------------ |
| `block` | Kommandoen stoppes (default når aktivert)  |
| `warn`  | Skriver advarsel, men lar kommandoen kjøre |
| `audit` | Logger stille uten å forstyrre             |

Start med `audit` for å se hva agentene gjør, gå til `warn`, og skru på `block` når du er trygg.

## Hva gjør gh guard?

gh guard er en default-deny policy engine mellom agenten og `gh`-kommandoer. Den klassifiserer over 150 kommandoer på tvers av 23 grupper i tre nivåer:

- **Read** (list, view, status) — fungerer på tvers av repoer
- **Write** (create, edit, close) — begrensa til repoet agenten jobber i
- **Destruktive** (delete, transfer, lock) — alltid blokkert

Agenten kan lese issues og PR-er fritt, opprette branches i sitt eget repo, men aldri slette noe eller røre andre repoer.

## Hva gjør git guard?

git guard blokkerer `git push`, `request-pull` og `send-pack`. Alt annet — commit, branch, rebase, stash — fungerer som normalt.

Trenger agenten å pushe til en fork? Legg til et unntak i config:

```toml
[[git_guard.allow_push]]
remote = "fork"
branches = ["agent/*"]
force = false
```

## Hva ser agenten når noe blokkeres?

```
⛔ sandbox restriction: `gh pr merge` is not allowed.
This command is classified as destructive and blocked by gh guard.
Please report this to the user and suggest an alternative approach.
```

Agenten får beskjed om å rapportere tilbake til deg — ingen retry-loops.

## Under panseret

- **Token-isolasjon**: Tokenet slettes fra filsystemet etter første lesing. Subprosesser kan ikke nå det.
- **API-scoping**: `gh api`-kall begrensa til `/repos/{current-repo}/...`. Org-level og cross-repo blokkeres.
- **Sikkerhetsmodell**: Policy bakes inn i wrapper-scriptet ved sandbox-oppstart. Agenten kan ikke endre reglene etter start.

## Rollout-plan

1. **Nå**: Opt-in — du aktiverer selv
2. **Neste fase**: Default `true` med `mode = "warn"`
3. **Deretter**: `mode = "block"` som default

---

Se [PR #67](https://github.com/navikt/cplt/pull/67) for implementasjonsdetaljer og full testdekning.
