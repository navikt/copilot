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
 * Only the fields the pages show are kept, including the per-class run counts
 * (k passed of n) and the bar behind each verdict. The manifest's `key` is
 * written as `id`, because gitleaks reads `"key": "<slug>"` as an API key.
 *
 * Rejected models come from manifest/capabilities.json: every model with a
 * capability block there that models.json no longer lists.
 */

const RAW = "https://raw.githubusercontent.com/navikt/mlx-workspace/main/manifest";
export const MANIFEST_URL = `${RAW}/models.json`;
export const CAPABILITIES_URL = `${RAW}/capabilities.json`;

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
  bar: Bar | null;
  classes: Classes;
};

/** The approval bar, see design.md §2.1 in navikt/mlx-workspace. */
export type Bar = { confidence: number; x_caught: number; x_silent: number; min_runs: number; min_tasks: number };

/** Verdict and run counts per task class: k runs passed of n, per mode. */
export type ClassVerdict = {
  delegate: string;
  local: string;
  delegate_k: number;
  delegate_n: number;
  local_k: number;
  local_n: number;
};
export type Classes = Record<string, ClassVerdict>;

/** A model measured but not (or no longer) in the manifest. replaced_by is the id of its successor, if any. */
export type RejectedModel = { model: string; replaced_by: string | null; bar: Bar | null; classes: Classes };

export type LocalModelTable = { source: string; models: LocalModel[]; rejected: RejectedModel[] };

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

/** A run count, 0 when missing. Anything but a non-negative integer throws. */
function count(c: Obj, name: string): number {
  const v = c[name] ?? 0;
  if (typeof v !== "number" || !Number.isInteger(v) || v < 0)
    throw new Error(`${name}=${JSON.stringify(v)} is not a count`);
  return v;
}

function projectBar(b: unknown): Bar | null {
  if (!isPlainObject(b)) return null;
  const f = (n: string, int = false) => {
    const v = b[n];
    if (typeof v !== "number" || !Number.isFinite(v)) throw new Error(`bar.${n} is not a number`);
    if (int ? !Number.isInteger(v) || v < 0 : v < 0 || v > 1) throw new Error(`bar.${n}=${v} is out of range`);
    return v;
  };
  return {
    confidence: f("confidence"),
    x_caught: f("x_caught"),
    x_silent: f("x_silent"),
    min_runs: f("min_runs", true),
    min_tasks: f("min_tasks", true),
  };
}

/** A capability block ({bar, classes}), from models.json or capabilities.json. */
function projectCapabilities(caps: unknown): { bar: Bar | null; classes: Classes } {
  const classes: Classes = {};
  const raw = isPlainObject(caps) && isPlainObject(caps.classes) ? caps.classes : {};
  for (const [id, c] of Object.entries(raw)) {
    if (!isPlainObject(c)) continue;
    classes[id] = {
      delegate: String(c.delegate ?? ""),
      local: String(c.local ?? ""),
      delegate_k: count(c, "delegate_k"),
      delegate_n: count(c, "delegate_n"),
      local_k: count(c, "local_k"),
      local_n: count(c, "local_n"),
    };
  }
  return { bar: projectBar(isPlainObject(caps) ? caps.bar : undefined), classes };
}

/** Models in capabilities.json's manifest_blocks that the manifest does not list, sorted by name. */
function projectRejected(capabilities: unknown, manifest: Obj, models: LocalModel[]): RejectedModel[] {
  if (!isPlainObject(capabilities) || !isPlainObject(capabilities.manifest_blocks)) {
    throw new Error("capabilities has no manifest_blocks");
  }
  const replaced = isPlainObject(manifest.replaced) ? manifest.replaced : {};
  const listed = new Set(models.map((m) => m.model));
  return Object.entries(capabilities.manifest_blocks)
    .filter(([model]) => !listed.has(model))
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([model, block]) => ({
      model,
      replaced_by: typeof replaced[model] === "string" ? (replaced[model] as string) : null,
      ...projectCapabilities(block),
    }));
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
  const { bar, classes } = projectCapabilities(e.capabilities);
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
    bar,
    classes,
  };
}

/**
 * Project the manifest (models.json) and capabilities.json. Refuses an empty
 * list, one without exactly one default, or capabilities without manifest_blocks.
 */
export function buildTable(manifest: unknown, capabilities: unknown): LocalModelTable {
  if (!isPlainObject(manifest) || !Array.isArray(manifest.models)) {
    throw new Error("manifest has no models list");
  }
  const models = manifest.models.map(projectEntry);
  if (models.length === 0) throw new Error("manifest lists no models; refusing to empty the table");
  const defaults = models.filter((m) => m.default).length;
  if (defaults !== 1) throw new Error(`manifest has ${defaults} default models, want 1`);
  return { source: MANIFEST_URL, models, rejected: projectRejected(capabilities, manifest, models) };
}
