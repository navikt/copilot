# 🧭 nav-pilot

nav-pilot er et CLI-verktøy og en AI-agent for Nav-utvikling med GitHub Copilot.

📖 **Online docs (primær):** https://ki-utvikling.nav.no/nav-pilot  
📝 **Endringslogg:** [docs/nav-pilot-changelog.md](nav-pilot-changelog.md)

## Kom i gang

```bash
# Installer CLI
brew install navikt/tap/nav-pilot

# I et repo
nav-pilot
nav-pilot install kotlin-backend
```

## Vanlige kommandoer

```bash
nav-pilot list --installed
nav-pilot sync
nav-pilot upgrade
nav-pilot feedback
```

## Personlig installasjon (valgfritt)

```bash
nav-pilot install --user --all
eval "$(nav-pilot env)"
```

## For bidragsytere

- Agent: `.github/agents/nav-pilot.agent.md`
- Skills: `.github/skills/<name>/`
- Instruksjoner: `.github/instructions/`

Detaljert bruk, CLI-referanse og arbeidsflyt vedlikeholdes i online docs.
