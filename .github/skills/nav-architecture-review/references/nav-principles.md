# Navs arkitekturprinsipper

## Overordnede prinsipper

### 1. Team First
Autonome team med «sirkel av autonomi». Teamet eier sine tjenester og tar beslutninger innenfor sin domene. Architecture Advice Process: søk råd fra berørte parter, men ta beslutningen selv.

### 2. Essensiell kompleksitet
Fokuser på essensiell kompleksitet (forretningslogikk). Unngå accidental complexity (unødvendig teknisk kompleksitet). Velg den enkleste løsningen som løser problemet.

### 3. Produktutvikling
Kontinuerlig utvikling fremfor prosjektbasert. Produktorganisert gjenbruk gjennom delte plattformtjenester (Nais, Wonderwall, Kafkarator).

### 4. DORA-metrikker
Mål og forbedre team-ytelse med DevOps Research and Assessment:
- **Deployment frequency** — hvor ofte deployer vi?
- **Lead time for changes** — tid fra commit til prod
- **Change failure rate** — andel deploys som forårsaker feil
- **Time to restore** — tid til å fikse feil i prod

## Arkitekturvalg-prinsipper

### Foretrekk plattform-kapabiliteter
Bruk Nais sine innebygde løsninger fremfor å bygge egne:

| Behov | Plattformløsning | Ikke bygg selv |
|-------|-------------------|----------------|
| Auth | Wonderwall sidecar | Egen auth-proxy |
| Secret management | Nais secrets / Vault | Egne secret stores |
| Service discovery | Kubernetes DNS | Egen service registry |
| Load balancing | Kubernetes ingress | Egen load balancer |
| Database | Cloud SQL via Nais | Egen database-server |
| Kafka | Kafka via Nais | Egen Kafka-kluster |
| Metrics | Prometheus auto-scrape | Egen metrics-pipeline |
| Tracing | OpenTelemetry auto-instrumentation | Egen tracing |

### Foretrekk konvensjon over konfigurasjon
- Standard prosjektstruktur (se nav-plan skill)
- Standard Nais-manifest maler
- Standard CI/CD-workflows
- Standard auth-mønstre

### Foretrekk løs kobling
- Tjeneste-til-tjeneste via Kafka (asynkront) fremfor REST (synkront) der mulig
- API-kontrakter fremfor delte databaser
- Events fremfor direkte kall der mulig

### Foretrekk observerbarhet fra dag 1
- Prometheus-metrikker (forretning + teknisk)
- Strukturert logging (JSON, ingen PII)
- Distribuert tracing (OpenTelemetry)
- Alerting med meningsfulle terskelverdier

## Architecture Advice Process

Når du tar en arkitekturbeslutning som påvirker andre:

1. **Beskriv beslutningen** i en ADR
2. **Identifiser berørte parter** — team som konsumerer eller produserer relaterte data/tjenester
3. **Søk råd** — del ADR med berørte, be om innspill
4. **Ta beslutningen** — du (teamet) eier beslutningen
5. **Kommuniser** — del resultatet med berørte parter
6. **Dokumenter** — oppdater ADR med endelig status

Prosessen er rådgivende, ikke godkjenningsbasert. Teamet har beslutningsmyndighet innenfor sin domene.
