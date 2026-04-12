# Nais-manifest maler per arketype

## Backend API (Kotlin/Ktor)

```yaml
apiVersion: nais.io/v1alpha1
kind: Application
metadata:
  name: {app-name}
  namespace: {team}
  labels:
    team: {team}
spec:
  image: "{{ image }}"
  port: 8080
  liveness:
    path: /isalive
    initialDelay: 5
  readiness:
    path: /isready
    initialDelay: 5
  prometheus:
    enabled: true
    path: /metrics
  resources:
    requests:
      cpu: 15m
      memory: 256Mi
    limits:
      memory: 512Mi
  replicas:
    min: 2
    max: 4
  # Auth — velg basert på beslutningstre
  azure:
    application:
      enabled: true
  tokenx:
    enabled: true
  # Database
  gcp:
    sqlInstances:
      - type: POSTGRES_15
        databases:
          - name: {app-name}-db
  # Tilgangsstyring — ALLTID eksplisitt
  accessPolicy:
    inbound:
      rules:
        - application: {frontend-app}
    outbound:
      rules: []
```

## Hendelsekonsument (Kafka)

```yaml
apiVersion: nais.io/v1alpha1
kind: Application
metadata:
  name: {app-name}
  namespace: {team}
  labels:
    team: {team}
spec:
  image: "{{ image }}"
  port: 8080
  liveness:
    path: /isalive
    initialDelay: 5
  readiness:
    path: /isready
    initialDelay: 5
  prometheus:
    enabled: true
    path: /metrics
  resources:
    requests:
      cpu: 15m
      memory: 256Mi
    limits:
      memory: 512Mi
  replicas:
    min: 2
    max: 4
  kafka:
    pool: nav-dev  # nav-prod i prod
  gcp:
    sqlInstances:
      - type: POSTGRES_15
        databases:
          - name: {app-name}-db
  accessPolicy:
    inbound:
      rules: []  # Konsumenter har sjelden inbound
    outbound:
      rules: []  # Fyll inn hvis tjenesten kaller andre
```

**Topic-definisjon (egen YAML):**

```yaml
apiVersion: kafka.nais.io/v1
kind: Topic
metadata:
  name: {team}.{domene}.v1
  namespace: {team}
  labels:
    team: {team}
spec:
  pool: nav-dev
  config:
    cleanupPolicy: delete
    minimumInSyncReplicas: 1
    partitions: 1        # Dev: 1, Prod: 6+
    replication: 1        # Dev: 1, Prod: 3
    retentionBytes: -1
    retentionHours: 336   # 14 dager
  acl:
    - team: {team}
      application: {app-name}
      access: readwrite
```

## Frontend (Next.js + ID-porten)

```yaml
apiVersion: nais.io/v1alpha1
kind: Application
metadata:
  name: {app-name}
  namespace: {team}
  labels:
    team: {team}
spec:
  image: "{{ image }}"
  port: 3000
  liveness:
    path: /api/isalive
    initialDelay: 5
  readiness:
    path: /api/isready
    initialDelay: 5
  prometheus:
    enabled: true
    path: /metrics
  resources:
    requests:
      cpu: 15m
      memory: 256Mi
    limits:
      memory: 512Mi
  replicas:
    min: 2
    max: 4
  # Innbygger-auth
  idporten:
    enabled: true
    sidecar:
      enabled: true
      autoLogin: true
      autoLoginIgnorePaths:
        - /api/isalive
        - /api/isready
  ingresses:
    - https://{app-name}.intern.dev.nav.no
  accessPolicy:
    outbound:
      rules:
        - application: {backend-api}
```

## Frontend (Next.js + Azure AD)

```yaml
apiVersion: nais.io/v1alpha1
kind: Application
metadata:
  name: {app-name}
  namespace: {team}
  labels:
    team: {team}
spec:
  image: "{{ image }}"
  port: 3000
  liveness:
    path: /api/isalive
    initialDelay: 5
  readiness:
    path: /api/isready
    initialDelay: 5
  prometheus:
    enabled: true
    path: /metrics
  resources:
    requests:
      cpu: 15m
      memory: 256Mi
    limits:
      memory: 512Mi
  replicas:
    min: 2
    max: 4
  # Saksbehandler-auth
  azure:
    application:
      enabled: true
      tenant: nav.no
    sidecar:
      enabled: true
      autoLogin: true
      autoLoginIgnorePaths:
        - /api/isalive
        - /api/isready
  ingresses:
    - https://{app-name}.intern.dev.nav.no
  accessPolicy:
    outbound:
      rules:
        - application: {backend-api}
```

## Batchjobb (Naisjob)

```yaml
apiVersion: nais.io/v1alpha1
kind: Naisjob
metadata:
  name: {app-name}
  namespace: {team}
  labels:
    team: {team}
spec:
  image: "{{ image }}"
  schedule: "0 6 * * *"  # Hver dag kl 06:00
  activeDeadlineSeconds: 3600
  resources:
    requests:
      cpu: 50m
      memory: 256Mi
    limits:
      memory: 512Mi
  azure:
    application:
      enabled: true
  gcp:
    sqlInstances:
      - type: POSTGRES_15
        databases:
          - name: {app-name}-db
  accessPolicy:
    outbound:
      rules:
        - application: {target-api}
          namespace: {target-namespace}
```
