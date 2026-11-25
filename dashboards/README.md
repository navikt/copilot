# Grafana dashboards

The folder says which Grafana a dashboard belongs in. A dashboard queries one Mimir/Loki/Tempo org only, never both.

| Folder    | Org (`X-Scope-OrgID`) | Data                                                                            |
| --------- | --------------------- | ------------------------------------------------------------------------------- |
| `tenant/` | `tenant`              | Our workloads in namespace `copilot`: app metrics, `kube_*`, `up`, logs, traces |
| `nais/`   | `nais`                | nav-pilot and Copilot client telemetry, which only exists in the nais org       |

Each dashboard's datasource variables (`DS_METRICS`, `DS_LOGS`, `DS_TRACES`) say in their description which org they must point at.

## Dashboards

| File                                 | Title                             | uid                           | Grafana folder | Tags                                   |
| ------------------------------------ | --------------------------------- | ----------------------------- | -------------- | -------------------------------------- |
| `tenant/copilot-home.json`           | Copilot: oversikt                 | `copilot-ecosystem`           | Copilot        | `domain:copilot`, `concern:ops`        |
| `tenant/nav-fleet-ops.json`          | nav-fleet: drift                  | `copilot-fleet-ops`           | nav-fleet      | `domain:nav-fleet`, `concern:ops`      |
| `tenant/nav-fleet-insights.json`     | nav-fleet: bruk                   | `copilot-fleet-insights`      | nav-fleet      | `domain:nav-fleet`, `concern:insights` |
| `tenant/my-copilot-ops.json`         | my-copilot og copilot-api: drift  | `copilot-web-ops`             | Copilot        | `domain:copilot`, `concern:ops`        |
| `tenant/copilot-seats-insights.json` | Copilot-lisenser                  | `copilot-seats-insights`      | Copilot        | `domain:copilot`, `concern:insights`   |
| `nais/nav-pilot-cli.json`            | nav-pilot CLI (nais)              | `copilot-navpilot-cli-nais`   | nav-pilot      | `domain:nav-pilot`, `concern:insights` |
| `nais/nav-pilot-local.json`          | nav-pilot — lokal inferens (nais) | `copilot-navpilot-local-nais` | nav-pilot      | `domain:nav-pilot`, `concern:insights` |
| `nais/copilot-clients.json`          | GitHub Copilot-klienter (nais)    | `copilot-clients-nais`        | nav-pilot      | `domain:nav-pilot`, `concern:insights` |

The home dashboard keeps the old `copilot-ecosystem` uid, so existing bookmarks land on it. The tenant files carry their uid, so links between them keep working. The nais files use Grafana's v2 format without a uid: give the uid above when you import them.

`nais/nav-pilot-local.json` is generated. Edit `scripts/build-local-dashboard.py`, not the JSON.

Panels titled "(placeholder)" keep the intended query for data that does not exist yet, with the issue that tracks it.

## Where the old files went

| Old file                     | Now                                                                                                                                                                                                                  |
| ---------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `copilot-ecosystem.json`     | Split: fleet rows in `tenant/nav-fleet-ops.json` and `tenant/nav-fleet-insights.json`; ingress, Wonderwall, platform, logs and traces in `tenant/my-copilot-ops.json`; seats in `tenant/copilot-seats-insights.json` |
| `nav-pilot-cli.json`         | `nais/nav-pilot-cli.json`                                                                                                                                                                                            |
| `nav-pilot-local.json`       | `nais/nav-pilot-local.json`                                                                                                                                                                                          |
| `unified-ai-experience.json` | `nais/copilot-clients.json`                                                                                                                                                                                          |
