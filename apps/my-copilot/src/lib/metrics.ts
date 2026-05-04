interface MetricsStore {
  pageViews: Map<string, number>;
}

const METRICS_KEY = "__mycopilot_metrics__";

function getStore(): MetricsStore {
  const g = globalThis as unknown as Record<string, MetricsStore>;
  if (!g[METRICS_KEY]) {
    g[METRICS_KEY] = { pageViews: new Map() };
  }
  return g[METRICS_KEY];
}

export function getEngagementMetrics(): string {
  const store = getStore();
  if (store.pageViews.size === 0) return "";

  const lines: string[] = [
    "# HELP mycopilot_page_views_total Total page views by section",
    "# TYPE mycopilot_page_views_total counter",
  ];
  for (const [page, count] of store.pageViews) {
    lines.push(`mycopilot_page_views_total{page="${page}"} ${count}`);
  }
  return lines.join("\n");
}
