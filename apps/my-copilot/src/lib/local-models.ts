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
import { MANIFEST_URL, buildTable, type LocalModelTable } from "./local-models-manifest";

export { MANIFEST_URL };

export type { LocalModel } from "./local-models-manifest";

const TIMEOUT_MS = 3000;

export const FALLBACK_TABLE: LocalModelTable = fallback;

export async function getLocalModels(): Promise<LocalModelTable> {
  try {
    const res = await fetch(MANIFEST_URL, {
      next: { revalidate: 3600 },
      signal: AbortSignal.timeout(TIMEOUT_MS),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return buildTable(await res.json());
  } catch (err) {
    console.error(`[local-models] using the checked-in fallback, manifest unavailable: ${String(err)}`);
    return FALLBACK_TABLE;
  }
}
