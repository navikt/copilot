# 2. TokenX-veksling via Texas med cache

Dato: 2026-03-04

## Kontekst

`vedtak-proxy` kaller `vedtak-api` og `arkiv-api` på vegne av innlogget bruker.
Hvert kall trenger et TokenX-token for API-ets audience. Texas-sidecaren veksler
brukerens token mot et slikt token. Uten cache går det ett kall til Texas per
forespørsel.

## Beslutning

- Vekslede tokener caches i minnet per pod, felles for alle klientene.
- Et vekslet token gjelder én bruker og én audience. Det skal aldri brukes for en
  annen bruker eller mot et annet API.
- Tokener fornyes når tiden som er igjen før de utløper, er kortere enn
  `TokenConfig.expiryLeeway`. Det dekker klokkeforskjell og tiden et kall er underveis.
- Texas har en grense for antall vekslinger per sekund. Samtidige forespørsler fra
  samme bruker mot samme API skal derfor dele én veksling.
- Tiden hentes fra en injisert `Clock`, så testene kan styre den.

## Konsekvenser

Cachen vokser med antall aktive brukere. Det er akseptabelt med dagens volum.
