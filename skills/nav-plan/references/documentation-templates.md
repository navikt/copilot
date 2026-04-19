# Dokumentasjon og leveransemaler

Maler for endringsfokusert dokumentasjon. Bruk disse når du leverer en plan — ikke bare kode, men alt som trengs for trygg utrulling og drift.

## 1. Endringsdokument

Generer dette for enhver ikke-triviell endring:

```markdown
# Endring: {tittel}

**Dato:** {dato}
**Team:** {team}
**Risiko:** Lav / Middels / Høy

## Hva endres?

{Kort beskrivelse av endringen — maks 3 setninger}

## Hvorfor?

{Motivasjon — forretningsbehov, teknisk gjeld, regulatorisk krav}

## Berørte komponenter

| Komponent | Type endring | Risiko |
|-----------|-------------|--------|
| {tjeneste-a} | API-endring (breaking) | Høy |
| {tjeneste-b} | Ny konsument | Lav |
| {database} | Skjemaendring | Middels |

## Berørte team

| Team | Hva påvirkes | Handling |
|------|-------------|---------|
| {team-x} | Konsumerer API v1 | Må migrere til v2 innen {dato} |

## Utrullingsstrategi

1. Deploy til dev — verifiser med {smoke test}
2. Feature toggle ON for testbrukere
3. Gradvis utrulling: 10% → 50% → 100%
4. Monitor i 48 timer
5. Fjern feature toggle

## Rollback-plan

- **Trigger:** {hva utløser rollback — feilrate > 1%, latency > 500ms}
- **Aksjon:** {slå av feature toggle / revert deploy / rollback migrering}
- **Datatap?** {Ja/Nei — hvis ja, beskriv mitigering}

## Post-deploy-verifisering

- [ ] Smoke test passerer i prod
- [ ] Forretningsmetrikker er stabile ({metrikknavn})
- [ ] Feilrate er under {terskel}
- [ ] Ingen nye feil i Sentry/logg
- [ ] Berørte team bekrefter at integrasjoner fungerer
```

## 2. Runbook-oppdatering

Legg til eller oppdater runbook for endringer som påvirker drift:

```markdown
# Runbook: {tjenestenavn}

## Oversikt

**Eier:** {team}
**Kritikalitet:** Lav / Middels / Høy / Kritisk
**Kontakt:** #{slack-kanal}

## Helsesjekk

| Sjekk | URL/kommando | Forventet resultat |
|-------|-------------|-------------------|
| Liveness | GET /isalive | 200 OK |
| Readiness | GET /isready | 200 OK |
| Business health | GET /metrics → {metrikk} | > 0 siste 5 min |

## Vanlige feil og løsninger

| Symptom | Årsak | Løsning |
|---------|-------|---------|
| 401 mot downstream | Token utløpt / feil scope | Sjekk at accessPolicy.outbound er riktig |
| Connection refused | Database nede / pool exhausted | `kubectl logs`, sjekk HikariCP-metrikker |
| Kafka consumer lag | Treg prosessering | Sjekk CPU-bruk, vurder flere replicas |

## Avhengigheter

| Tjeneste | Retning | Konsekvens hvis nede |
|----------|---------|---------------------|
| {pdl-api} | Outbound | Kan ikke slå opp person — returnerer 503 |
| {kafka-topic} | Inbound | Meldinger akkumuleres, prosesseres ved restart |

## Eskalering

1. Sjekk dashboards: {Grafana-URL}
2. Sjekk logger: `kubectl logs -n {ns} -l app={app} --tail=100`
3. Kontakt team: #{slack-kanal}
4. Ved alvorlig hendelse: Følg Navs beredskapsrutine
```

## 3. API-endringsdokument

For API-endringer som påvirker konsumenter:

```markdown
# API-endring: {endpunkt}

**Versjon:** v{n} → v{n+1}
**Breaking:** Ja / Nei
**Migrasjonsfrist:** {dato}

## Endringer

### Nye felt
| Felt | Type | Beskrivelse |
|------|------|------------|
| {felt} | string | {beskrivelse} |

### Fjernede felt
| Felt | Erstatning | Migrasjonsveiledning |
|------|-----------|---------------------|
| {gammelt_felt} | {nytt_felt} | Bruk {nytt_felt} i stedet |

### Endret oppførsel
| Endpunkt | Før | Etter |
|----------|-----|-------|
| GET /api/v2/vedtak | Returnerer `status: "AKTIV"` | Returnerer `status: "active"` |

## Eksempler

### Før (v1)
```json
{ "status": "AKTIV", "belop": 1000 }
```

### Etter (v2)
```json
{ "status": "active", "beloep": 1000, "currency": "NOK" }
```

## Migrasjonsveiledning

1. Oppdater klient til å akseptere både v1 og v2
2. Bytt til v2-endepunkt
3. Fjern v1-kompatibilitet etter {dato}
```

## 4. Observerbarhet for endring

Definer dette for enhver endring som påvirker produksjonsadferd:

```markdown
## Observerbarhet

### Suksesskriterier (hvordan vet vi at endringen fungerer?)

| Signal | Metrikk/logg | Forventet verdi | Alarm-terskel |
|--------|-------------|-----------------|---------------|
| Funksjonelt | {business_metric}_total | > 0 per minutt | 0 i 5 min |
| Teknisk | http_request_duration_seconds | p99 < 500ms | p99 > 1s |
| Feil | http_errors_total{status="5xx"} | < 0.1% | > 1% |

### Dashboards

- Tjeneste-dashboard: {Grafana-URL}
- Forretning-dashboard: {Grafana-URL}

### Migrerings-spesifikk observerbarhet (ved gradvis utrulling)

| Signal | Metrikk | Formål |
|--------|---------|--------|
| Gammel path | {metric}_old_path_total | Verifiserer at trafikk flyttes |
| Ny path | {metric}_new_path_total | Verifiserer at ny path fungerer |
| Avvik | {metric}_reconciliation_diff | Oppdager datadivergenser |
| Toggle-fordeling | feature_toggle_evaluation{toggle="X"} | Verifiserer utrullingstakt |
```

## 5. Sjekkliste: Klar for produksjon

Bruk denne sjekklisten for å verifisere at endringen er klar for utrulling:

### Alle endringer
- [ ] Tester passerer (unit + integrasjon)
- [ ] Kodegjennomgang er godkjent
- [ ] Nais-manifest er oppdatert (accessPolicy, resurser, auth)
- [ ] Observerbarhet er på plass (metrikker, logger, dashboard)
- [ ] Rollback-plan er definert

### Endringer med ekstern påvirkning
- [ ] Berørte team er informert
- [ ] API-dokumentasjon er oppdatert
- [ ] Bakoverkompatibilitet er verifisert
- [ ] Migrasjonsveiledning er publisert

### Databaseendringer
- [ ] Migrasjon er testet med realistisk datamengde
- [ ] Migrasjon er bakoverkompatibel (gammel kode + nytt skjema)
- [ ] Indekser er lagt til for nye spørringer
- [ ] Rollback-migrasjon er forberedt (hvis nødvendig)

### Sikkerhet
- [ ] PII logges ikke
- [ ] Auth er riktig for alle endepunkter
- [ ] Input-validering er på plass
- [ ] Tilgangskontroll er verifisert
