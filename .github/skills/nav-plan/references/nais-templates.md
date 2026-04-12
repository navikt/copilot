# Nais-planleggingsguide per arketype

> **Maler:** For fullstendige, vedlikeholdte Nais-manifest-maler, bruk `@nais-agent`.
> Denne guiden hjelper deg velge riktig arketype og planlegge konfigurasjonen.

## Arketype-oversikt

| Arketype | Port | Auth | Lagring | Bruk `@nais-agent` for |
|----------|------|------|---------|----------------------|
| Backend API (Kotlin/Ktor) | 8080 | Azure AD + TokenX | PostgreSQL | Komplett manifest med auth, GCP SQL, accessPolicy |
| Hendelsekonsument (Kafka) | 8080 | — | PostgreSQL + Kafka | Manifest med kafka pool, topic-definisjon |
| Frontend (Next.js + ID-porten) | 3000 | ID-porten sidecar | — | Manifest med sidecar, ingress, autoLogin |
| Frontend (Next.js + Azure AD) | 3000 | Azure AD sidecar | — | Manifest med sidecar, ingress, autoLogin |
| Batchjobb (Naisjob) | — | Azure AD | PostgreSQL | Naisjob med schedule, activeDeadlineSeconds |

## Planlegging: Hva du må bestemme

### 1. Auth-mekanisme

Se [decision-trees.md](./decision-trees.md) Steg 1 for komplett auth-beslutningstre.

### 2. Tilgangsstyring (accessPolicy)

Alltid eksplisitt — bruk `@nais-agent` for å generere riktig accessPolicy:

- **Inbound:** Hvem kan kalle tjenesten din? (frontend, andre tjenester)
- **Outbound:** Hvem kaller tjenesten din? (PDL, andre APIer)
- ⚠️ Glem ikke `namespace` og `cluster` for tjenester utenfor eget namespace

### 3. Ressurser

Startpunkter (juster basert på behov):

```
requests:  cpu: 15m, memory: 256Mi
limits:    memory: 512Mi
replicas:  min: 2, max: 4
```

### 4. Database (PostgreSQL)

- Dev: `db-f1-micro` (standard)
- Prod: `db-custom-1-3840` med `highAvailability: true` og `diskAutoresize: true`

### 5. Kafka-topics

```
Navneformat: {team}.{domene}.v{versjon}
Dev:  partitions: 1, replication: 1
Prod: partitions: 6+, replication: 3
```

## Sjekkliste for nytt manifest

- [ ] Valgt riktig arketype
- [ ] Auth-mekanisme bestemt (se beslutningstreet)
- [ ] accessPolicy definert (inbound + outbound)
- [ ] Ressurser tilpasset forventet last
- [ ] Separate manifester for dev og prod (eller bruk variabler)
- [ ] Health-endepunkter implementert (`/isalive`, `/isready`, `/metrics`)
