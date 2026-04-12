# Diagnostiske trær — detaljerte kommandoer

## Pod-diagnostikk — komplett kommando-referanse

### Steg 1: Status

```bash
# Oversikt over alle pods for appen
kubectl get pods -n {namespace} -l app={app-name} -o wide

# Detaljert pod-info
kubectl describe pod -n {namespace} {pod-name}

# Nais app-status
kubectl get app -n {namespace} {app-name} -o yaml | grep -A 20 status
```

### Steg 2: Logs

```bash
# Siste logs
kubectl logs -n {namespace} -l app={app-name} --tail=100

# Logs fra forrige krasj (CrashLoopBackOff)
kubectl logs -n {namespace} {pod-name} --previous --tail=100

# Følg logs i sanntid
kubectl logs -n {namespace} -l app={app-name} -f --tail=10

# Filtrer på feilmeldinger
kubectl logs -n {namespace} -l app={app-name} --tail=500 | grep -i "error\|exception\|fatal\|panic"
```

### Steg 3: Events

```bash
# Pod-events (viser scheduling, pulling, started, failed)
kubectl get events -n {namespace} --sort-by='.lastTimestamp' | grep {app-name}

# Namespace-events (bredere)
kubectl get events -n {namespace} --sort-by='.lastTimestamp' | tail -20
```

### Steg 4: Ressurser

```bash
# Aktuelt ressursforbruk
kubectl top pod -n {namespace} -l app={app-name}

# Ressurs-requests vs limits
kubectl get pod -n {namespace} {pod-name} -o jsonpath='{.spec.containers[0].resources}'
```

## Auth-diagnostikk — komplett kommando-referanse

### Dekode JWT-token

```bash
# Dekode payload (uten verifikasjon)
echo "{token}" | cut -d'.' -f2 | base64 -d 2>/dev/null | jq .

# Viktige felt å sjekke:
# - iss (issuer): hvem utstedte token
# - aud (audience): hvem token er ment for
# - exp (expiry): når token utløper
# - sub (subject): hvem token representerer
# - azp (authorized party): klient som fikk token
```

### Sjekk auth-konfigurasjon i pod

```bash
# Se alle auth-relaterte env-vars
kubectl get pod {pod} -n {namespace} \
  -o jsonpath='{range .spec.containers[0].env[*]}{.name}={.value}{"\n"}{end}' \
  | grep -E 'AZURE|TOKEN_X|IDPORTEN|MASKINPORTEN'

# Sjekk at JWKS-endpoint er tilgjengelig
kubectl exec -n {namespace} {pod} -- \
  wget -qO- --timeout=5 "$AZURE_OPENID_CONFIG_JWKS_URI" 2>&1 | head -1
```

### Sjekk accessPolicy

```bash
# Se gjeldende accessPolicy
kubectl get app -n {namespace} {app-name} -o yaml | grep -A 30 accessPolicy

# Se network policies
kubectl get networkpolicy -n {namespace} -l app={app-name}
```

### Vanlige auth-feilmønstre

| Feilmelding | Årsak | Løsning |
|------------|-------|---------|
| `Token validation failed: wrong issuer` | Token fra feil IdP | Sjekk om kaller bruker riktig auth-mekanisme |
| `Token validation failed: wrong audience` | Token ment for annen app | Sjekk target audience i token exchange |
| `Token validation failed: expired` | Token utløpt | Sjekk token-refresh, klokkesynk |
| `Connection refused: JWKS endpoint` | Kan ikke nå issuer | Sjekk outbound accessPolicy til login.microsoftonline.com |
| `No bearer token found` | Manglende Authorization-header | Sjekk at frontend/sidecar sender token |

## Kafka-diagnostikk — komplett kommando-referanse

### Sjekk konsument

```bash
# Sjekk logs for Kafka-relaterte meldinger
kubectl logs -n {namespace} -l app={app-name} --tail=200 \
  | grep -i "kafka\|consumer\|producer\|topic\|offset\|partition"

# Sjekk Prometheus-metrikker (via port-forward)
kubectl port-forward -n {namespace} svc/{app-name} 8080:8080
curl -s localhost:8080/metrics | grep kafka
```

### Rapids & Rivers feilsøking

```bash
# Sjekk at River-validering matcher
# Vanlige feil:
# - demandValue("@event_name", "feil_navn") → meldinger ignoreres stille
# - requireKey("felt_som_mangler") → kaster exception
# - Manglende interestedIn() → felt er null

# Sjekk rejected messages (hvis app logger dem)
kubectl logs -n {namespace} -l app={app-name} --tail=500 \
  | grep -i "rejected\|invalid\|validation"
```

## Database-diagnostikk — komplett kommando-referanse

### Sjekk tilkobling

```bash
# Se database env-vars
kubectl get pod {pod} -n {namespace} \
  -o jsonpath='{range .spec.containers[0].env[*]}{.name}={.value}{"\n"}{end}' \
  | grep DB_

# Sjekk HikariCP-status i logs
kubectl logs -n {namespace} -l app={app-name} --tail=200 \
  | grep -i "hikari\|connection pool\|datasource"
```

### Vanlige database-feilmønstre

| Feilmelding | Årsak | Løsning |
|------------|-------|---------|
| `Connection is not available, request timed out` | Pool exhaustion | Reduser `maximumPoolSize`, sjekk connection leaks |
| `FATAL: too many connections for role` | Alle connections brukt | `replicas × maxPool` > `max_connections` |
| `FATAL: password authentication failed` | Feil credentials | Sjekk at Nais har generert riktige secrets |
| `Flyway migration failed` | SQL-feil i migrasjon | Sjekk migrasjonsfil, fiks SQL, eventuelt repair |
| `relation "table" does not exist` | Flyway ikke kjørt | Sjekk at Flyway er konfigurert i app-startup |
