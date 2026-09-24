/**
 * Validation and mapping of navikt/mlx-workspace's manifest/models.json, the
 * manifest nav-pilot reads at `alpha local init` and `start`, onto the shape
 * the nav-pilot docs page renders.
 *
 * Shared by the page's runtime loader (local-models.ts) and
 * scripts/sync-local-models.mjs, which writes the checked-in fallback copy
 * (local-models.json). One module, so the two cannot drift apart.
 *
 * Plain erasable TypeScript without imports: the sync script loads this file
 * straight into Node, which strips the types.
 *
 * Only the fields the page shows are kept. The per-class run counts are left
 * out so that a benchmark rerun that changes no verdict leaves the fallback
 * copy alone. The manifest's `key` is written as `id`, because gitleaks reads
 * `"key": "<slug>"` as an API key.
 */

export const MANIFEST_URL = "https://raw.githubusercontent.com/navikt/mlx-workspace/main/manifest/models.json";

export type LocalModel = {
  id: string;
  name: string;
  model: string;
  default: boolean;
  role: string;
  weights_gb: number;
  min_ram_gb: number;
  min_nav_pilot: string | null;
  context: number;
  output: number;
  temperature: number | null;
  top_p: number | null;
  prefill_step: number | null;
  classes: Record<string, { delegate: string; local: string }>;
};

export type LocalModelTable = { source: string; models: LocalModel[] };

type Obj = Record<string, unknown>;

function isPlainObject(v: unknown): v is Obj {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

// nav-pilot's release-version format (cli/nav-pilot/internal/agentpakke/version.go).
const RELEASE_VERSION = /^\d{4}\.\d{2}\.\d{2}-\d{6}(-[0-9a-zA-Z][0-9a-zA-Z.-]*)?$/;

/** A manifest param as a number, or null when unset. Params are strings; anything else throws. */
function num(params: Obj, name: string): number | null {
  const v = params[name];
  if (v === undefined) return null;
  const n = typeof v === "string" && v.trim() !== "" ? Number(v) : NaN;
  if (!Number.isFinite(n)) throw new Error(`param ${name}=${JSON.stringify(v)} is not a number string`);
  return n;
}

/** A required positive integer field. */
function int(e: Obj, name: string): number {
  const v = e[name];
  if (typeof v !== "number" || !Number.isInteger(v) || v <= 0) throw new Error(`${e.key} has no valid ${name}`);
  return v;
}

/**
 * min_nav_pilot: null when omitted. nav-pilot withholds an entry whose value is
 * present but unreadable, so the table must not show it as available: throw.
 */
function minVersion(e: Obj): string | null {
  if (!("min_nav_pilot" in e)) return null;
  const v = e.min_nav_pilot;
  if (typeof v !== "string" || !RELEASE_VERSION.test(v)) {
    throw new Error(`${e.key} has an unreadable min_nav_pilot ${JSON.stringify(v)}`);
  }
  return v;
}

/** Project one manifest entry onto what the page renders. Throws on a bad entry. */
function projectEntry(e: unknown): LocalModel {
  if (!isPlainObject(e)) throw new Error("model entry is not an object");
  const str = (f: string) => {
    const v = e[f];
    if (typeof v !== "string" || !v) throw new Error(`model entry lacks ${f}`);
    return v;
  };
  const [key, name, model, role] = ["key", "name", "model", "role"].map(str);
  const params = isPlainObject(e.params) ? e.params : {};
  const context = num(params, "MLX_OPENCODE_CONTEXT");
  const output = num(params, "MLX_OPENCODE_OUTPUT");
  if (context === null || output === null) {
    throw new Error(`${key} has no MLX_OPENCODE_CONTEXT/OUTPUT`);
  }
  const classes: LocalModel["classes"] = {};
  const caps = e.capabilities;
  const raw = isPlainObject(caps) && isPlainObject(caps.classes) ? caps.classes : {};
  for (const [id, c] of Object.entries(raw)) {
    if (!isPlainObject(c)) continue;
    classes[id] = { delegate: String(c.delegate ?? ""), local: String(c.local ?? "") };
  }
  return {
    id: key,
    name,
    model,
    default: e.default === true,
    role,
    weights_gb: int(e, "weights_gb"),
    min_ram_gb: int(e, "min_ram_gb"),
    min_nav_pilot: minVersion(e),
    context,
    output,
    temperature: num(params, "MLX_NAV_PILOT_TEMPERATURE"),
    top_p: num(params, "MLX_NAV_PILOT_TOP_P"),
    prefill_step: num(params, "MLX_PREFILL_STEP_SIZE"),
    classes,
  };
}

/** Project the whole manifest. Refuses an empty list or one without exactly one default. */
export function buildTable(manifest: unknown): LocalModelTable {
  if (!isPlainObject(manifest) || !Array.isArray(manifest.models)) {
    throw new Error("manifest has no models list");
  }
  const models = manifest.models.map(projectEntry);
  if (models.length === 0) throw new Error("manifest lists no models; refusing to empty the table");
  const defaults = models.filter((m) => m.default).length;
  if (defaults !== 1) throw new Error(`manifest has ${defaults} default models, want 1`);
  return { source: MANIFEST_URL, models };
}
