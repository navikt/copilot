/**
 * The local-model table on the nav-pilot docs page, read from
 * navikt/mlx-workspace's manifest at run time and revalidated hourly.
 *
 * On a fetch error, a timeout, a non-200 answer or a manifest that fails
 * validation, the page falls back to the checked-in local-models.json, which
 * scripts/sync-local-models.mjs refreshes. Every string in the table is
 * rendered as text, never as HTML.
 */
import fallback from "./local-models.json";
import {
  CAPABILITIES_URL,
  MANIFEST_URL,
  REPORTS_URL,
  buildReports,
  buildTable,
  type LocalModelTable,
  type ReportIndex,
} from "./local-models-manifest";

export { MANIFEST_URL };

export type { LocalModel, RejectedModel, ClassVerdict, Bar, Report, ReportIndex } from "./local-models-manifest";

const TIMEOUT_MS = 3000;

const { reports: fallbackReports, ...fallbackTable } = fallback;
export const FALLBACK_TABLE: LocalModelTable = fallbackTable;
export const FALLBACK_REPORTS = fallbackReports as ReportIndex;

/** One file from navikt/mlx-workspace, cached for an hour. Throws on timeout or a non-200 answer. */
async function fetchJson(url: string): Promise<{ body: unknown; date: string | null }> {
  const res = await fetch(url, { next: { revalidate: 3600 }, signal: AbortSignal.timeout(TIMEOUT_MS) });
  if (!res.ok) throw new Error(`HTTP ${res.status} for ${url}`);
  return { body: await res.json(), date: res.headers.get("date") };
}

/**
 * fetchedAt is when the live manifest was read, or null for the checked-in copy.
 * It comes from the response's Date header, which the fetch cache keeps, so a
 * cache hit reports the original fetch rather than render time.
 */
export async function getLocalModels(): Promise<LocalModelTable & { fetchedAt: string | null }> {
  try {
    const [manifest, capabilities] = await Promise.all([fetchJson(MANIFEST_URL), fetchJson(CAPABILITIES_URL)]);
    return {
      ...buildTable(manifest.body, capabilities.body),
      fetchedAt: new Date(manifest.date ?? Date.now()).toISOString(),
    };
  } catch (err) {
    console.error(`[local-models] using the checked-in fallback, manifest unavailable: ${String(err)}`);
    return { ...FALLBACK_TABLE, fetchedAt: null };
  }
}

/** The report index, fetched on its own so a bad reports.json never hides the model table. */
export async function getLocalReports(): Promise<ReportIndex> {
  try {
    return buildReports((await fetchJson(REPORTS_URL)).body);
  } catch (err) {
    console.error(`[local-models] using the checked-in report index, reports.json unavailable: ${String(err)}`);
    return FALLBACK_REPORTS;
  }
}
