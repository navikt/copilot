# 2. TokenX-veksling via Texas med cache

Dato: 2026-03-04

## Kontekst

`vedtak-proxy` kaller `vedtak-api` på vegne av innlogget bruker. Hver forespørsel
trenger et TokenX-token for audience `dev-gcp:vedtak:vedtak-api`. Texas-sidecaren
veksler brukerens token mot et slikt token. Uten cache går det ett kall til Texas
per forespørsel.

## Beslutning

- Vekslede tokener caches i minnet per pod.
- Et vekslet token hører til brukeren som ga det, og skal aldri brukes for en
  annen brukers forespørsel.
- Tokener fornyes når det er mindre enn 30 sekunder igjen før de utløper. Det dekker
  klokkeforskjell mellom podene og `vedtak-api` og tiden et kall er underveis.
- Tiden hentes fra en injisert `Clock`, så testene kan styre den.

## Konsekvenser

Cachen vokser med antall aktive brukere. Det er akseptabelt med dagens volum.
