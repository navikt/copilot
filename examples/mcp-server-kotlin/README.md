# MCP-server i Kotlin (referanse)

Liten MCP-server med Ktor og den offisielle [MCP Kotlin SDK](https://github.com/modelcontextprotocol/kotlin-sdk) (0.16.0). Den viser bare det Nav-spesifikke. Semantikken er kopiert fra Go-referansen [`apps/mcp-kodeverk`](../../apps/mcp-kodeverk) (`auth.go` og testene). Retningslinjene: [#2490](https://github.com/navikt/copilot/issues/2490).

## Hva den viser

| Krav | Hvor |
|---|---|
| Innkommende token valideres med Texas-introspeksjon. `azp` må være fleet-gateways klient-ID (fra `AZURE_APP_PRE_AUTHORIZED_APPS`), `NAVident` må finnes, app-tokens (`idtyp=app` eller rollen `access_as_application`) avvises. Mangler konfigurasjon, svarer serveren 503. | `Auth.kt` |
| OBO via Texas til nedstrøms API. Innkommende token videresendes aldri. | `Texas.exchange`, `Server.kt` |
| Tak på forespørsel (64 KiB, 413), nedstrøms svar (1 MiB, feil) og resultat til modellen (32 KiB, avkortet med hint) | `Server.kt` |
| Ett lesende verktøy, `kodeverk_list`, med alle fire annotasjoner (`readOnlyHint: true`). Annotasjoner er hint. Serveren håndhever lesing ved å bare kalle GET. | `Server.kt` |
| Metrikker per verktøy (`mcp_tool_calls_total`, `mcp_tool_errors_total`, `mcp_tool_duration_seconds_sum`) på `/metrics`. Label er bare registrerte verktøynavn. | `Metrics.kt` |
| Test | `src/test/.../ServerTest.kt` |
| Nais-skisse med `accessPolicy` inn fra fleet-gateway | `.nais/app.yaml` |

Serveren er tilstandsløs (`mcpStatelessStreamableHttp`): ett `Server`-objekt per forespørsel, så brukerens token ligger i closure og ikke i en sesjon.

## Kjøre lokalt

Krever JDK 21 (bruk ikke nyere JDK enn Gradle 8.14 støtter). Wrapperen henter Gradle 8.14.7.

```sh
cd examples/mcp-server-kotlin
./gradlew test
```

CI bygger ikke dette prosjektet. Kjør testene selv før du endrer noe.

## Kjente forenklinger

- Forespørsler uten `Content-Length` (chunked) avvises med 411, slik at størrelsestaket ikke kan omgås. Gatewayen sender `Content-Length`.
- Metrikkene er en enkel tekstutskrift uten bibliotek. Bytt til Micrometer hvis du trenger histogram.
- Ingen rate limit, `Mcp-*`-hodesjekk (`-32020`) eller `outputSchema`. Se Go-referansen.
- Tokenvalidering kaller Texas på hver forespørsel, uten cache.
