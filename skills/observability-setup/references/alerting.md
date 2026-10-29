# Prometheus alert rules and alerting practice

## Alert Rules (Nais PrometheusRule)

Nais alerts are a `PrometheusRule` resource in `.nais/alert.yml`, deployed like the app. Each rule needs a `namespace` label.

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: my-app-alerts
  namespace: myteam
  labels:
    team: myteam
spec:
  groups:
    - name: my-app-alerts
      rules:
        - alert: HighErrorRate
          expr: rate(http_requests_total{app="my-app",status=~"5.."}[5m]) > 0.05
          for: 5m
          labels:
            namespace: myteam # required
            severity: critical
          annotations:
            summary: "Error rate is {{ $value | humanizePercentage }}"
            consequence: "Users get errors"
            action: "Check logs in Grafana Loki"

        - alert: HighResponseTime
          expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket{app="my-app"}[5m])) > 1
          for: 10m
          labels:
            namespace: myteam
            severity: warning
          annotations:
            summary: "95th percentile response time is {{ $value }}s"

        - alert: PodNotReady
          expr: kube_pod_status_ready{namespace="myteam",condition="false"} > 0
          for: 5m
          labels:
            namespace: myteam
            severity: critical
          annotations:
            summary: "Pod is not ready"
```

Source: [Nais alerting docs](https://doc.nais.io/observability/alerting/how-to/prometheus-basic/).

## Alerting Best Practices

1. **Alert on symptoms, not causes**
2. **Set appropriate thresholds**
3. **Include `action` and runbook links in annotations**
4. **Avoid alert fatigue**
5. **Test alerts in staging**

## Common Nais Alert Patterns

Rules to put under `spec.groups[].rules` in the `PrometheusRule` above. Add `namespace: myteam` to each rule's labels.

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
