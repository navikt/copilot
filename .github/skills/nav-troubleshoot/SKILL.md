---
name: nav-troubleshoot
description: Strukturerte diagnostiske trær for vanlige Nav-plattformproblemer — pod-krasj, auth-feil, Kafka-lag og databaseproblemer
---

# Nav Troubleshoot — Platform Diagnostics

Strukturerte diagnostiske trær for vanlige problemer på Nais-plattformen. Erstatter «spør i Slack-kanalen» med guidet feilsøking.

## Workflow

1. **Identifiser symptomet** — hva feiler?
2. **Følg diagnostisk tre** — steg-for-steg med kommandoer
3. **Finn rotårsak** — hva outputen betyr
4. **Fiks** — konkret løsning

## Symptom-oversikt

| Symptom | Start her |
|---------|-----------|
| Pod starter ikke / krasjer | [Pod-problemer](#pod-problemer) |
| 401 Unauthorized / 403 Forbidden | [Auth-feil](#auth-feil) |
| Kafka consumer lag / meldinger prosesseres ikke | [Kafka-problemer](#kafka-problemer) |
| Database-tilkoblingsfeil | [Database-problemer](#database-problemer) |
| Treg responstid | [Ytelse](#ytelsesproblemer) |
| Deploy feiler | [Deploy-problemer](#deploy-problemer) |

Se [diagnostic-trees.md](./references/diagnostic-trees.md) for detaljerte diagnostiske trær med kommandoer.

## Pod-problemer

### CrashLoopBackOff

```bash
# 1. Sjekk pod-status
kubectl get pods -n {namespace} -l app={app-name}

# 2. Sjekk logs fra forrige krasj
kubectl logs -n {namespace} -l app={app-name} --previous --tail=50

# 3. Sjekk events
kubectl describe pod -n {namespace} {pod-name} | grep -A 20 Events
```

**Vanlige årsaker:**

| Log-output | Årsak | Løsning |
|-----------|-------|---------|
| `OOMKilled` | For lite minne | Øk `resources.limits.memory` |
| `java.lang.OutOfMemoryError` | Java heap for liten | Legg til `-Xmx` eller øk memory limit |
| `Connection refused: localhost:5432` | Database ikke klar | Sjekk Cloud SQL-instans, Flyway-migrasjon |
| `AZURE_APP_CLIENT_ID not set` | Manglende env-var | Sjekk at `azure.application.enabled: true` i Nais |
| `No such file or directory` | Feil Dockerfile COPY | Verifiser at build-artefakt kopieres riktig |
| Port-mismatch | App lytter på feil port | Sjekk at `spec.port` matcher appens port |

### ImagePullBackOff

```bash
# Sjekk image-navn
kubectl describe pod -n {namespace} {pod-name} | grep Image

# Vanlige årsaker:
# - Feil image-tag (bygget mislyktes)
# - GAR-autentisering feilet
# - Image finnes ikke
```

### Pending (pod starter aldri)

```bash
# Sjekk om det er ressurs-begrensninger
kubectl describe pod -n {namespace} {pod-name} | grep -A 5 Conditions

# Vanlige årsaker:
# - Ikke nok ressurser i klusteret
# - PersistentVolumeClaim ikke bundet
# - Node-selektor matcher ikke
```

## Auth-feil

### 401 Unauthorized

```bash
# 1. Sjekk om token er gyldig
echo "{token}" | cut -d'.' -f2 | base64 -d 2>/dev/null | jq .

# 2. Sjekk issuer
# Token bør ha: "iss": "https://login.microsoftonline.com/{tenant}/v2.0"

# 3. Sjekk audience
# Token bør ha: "aud": "{din-app-client-id}"

# 4. Sjekk om JWKS er tilgjengelig
kubectl exec -n {namespace} {pod} -- wget -qO- $AZURE_OPENID_CONFIG_JWKS_URI | head -1
```

**Diagnostisk tre:**

```
401 Unauthorized
├── Har forespørselen Authorization-header?
│   ├── Nei → Kaller mangler token, sjekk frontend/sidecar
│   └── Ja → Gå videre
│
├── Er token fra riktig issuer?
│   ├── Azure AD men forventet TokenX → Feil auth-flow
│   ├── ID-porten men forventet Azure AD → Feil sidecar-config
│   └── Riktig issuer → Gå videre
│
├── Er audience riktig?
│   ├── Feil audience → Kaller sender token til feil mottaker
│   └── Riktig → Gå videre
│
├── Er token utløpt?
│   ├── exp < nåtid → Token expired, sjekk token-refresh
│   └── Gyldig → Gå videre
│
└── Er JWKS tilgjengelig fra podden?
    ├── Nei → Nettverksproblem, sjekk accessPolicy outbound
    └── Ja → Sjekk token-validation-konfigurasjon
```

### 403 Forbidden

```
403 Forbidden
├── Er accessPolicy.inbound konfigurert?
│   ├── Nei → Legg til kaller i inbound rules
│   └── Ja → Gå videre
│
├── Er kaller registrert i inbound?
│   ├── Nei → Legg til: application: {kaller}, namespace: {ns}
│   └── Ja → Gå videre
│
└── Er det applikasjonsnivå-autorisasjon?
    ├── Ja → Sjekk roller/grupper i token
    └── Nei → Sjekk Nais app-status: kubectl get app {name} -o yaml
```

## Kafka-problemer

### Consumer lag

```bash
# 1. Sjekk consumer group status (krever Kafka CLI-tilgang)
# Alternativt: sjekk Prometheus-metrikker
# kafka_consumer_group_lag > 0

# 2. Sjekk pod-logs for feil
kubectl logs -n {namespace} -l app={app-name} --tail=100 | grep -i "error\|exception\|failed"

# 3. Sjekk om konsumenten prosesserer
kubectl logs -n {namespace} -l app={app-name} --tail=20 | grep -i "processed\|consumed"
```

**Diagnostisk tre:**

```
Kafka consumer lag
├── Øker lag kontinuerlig?
│   ├── Ja → Konsumenten kan ikke holde tritt
│   │   ├── Sjekk prosesseringstid per melding
│   │   ├── Vurder å øke partitions + replicas
│   │   └── Sjekk om det er en poison pill (melding som feiler)
│   └── Nei, sporadisk → Normal variasjon, sannsynligvis OK
│
├── Er konsumenten oppe?
│   ├── Nei → Sjekk pod-status (CrashLoopBackOff?)
│   └── Ja → Gå videre
│
├── Logger konsumenten feil?
│   ├── Deserialisering-feil → Schema-mismatch, sjekk producer
│   ├── DB-feil → Database-problem, se database-seksjon
│   └── Ingen feil → Sjekk om den faktisk leser fra riktig topic
│
└── Rapids & Rivers?
    ├── Sjekk at validate()-regler matcher meldingsformat
    ├── Sjekk at @event_name er riktig
    └── Prøv å legge til interestedIn() for feilsøking
```

## Database-problemer

### Connection refused / timeout

```bash
# 1. Sjekk at Cloud SQL-instans kjører
# Nais Console → App → Database → Status

# 2. Sjekk env-vars
kubectl get pod {pod} -n {namespace} -o jsonpath='{range .spec.containers[0].env[*]}{.name}={.value}{"\n"}{end}' | grep DB_

# 3. Sjekk pool-status (i app-logs)
kubectl logs -n {namespace} -l app={app-name} | grep -i "hikari\|connection\|pool"
```

**Diagnostisk tre:**

```
Database-tilkoblingsfeil
├── Er Cloud SQL-instans oppe?
│   ├── Nei → Sjekk GCP Console / Nais Console
│   └── Ja → Gå videre
│
├── Er env-vars satt?
│   ├── DB_HOST/DB_PORT/DB_DATABASE mangler → Sjekk Nais-manifest gcp.sqlInstances
│   └── Satt → Gå videre
│
├── Feilet Flyway-migrasjon?
│   ├── Ja → Sjekk SQL-feil i startup-log
│   └── Nei → Gå videre
│
├── Pool exhaustion?
│   ├── «Connection is not available» → Reduser maxPoolSize, sjekk lekkasjer
│   ├── Mange «active» connections → Trege queries, sjekk EXPLAIN
│   └── Nei → Gå videre
│
└── max_connections nådd?
    ├── Ja → Reduser pool per replica: replicas × maxPool < max_connections
    └── Nei → Sjekk nettverks-tilgang (Cloud SQL proxy)
```

## Ytelsesproblemer

```
Treg responstid
├── Hvor er flaskehalsen?
│   ├── Sjekk Prometheus: http_request_duration_seconds
│   ├── Sjekk Tempo: distribuert trace
│   └── Sjekk Grafana Loki: loggtider
│
├── Database-queries?
│   ├── Manglende indekser → EXPLAIN ANALYZE
│   ├── N+1 queries → Bruk JOIN eller batch
│   └── Store result sets → Paginering
│
├── Ekstern tjeneste treg?
│   ├── Sjekk response time per dependency
│   ├── Vurder circuit breaker
│   └── Vurder caching
│
└── Ressursbegrensning?
    ├── CPU throttling → ALDRI sett CPU limits, sjekk requests
    └── Memory pressure → Øk memory limit
```

## Deploy-problemer

```
Deploy feiler
├── GitHub Actions-feil?
│   ├── Build-feil → Sjekk kompileringsfeil i actions-log
│   ├── Docker build-feil → Sjekk Dockerfile
│   └── Push-feil → Sjekk GAR-tilgang
│
├── Nais deploy-feil?
│   ├── «invalid manifest» → Valider YAML-syntaks
│   ├── «unauthorized» → Sjekk deploy-key/workload-identity
│   └── «resource quota exceeded» → Sjekk team-kvote
│
└── Deploy OK, men app feiler?
    ├── Sjekk pod-status (se Pod-problemer over)
    └── Sjekk rollout: kubectl rollout status deployment/{app}
```

## Boundaries

### ✅ Always

- Start med å identifisere symptomet før du kjører kommandoer
- Følg det diagnostiske treet steg for steg
- Sjekk logs og events før du endrer konfigurasjon
- Foreslå minst invasive fiks først

### ⚠️ Ask First

- Endre produksjons-konfigurasjon
- Restarte pods i produksjon
- Endre database-konfigurasjon

### 🚫 Never

- Endre secrets direkte i klusteret
- Kjør `kubectl delete pod` i prod uten å forstå årsaken
- Ignorer OOMKilled — det vil skje igjen
