# Drift

Appen kjører på NAIS med Azure AD-innlogging via Wonderwall. Wonderwall legger
saksbehandlerens token i `Authorization`-headeren.

- NAIS kaller `/api/isalive` og `/api/isready` uten token. De må alltid svare 200,
  ellers starter podene på nytt i en løkke.
- Alle andre ruter under `/api/` krever et gyldig Azure-token. Tokenet valideres
  med `requireSaksbehandler()` i `lib/auth.ts`.
- `AZURE_OPENID_CONFIG_JWKS_URI`, `AZURE_OPENID_CONFIG_ISSUER` og
  `AZURE_APP_CLIENT_ID` settes av NAIS.
