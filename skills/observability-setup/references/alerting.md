# Prometheus alert rules and alerting practice

## Alert Rules (Prometheus)

```yaml
groups:
  - name: app-alerts
    interval: 30s
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value | humanizePercentage }}"

      - alert: HighResponseTime
        expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 1
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High response time detected"
          description: "95th percentile response time is {{ $value }}s"

      - alert: PodNotReady
        expr: kube_pod_status_ready{condition="false"} > 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Pod is not ready"
```

## Alerting Best Practices

1. **Alert on symptoms, not causes**
2. **Set appropriate thresholds**
3. **Include runbooks in annotations**
4. **Avoid alert fatigue**
5. **Test alerts in staging**

## Common Nais Alert Patterns

```yaml
# Application availability
- alert: ApplicationDown
  expr: up{app="my-app"} == 0
  for: 2m
  labels:
    severity: critical
    team: myteam
  annotations:
    summary: "Application {{ $labels.app }} is down"
    description: "No instances of {{ $labels.app }} are running"
    runbook: "https://teamdocs/runbooks/app-down.md"

# High memory usage
- alert: HighMemoryUsage
  expr: |
    (container_memory_working_set_bytes{app="my-app"}
    / container_spec_memory_limit_bytes{app="my-app"}) > 0.9
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "High memory usage on {{ $labels.pod }}"
    description: "Memory usage is {{ $value | humanizePercentage }}"

# Database connection pool exhaustion
- alert: DatabaseConnectionPoolExhausted
  expr: |
    hikaricp_connections_active
    / hikaricp_connections_max > 0.9
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Database connection pool almost full"

# Kafka consumer lag
- alert: KafkaConsumerLag
  expr: kafka_consumer_lag > 10000
  for: 15m
  labels:
    severity: warning
  annotations:
    summary: "High Kafka consumer lag on {{ $labels.topic }}"
    description: "Consumer lag is {{ $value }}"

# DORA: Deployment frequency (low)
- alert: LowDeploymentFrequency
  expr: |
    sum(increase(deployments_total{team="myteam"}[7d]))
    < 5
  labels:
    severity: info
  annotations:
    summary: "Low deployment frequency for team"
    description: "Only {{ $value }} deployments in last 7 days"

# DORA: Lead time for changes (high)
- alert: HighLeadTime
  expr: |
    histogram_quantile(0.95,
      rate(deployment_lead_time_seconds_bucket[1d])
    ) > 86400
  labels:
    severity: info
  annotations:
    summary: "High lead time for changes"
    description: "95th percentile lead time is {{ $value | humanizeDuration }}"
```
