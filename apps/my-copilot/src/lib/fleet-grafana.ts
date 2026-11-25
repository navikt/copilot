/** Link to the fleet-gateway usage dashboard for one route, in the cluster my-copilot runs in. */
export function fleetGrafanaUrl(route: string, cluster = process.env.NAIS_CLUSTER_NAME ?? ""): string {
  // Local dev has no cluster and talks to dev fleet-control.
  const env = !cluster || cluster.startsWith("dev-") ? "dev" : "prod";
  return `https://grafana.nav.cloud.nais.io/d/copilot-fleet-insights?orgId=1&var-route=${encodeURIComponent(route)}&var-env=${env}`;
}
