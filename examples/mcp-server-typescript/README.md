# MCP-server i TypeScript (referanse)

Liten referanse for den Nav-spesifikke delen av en MCP-server, med det offisielle SDK-et
[`@modelcontextprotocol/server`](https://github.com/modelcontextprotocol/typescript-sdk) v2
(MCP 2026-07-28). Semantikken er hentet fra Go-referansen `apps/mcp-kodeverk` (`auth.go`).
Alt som ikke er Nav-spesifikt overlater vi til SDK-et.

## Hva eksempelet viser

| Krav | Hvor |
| --- | --- |
| Texas-introspeksjon: `azp` er fleet-gateways klient-ID (fra `AZURE_APP_PRE_AUTHORIZED_APPS`), `NAVident` kreves, app-tokens avvises (`idtyp=app` eller rollen `access_as_application`). Mangler konfigurasjon, svarer serveren 503. | `src/auth.ts` (`validateToken`) |
| Texas OBO til nedstrøms API. Det innkommende tokenet sendes aldri videre. | `src/auth.ts` (`exchangeToken`) |
| Tak på forespørsel (1 MiB, svarer 413) og på nedstrømssvar og verktøyutdata (32 KiB). | `src/server.ts` |
| Ett skrivebeskyttet verktøy, `list_codes`, med `readOnlyHint: true`. | `src/server.ts` |
| Målinger per verktøy (`mcp_tool_calls_total`, `mcp_tool_duration_seconds_sum`) på `/metrics`. Etiketter: verktøy og utfall, så kardinaliteten er fast. | `src/server.ts` (`Metrics`) |
| Test, inkludert at hver auth-regel avviser. | `src/server.test.ts` |
| Nais-skisse med `accessPolicy` inn fra fleet-gateway. | `.nais/app.yaml` |

`readOnlyHint` er et hint til klienten, ikke en sikkerhetsgrense. Serveren skriver ikke uansett.

## Kjør lokalt

Krever Node 22 eller nyere og pnpm.

```sh
pnpm install
pnpm check   # typesjekk
pnpm test    # vitest, uten nettverk: Texas og nedstrøms er falske
pnpm start   # trenger Texas-miljøvariabler fra Nais
```

CI bygger ikke dette eksempelet. Kjør `pnpm check` og `pnpm test` selv når du endrer det.

## Miljøvariabler

| Variabel | Beskrivelse |
| --- | --- |
| `NAIS_TOKEN_INTROSPECTION_ENDPOINT`, `NAIS_TOKEN_EXCHANGE_ENDPOINT` | Texas (injiseres av Nais) |
| `AZURE_APP_PRE_AUTHORIZED_APPS`, `NAIS_CLUSTER_NAME` | Gir fleet-gateways klient-ID. Mangler den, avvises alle kall. |
| `KODEVERK_URL`, `KODEVERK_SCOPE` | Nedstrøms API og OBO-scope |
| `PORT` | Standard 8080 |

## Kjente begrensninger

- Svar fra Texas leses helt inn i minnet før størrelsen sjekkes. Texas er en sidecar på localhost.
- Ingen rate limit og ingen cache. Se retningslinjene i navikt/copilot#2490.
- Eksempelet viser ikke `outputSchema`/`structuredContent`, paginering eller skriveverktøy.
