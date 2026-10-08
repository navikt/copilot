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

/** fetchedAt is when the live manifest was read, or null for the checked-in copy. */
export async function getLocalModels(): Promise<LocalModelTable & { fetchedAt: string | null }> {
  try {
    const res = await fetch(MANIFEST_URL, {
      next: { revalidate: 3600 },
      signal: AbortSignal.timeout(TIMEOUT_MS),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return { ...buildTable(await res.json()), fetchedAt: new Date().toISOString() };
  } catch (err) {
    console.error(`[local-models] using the checked-in fallback, manifest unavailable: ${String(err)}`);
    return { ...FALLBACK_TABLE, fetchedAt: null };
  }
}
