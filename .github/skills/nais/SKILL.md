---
name: nais
description: Nais-deployment, GCP-ressurser, pod-lifecycle og feilsøking på plattformen
license: MIT
compatibility: Application deployed on Nais (Kubernetes on GCP)
metadata:
  domain: platform
  tags: nais kubernetes gcp deployment infrastructure troubleshooting
---

# Nais Platform Skill

Patterns and procedures for deploying, configuring, and troubleshooting applications on Nais (Kubernetes on GCP).

## When to Use

- Creating or modifying Nais manifests
- Adding GCP resources (PostgreSQL, Kafka, buckets)
- Configuring access policies and ingress
- Troubleshooting pod startup failures or crashes
- Understanding pod lifecycle and graceful shutdown

## Commands

```bash
# Check pod status
kubectl get pods -n <namespace> -l app=<app-name>

# View pod logs
kubectl logs -n <namespace> -l app=<app-name> --tail=100

# Describe pod (events, errors)
kubectl describe pod -n <namespace> <pod-name>

# View Nais app status
kubectl get app -n <namespace> <app-name> -o yaml

# Restart deployment (rolling)
kubectl rollout restart deployment/<app-name> -n <namespace>

# Port-forward for local debugging
kubectl port-forward -n <namespace> svc/<app-name> 8080:80
```

## Nais Manifest Structure

Every Nais application requires:

```yaml
apiVersion: nais.io/v1alpha1
kind: Application
metadata:
  name: app-name
  namespace: team-namespace
  labels:
    team: team-namespace
spec:
  image: {{ image }} # Replaced by CI/CD
  port: 8080

  # Observability (required)
  prometheus:
    enabled: true
    path: /metrics

  # Health checks (required)
  liveness:
    path: /isalive
    initialDelay: 5
  readiness:
    path: /isready
    initialDelay: 5

  # Resources (required)
  resources:
    requests:
      cpu: 50m
      memory: 256Mi
    limits:
      memory: 512Mi
```

## Pod Lifecycle and Graceful Shutdown

When Kubernetes terminates a pod on NAIS:

1. **K8s notifies load balancer and pod simultaneously** — LB starts draining connections
2. **NAIS preStop hook runs `sleep 5`** — gives LB time to stop routing new traffic
3. **App receives SIGTERM** — no new requests arrive from LB
4. **App drains in-flight requests and exits**
5. **After `terminationGracePeriodSeconds` (default 30s): SIGKILL**

**Key insight:** Readiness probes are NOT involved in shutdown. Your app just needs to handle SIGTERM, finish in-flight requests, and exit cleanly.

Common anti-patterns:
- ❌ Setting readiness to `false` on SIGTERM — unnecessary
- ❌ `terminationGracePeriodSeconds` too low — must be > 5s (preStop) + drain time
- ❌ Adding a `preStop` hook with extra sleep — NAIS already injects `sleep 5`

## Common Tasks

### Adding PostgreSQL Database

```yaml
gcp:
  sqlInstances:
    - type: POSTGRES_15
      databases:
        - name: myapp-db
          envVarPrefix: DB
```

Application receives: `DB_HOST`, `DB_PORT`, `DB_DATABASE`, `DB_USERNAME`, `DB_PASSWORD`

### Configuring Kafka

```yaml
kafka:
  pool: nav-dev # or nav-prod
```

### Azure AD Authentication

```yaml
azure:
  application:
    enabled: true
    tenant: nav.no
```

### TokenX for Service-to-Service

```yaml
tokenx:
  enabled: true

accessPolicy:
  inbound:
    rules:
      - application: calling-app
        namespace: calling-namespace
  outbound:
    rules:
      - application: downstream-app
        namespace: downstream-namespace
```

### Ingress Configuration

```yaml
ingresses:
  - https://myapp.intern.dev.nav.no # Internal dev
  - https://myapp.dev.nav.no        # External dev
```

### Scaling

```yaml
replicas:
  min: 2
  max: 4
  cpuThresholdPercentage: 80
```

## Resource Recommendations

| Size | CPU request | Memory request | Memory limit |
|------|-------------|----------------|--------------|
| Small | 50m | 256Mi | 512Mi |
| Medium | 100m | 512Mi | 1Gi |
| Large | 200m | 1Gi | 2Gi |

**Never set CPU limits** — causes throttling. Use requests only.

## Troubleshooting

### Pod Not Starting

1. Check logs: `kubectl logs -n namespace pod-name`
2. Check events: `kubectl describe pod -n namespace pod-name`
3. Verify health endpoints return 200 OK
4. Check resource limits (memory/CPU)

### Database Connection Issues

1. Verify database exists in GCP Console
2. Check environment variables are injected
3. Verify Cloud SQL Proxy is running
4. Check network policies allow connection

### Kafka Connection Issues

1. Verify `kafka.pool` is correct (nav-dev/nav-prod)
2. Check Kafka credentials are injected
3. Verify SSL configuration
4. Check topic exists and permissions are correct

## Deployment Workflow

1. Create `.nais/app.yaml` manifest
2. Implement health endpoints (`/isalive`, `/isready`, `/metrics`)
3. Test locally with Docker
4. Deploy to dev environment
5. Verify metrics in Grafana
6. Check logs in Loki
7. Create prod manifest (`.nais/app-prod.yaml`)
8. Deploy to production

## Gotchas

- `accessPolicy` defaults to deny-all — you must explicitly allow traffic
- Don't set CPU limits — only requests (limits cause throttling)
- Memory limits are mandatory — missing limits cause OOM cluster issues
- NAIS injects `preStop: sleep 5` — don't add your own
- `{{ image }}` in manifest is replaced by CI/CD — don't hardcode images
- Environment-specific manifests (`app-dev.yaml`, `app-prod.yaml`) are the norm

## Boundaries

### ✅ Always

- Include liveness, readiness, and metrics endpoints
- Set memory limits
- Define explicit `accessPolicy` for network traffic
- Use environment-specific manifests
- Run `kubectl get app <name> -o yaml` to verify deployment

### ⚠️ Ask First

- Changing production resource limits or replicas
- Adding new GCP resources (cost implications)
- Modifying network policies (`accessPolicy`)
- Changing Kafka topic configurations
- Adding new ingress domains

### 🚫 Never

- Store secrets in Git
- Deploy directly without CI/CD pipeline
- Skip health endpoints
- Set CPU limits
- Remove memory limits
