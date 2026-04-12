# Navs dataklassifisering

Alle data i Nav klassifiseres etter sensitivitet. Klassifiseringen bestemmer krav til lagring, tilgang og logging.

## Nivåer

| Nivå | Beskrivelse | Eksempler | Krav |
|------|-------------|-----------|------|
| **Åpen** | Offentlig informasjon | Regelverk, satser, generell info | Ingen spesielle krav |
| **Intern** | Ikke-sensitiv intern info | Team-dokumentasjon, teknisk config | Bare Nav-ansatte |
| **Fortrolig** | Personopplysninger | Fnr, navn, adresse, ytelseshistorikk | Tilgangsstyring, audit-logging, GDPR |
| **Strengt fortrolig** | Spesielle kategorier | Helseopplysninger, kode 6/7 (skjermede personer) | Streng tilgangsstyring, kryptering, minimal eksponering |

## PII-kategorier i Nav

| Kategori | Eksempler | Spesielle krav |
|----------|-----------|----------------|
| Identifikator | Fødselsnummer (fnr), D-nummer, aktør-ID | Aldri logg, alltid kryptert i transit |
| Kontaktinfo | Navn, adresse, telefon, e-post | GDPR-samtykke, sletteregler |
| Ytelsesdata | Vedtak, utbetalinger, søknader | Tilgangsstyring per ytelse |
| Helseopplysninger | Diagnoser, legeerklæringer, arbeidsevnevurderinger | Art. 9 GDPR, strengt fortrolig |
| Skjermingsdata | Kode 6 (strengt fortrolig adresse), kode 7 (fortrolig adresse) | Absolutt minimum tilgang, spesialbehandling |

## Konsekvenser for arkitektur

### Fortrolig (de fleste Nav-tjenester)

```yaml
# Nais-manifest
spec:
  accessPolicy:
    inbound:
      rules:
        - application: spesifikk-kaller  # Aldri åpent
    outbound:
      rules:
        - application: pdl-api
          namespace: pdl
```

```kotlin
// Logging — bare korrelasjons-IDer, aldri PII
log.info("Behandler vedtak", kv("vedtakId", vedtak.id), kv("sakId", sak.id))
// ALDRI: log.info("Vedtak for ${bruker.fnr}")
```

### Strengt fortrolig

- Minimalt antall tjenester med tilgang
- Ekstra tilgangskontroll i kode (ikke bare Nais accessPolicy)
- Kryptering at rest og in transit
- Audit-logging av all tilgang
- Vurder om data kan anonymiseres/pseudonymiseres
