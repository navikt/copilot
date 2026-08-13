# Ticket: cplt one-shot token broker for Copilot auth (Keychain scope reduction)

## Bakgrunn

Vi ønsker å redusere behovet for bred Keychain-tilgang for `--agent copilot` i cplt.
I dag er Keychain-tilgang et kjent tradeoff for Copilot-auth.

Forslag: parent-prosess henter token før sandboxing og leverer det én gang til `gh`-wrapper via broker, uten å skrive token til disk.

## Mål

Redusere eksponeringsflate (Keychain + token på disk) uten å knekke Copilot-flyt.

## Viktig avgrensning

Dette er **hardening**, ikke en hard sikkerhetsgrense mot ondsinnet agent med samme UID.
Agenten kan fortsatt be om token via legitim auth-flyt.

## Foreslått design

Auth-prioritet:

1. `GH_TOKEN` i env
2. parent-side pre-ekstraksjon + one-shot broker
3. Keychain fallback (eller fail-closed via policy)

Nye policyvalg (forslag):

- `copilot.keychain_fallback = true|false`
- `copilot.auth_mode = auto|env|broker_only|keychain_only`

## Implementasjonsoppgaver

1. Legg inn broker-komponent i cplt parent-prosess.
2. Koble `gh`-wrapper til one-shot token callback.
3. Zeroize/minimer token-buffer i parent/wrapper etter bruk.
4. Legg til tydelig startup-logg: hvilken auth-path ble valgt.
5. Legg til config-validering for nye auth-policy keys.
6. Dokumenter sikkerhetsmodell og begrensninger i `../SECURITY.md`.

## Akseptansekriterier

- Copilot fungerer fortsatt i normal setup uten manuell re-auth.
- Når broker-path lykkes: ingen disk-cache av token + ingen nødvendig Keychain-read i sandbox for den kjøringen.
- Når broker-path feiler: tydelig fallback/feilmelding iht. policy.
- Tester dekker:
  - auth-path selection
  - fallback-logikk
  - one-shot semantics
  - feil med forklarende output

## Risiko / åpne spørsmål

- Kompatibilitet med eksisterende `gh_guard`/`block_auth_token`-flyt.
- Drift på macOS vs Linux vs Windows (Keychain vs Secret Service vs Windows Credential Manager).
- Hvor mye Copilot-klienten selv fortsatt forventer Keychain-kall senere i sesjonen.

## Foreslått branch

`feature/cplt-token-broker-auth-path`

