# Nyhetssaker

Hver fil i `articles/` er én sak på [ki-utvikling.nav.no/nyheter](https://ki-utvikling.nav.no/nyheter). Filnavnet uten `.md` blir adressen, så bruk bare små bokstaver, tall og bindestrek.

## Front matter

```yaml
---
title: "Tittelen på saken"
date: 2026-09-25
author: github-brukernavn
category: nav-pilot
excerpt: "Én til to setninger som står i lista."
tags:
  - nav-pilot
---
```

| Felt       | Påkrevd | Hva det gjør                                                                                                |
| ---------- | ------- | ----------------------------------------------------------------------------------------------------------- |
| `title`    | ja      | Overskriften.                                                                                               |
| `date`     | ja      | Publiseringsdato, `ÅÅÅÅ-MM-DD`. Lista sorteres på den.                                                      |
| `excerpt`  | ja      | Ingressen i lista. Kommer med i `news.json` som `summary`, men nav-pilot viser bare tittel og lenke.        |
| `category` | nei     | `copilot`, `nav`, `nav-pilot`, `praksis` eller `oppsummering`. Ukjent eller manglende verdi blir `copilot`. |
| `tags`     | nei     | Emneord.                                                                                                    |
| `author`   | nei     | GitHub-brukernavnet til forfatteren.                                                                        |
| `lang`     | nei     | `nb` (standard) eller `en`. Bare norske saker kommer med i `news.json`.                                     |
| `url`      | nei     | Lenke til kilden. En sak med `url` og uten brødtekst blir en lenkesak.                                      |
| `featured` | nei     | `true` løfter saken øverst i lista.                                                                         |
| `draft`    | nei     | `true` holder saken borte fra nettstedet og fra `news.json`.                                                |
| `cli`      | nei     | `true` viser saken i nav-pilot. Se under.                                                                   |

## `cli: true`

Sett `cli: true` bare på saker som er skrevet for dem som bruker nav-pilot. Hver slik sak vises én gang til hver nav-pilot-bruker, som en linje etter en økt, i 30 dager etter `date`. Alle nav-pilot-brukere ser den, så bruk flagget sjelden.

nav-pilot leser de 20 nyeste norske sakene fra [`/news.json`](https://ki-utvikling.nav.no/news.json) og viser bare sakene med `cli: true`. Linja vises ikke uten terminal, i CI, etter Ctrl-C, når nav-pilot allerede har vist noe annet etter økta (en spørreundersøkelse eller et tips), med `news = false` eller når telemetri er slått av. `nav-pilot news` lister de ti nyeste sakene, med eller uten flagget, og regner dem som vist.
