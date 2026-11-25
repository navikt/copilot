# Grafana dashboards

The folder says which Grafana a dashboard belongs in. A dashboard queries one Mimir/Loki/Tempo org only, never both.

| Folder    | Org (`X-Scope-OrgID`) | Data                                                                            |
| --------- | --------------------- | ------------------------------------------------------------------------------- |
| `tenant/` | `tenant`              | Our workloads in namespace `copilot`: app metrics, `kube_*`, `up`, logs, traces |
| `nais/`   | `nais`                | nav-pilot and Copilot client telemetry, which only exists in the nais org       |

Each dashboard's datasource variables (`DS_METRICS`, `DS_LOGS`, `DS_TRACES`) say in their description which org they must point at.

## Dashboards

| File                             | Title                                  | uid                       | Grafana folder | Tags                                   |
| -------------------------------- | -------------------------------------- | ------------------------- | -------------- | -------------------------------------- |
| `tenant/copilot-home.json`       | Copilot: overview                      | `copilot-ecosystem`       | Copilot        | `domain:copilot`, `concern:ops`        |
| `tenant/nav-fleet-ops.json`      | nav-fleet: operations                  | `copilot-fleet-ops`       | nav-fleet      | `domain:nav-fleet`, `concern:ops`      |
| `tenant/nav-fleet-insights.json` | nav-fleet: usage                       | `copilot-fleet-insights`  | nav-fleet      | `domain:nav-fleet`, `concern:insights` |
| `tenant/my-copilot-ops.json`     | my-copilot and copilot-api: operations | `copilot-web-ops`         | Copilot        | `domain:copilot`, `concern:ops`        |
| `nais/nav-pilot-cli.json`        | nav-pilot CLI (nais)                   | `copilot-nav-pilot-cli`   | nav-pilot      | `domain:nav-pilot`, `concern:insights` |
| `nais/nav-pilot-local.json`      | nav-pilot local inference (nais)       | `copilot-nav-pilot-local` | nav-pilot      | `domain:nav-pilot`, `concern:insights` |
| `nais/copilot-clients.json`      | GitHub Copilot clients (nais)          | `copilot-clients`         | nav-pilot      | `domain:nav-pilot`, `concern:insights` |

The home dashboard keeps the old `copilot-ecosystem` uid, so existing bookmarks land on it. The tenant files carry their uid, so links between them keep working. The nais files are complete Grafana 13 v2 resources (`apiVersion: dashboard.grafana.app/v2`) with the uid in `metadata.name`.

`nais/nav-pilot-local.json` is generated. Edit `scripts/build-local-dashboard.py`, not the JSON.

Panels titled "(placeholder)" keep the intended query for data that does not exist yet, with the issue that tracks it.

## Where the old files went

| Old file                     | Now                                                                                                                                                                                                                                      |
| ---------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `copilot-ecosystem.json`     | Split: fleet rows in `tenant/nav-fleet-ops.json` and `tenant/nav-fleet-insights.json`; ingress, Wonderwall, platform, logs and traces in `tenant/my-copilot-ops.json`; seats in `tenant/copilot-home.json`, which also keeps the old uid |
| `nav-pilot-cli.json`         | `nais/nav-pilot-cli.json`                                                                                                                                                                                                                |
| `nav-pilot-local.json`       | `nais/nav-pilot-local.json`                                                                                                                                                                                                              |
| `unified-ai-experience.json` | `nais/copilot-clients.json`                                                                                                                                                                                                              |
