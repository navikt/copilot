# Nav Copilot

Navs verktøy for KI-assistert utvikling: CLI-et nav-pilot, nettstedet [ki-utvikling.nav.no](https://ki-utvikling.nav.no), MCP-gatewayen fleet-gateway og agenter, instruksjoner og skills for Navs stack.

## Hvor du finner hva

Brukerdokumentasjonen ligger på nettstedet, ikke her:

- [ki-utvikling.nav.no/kom-i-gang](https://ki-utvikling.nav.no/kom-i-gang): kom i gang med KI-assistert utvikling i Nav
- [ki-utvikling.nav.no/nav-pilot](https://ki-utvikling.nav.no/nav-pilot): installer og bruk nav-pilot
- [ki-utvikling.nav.no/verktoy](https://ki-utvikling.nav.no/verktoy): agenter, instruksjoner, skills og MCP-servere
- [ki-utvikling.nav.no/cplt](https://ki-utvikling.nav.no/cplt): cplt, sandkassen for kodeagenter ([navikt/cplt](https://github.com/navikt/cplt))
- [mcp-registry.nav.no](https://mcp-registry.nav.no): Nav-godkjente MCP-servere

## Innhold i repoet

- `agents/`, `instructions/`, `prompts/`, `skills/`: innholdet nav-pilot installerer ([oversikt](docs/README.skills.md))
- [`cli/nav-pilot`](cli/nav-pilot): CLI-et for utviklere
- [`cli/mcp-compliance`](cli/mcp-compliance): sjekker MCP-servere bak fleet-gateway
- [`apps/my-copilot`](apps/my-copilot): nettstedet ki-utvikling.nav.no (Next.js)
- [`apps/copilot-api`](apps/copilot-api): backend for my-copilot
- [`apps/copilot-cli`](apps/copilot-cli): innlogging og gateway for nav-pilot
- [`apps/copilot-survey`](apps/copilot-survey): brukerundersøkelser
- [`apps/copilot-metrics`](apps/copilot-metrics): nattlig jobb for bruksmetrikker
- [`apps/copilot-adoption`](apps/copilot-adoption): skanner navikt-repoer etter KI-oppsett
- [`apps/fleet-gateway`](apps/fleet-gateway): OAuth-server for Navs interne MCP-servere
- [`apps/mcp-registry`](apps/mcp-registry): MCP-registeret
- [`apps/mcp-onboarding`](apps/mcp-onboarding), [`apps/mcp-dolly`](apps/mcp-dolly), [`apps/mcp-kodeverk`](apps/mcp-kodeverk), [`apps/mcp-team-catalog`](apps/mcp-team-catalog): MCP-servere

Arkitekturen er beskrevet i [ARCHITECTURE.md](ARCHITECTURE.md).

## Bidra

Du trenger [mise](https://mise.jdx.dev). Fra rotmappa:

```bash
mise install
mise check
mise test
```

[AGENTS.md](AGENTS.md) har konvensjonene, og [SECURITY.md](SECURITY.md) har sikkerhetsmodellen. Endrer du `agents/`, `instructions/`, `prompts/` eller `skills/`, kjør `mise run generate` først.

## Lisens

[MIT](LICENSE)
