// Link guard: every URL and anchor on ki-utvikling.nav.no that has ever been
// linked or published must keep resolving.
//
// src/lib/link-inventory.json lists every path and anchor that must resolve.
// The tests below check, without a server or network, that
//   1. each path matches a route in src/app (a dynamic segment only matches a
//      slug that exists), a file in public/, or a permanent redirect in
//      next.config.ts to one of those;
//   2. each anchor is defined in the source of the page it lands on, or is
//      mapped to one that is in src/lib/legacy-anchors.ts;
//   3. every site link found in the repo, every route, and every anchor a page
//      defines today is in the inventory, so new ones are added on purpose.
//
// After adding a page, anchor or link, run `pnpm link-inventory:update`. It
// only adds entries. Never delete one: add a redirect or a legacy anchor.
import { findByItemKey } from "@/lib/item-key";
import fs from "node:fs";
import path from "node:path";
import nextConfig from "../next.config";
import sitemap from "@/app/sitemap";
import { allGuides } from "@/app/(nb)/praksis/data";
import { sourceHeadings } from "@/lib/page-headings";
import { INSTALL_TYPES } from "@/lib/install-redirect";
import { LEGACY_ANCHORS } from "@/lib/legacy-anchors";
import { getArticle, getLinkTarget } from "@/lib/news";
import inventoryJson from "@/lib/link-inventory.json";
import { autoLoginIgnorePaths, matches } from "../scripts/auto-login-ignore-paths.mjs";
import manifest from "@/lib/copilot-manifest.json";
import mcpAllowlist from "../../mcp-registry/allowlist.json";

type Entry = { sources: string[]; anchors: Record<string, string[]> };
type Found = { path: string; anchor?: string; item?: string; source: string };

const inventory: Record<string, Entry> = inventoryJson;
const APP_ROOT = path.resolve(__dirname, "..");
const APP_DIR = path.join(APP_ROOT, "src", "app");
const REPO = path.resolve(APP_ROOT, "..", "..");
const INVENTORY_FILE = path.join(APP_ROOT, "src", "lib", "link-inventory.json");
const rel = (file: string) => path.relative(REPO, file);

// Dynamic segments and which values exist. A new dynamic route must be added
// here, or the test below fails.
const DYNAMIC: Record<string, (value: string) => boolean> = {
  "/nyheter/[slug]": (s) => getArticle(s) !== null || getLinkTarget(s) !== null,
  "/en/news/[slug]": (s) => getArticle(s, "en") !== null || getLinkTarget(s, "en") !== null,
  "/praksis/guide/[slug]": (s) => allGuides.some((g) => g.id === s),
  "/install/[type]": (s) => Object.hasOwn(INSTALL_TYPES, s),
  // ponytail: video ids come from a GCS feed at request time, so any id passes.
  // Check against the feed if video links start appearing in the repo.
  "/videos/[id]": () => true,
};

// Redirects that are deliberately temporary: aliases, not moved pages.
const TEMPORARY_REDIRECTS = new Set(["/en", "/nyheter"]);

// Served by Wonderwall, the login sidecar in front of the app.
const PLATFORM_PREFIXES = ["/oauth2/"];

// ── routes ───────────────────────────────────────────────────────────────────

type Route = { pattern: string; file: string };

function routePattern(file: string): string {
  const segments = path
    .relative(APP_DIR, path.dirname(file))
    .split(path.sep)
    .filter((s) => s && !(s.startsWith("(") && s.endsWith(")")));
  return "/" + segments.join("/");
}

const routes: Route[] = fs
  .globSync("**/{page.tsx,route.ts}", { cwd: APP_DIR })
  .map((f) => path.join(APP_DIR, f))
  .map((file) => ({ pattern: routePattern(file), file }))
  .filter((r) => !r.pattern.startsWith("/api") && r.pattern !== "/health");

function matchRoute(pathname: string): Route | undefined {
  const parts = pathname.split("/").filter(Boolean);
  const fits = (r: Route, dynamic: boolean) => {
    const segs = r.pattern.split("/").filter(Boolean);
    return (
      segs.length === parts.length &&
      segs.every((s, i) => s === parts[i] || (dynamic && s.startsWith("[") && DYNAMIC[r.pattern]?.(parts[i])))
    );
  };
  return routes.find((r) => fits(r, false)) ?? routes.find((r) => fits(r, true));
}

// Host redirects (old min-copilot hosts) never change a path on the site's own
// host, so the inventory skips them; src/host-redirect.test.ts covers them.
const redirects = (await nextConfig.redirects!()).filter((r) => !r.has);

// Where a path ends up: a page, a route handler or a public file. Returns an
// error message when it does not resolve.
function resolvePath(pathname: string, hops = 0): { file: string } | { error: string } {
  const redirect = redirects.find((r) => r.source === pathname);
  if (redirect) {
    if (!redirect.permanent && !TEMPORARY_REDIRECTS.has(pathname)) {
      return {
        error: `redirect ${pathname} → ${redirect.destination} is not permanent; set permanent: true in next.config.ts or add it to TEMPORARY_REDIRECTS`,
      };
    }
    if (hops > 5) return { error: `redirect loop at ${pathname}` };
    return resolvePath(redirect.destination.split("#")[0], hops + 1);
  }
  if (PLATFORM_PREFIXES.some((prefix) => pathname.startsWith(prefix))) return { file: "" };
  const route = matchRoute(pathname);
  if (route) return { file: route.file };
  if (pathname === "/sitemap.xml") return { file: path.join(APP_DIR, "sitemap.ts") };
  // Written into public/ at build time (scripts/build-search-index.ts), so not there in CI.
  if (pathname === "/news.json") return { file: path.join(APP_ROOT, "scripts", "build-search-index.ts") };
  const publicFile = path.join(APP_ROOT, "public", decodeURIComponent(pathname));
  if (pathname !== "/" && fs.existsSync(publicFile) && fs.statSync(publicFile).isFile()) return { file: publicFile };
  return { error: `no route, public file or redirect for ${pathname}` };
}

// The path the browser shows after following redirects. LEGACY_ANCHORS is
// keyed by it, since that is what HashAnchorScroll sees.
function landingPath(pathname: string, hops = 0): string {
  const redirect = redirects.find((r) => r.source === pathname);
  return redirect && hops <= 5 ? landingPath(redirect.destination.split("#")[0], hops + 1) : pathname;
}

// ── anchors ──────────────────────────────────────────────────────────────────

// The source files of a page: everything in its directory except nested routes,
// and the files those import from src/components or by relative path, one
// level down.
function pageSources(routeFile: string): string[] {
  const dir = path.dirname(routeFile);
  const own = fs
    .globSync("**/*.{ts,tsx}", { cwd: dir })
    .filter((f) => !/\.(test|stories)\.tsx?$/.test(f))
    .filter((f) => {
      const parts = path
        .dirname(f)
        .split(path.sep)
        .filter((p) => p !== ".");
      return !parts.some((_, i) =>
        ["page.tsx", "route.ts"].some((n) => fs.existsSync(path.join(dir, ...parts.slice(0, i + 1), n)))
      );
    })
    .map((f) => path.join(dir, f));
  return [...new Set([...own, ...own.flatMap(imports)])];
}

// ponytail: static `from "…"` imports only; dynamic import() and re-exports
// are not followed. Follow them if an anchor in one goes missing.
function imports(file: string): string[] {
  return [...fs.readFileSync(file, "utf-8").matchAll(/from\s+"(@\/components\/[^"]+|\.\.?\/[^"]+)"/g)].flatMap((m) => {
    const base = m[1].startsWith("@/")
      ? path.join(APP_ROOT, "src", m[1].slice(2))
      : path.resolve(path.dirname(file), m[1]);
    const hit = ["", ".tsx", ".ts", "/index.tsx", "/index.ts"]
      .map((ext) => base + ext)
      .find((f) => fs.existsSync(f) && fs.statSync(f).isFile());
    return hit && !/\.(test|stories)\.tsx?$/.test(hit) ? [hit] : [];
  });
}

// Ids a page defines: literal id="…" attributes, LinkableHeading ids (explicit,
// or slugified from plain-text children, as the component does) and Tabs
// hashIds. With headingsOnly, other id="…" attributes are left out: they only
// need to keep working once something links to them, and the repo scan finds
// those links.
function definedAnchors(routeFile: string, headingsOnly = false): Map<string, string> {
  const ids = new Map<string, string>();
  for (const file of pageSources(routeFile)) {
    const src = fs.readFileSync(file, "utf-8");
    for (const m of src.matchAll(/(?<![\w-])id="([^"]+)"/g)) {
      const tag = src.slice(src.lastIndexOf("<", m.index) + 1).match(/^[\w.]+/)?.[0] ?? "";
      if (!headingsOnly || tag === "LinkableHeading") ids.set(m[1], rel(file));
    }
    for (const h of sourceHeadings(src)) {
      if (h.tag === "LinkableHeading" && h.id) ids.set(h.id, rel(file));
    }
    for (const m of src.matchAll(/hashIds:\s*\[([^\]]*)\]/g)) {
      for (const id of m[1].matchAll(/"([^"]+)"/g)) ids.set(id[1], rel(file));
    }
  }
  return ids;
}

// ── scanning the repo for links ──────────────────────────────────────────────

const SITE_URL = /https?:\/\/(?:ki-utvikling\.nav\.no|min-copilot\.ansatt\.nav\.no)(\/[^\s"'`)<>\]]*)?/g;
// Relative links in code the site renders.
const CODE_LINK =
  /(?:\b(?:href|link|to)|Href)\s*[=:]\s*\{?\s*["'`](\/[^"'`\s]*)["'`]|\b(?:redirect|permanentRedirect|push|replace)\(\s*["'`](\/[^"'`\s]*)["'`]/g;
const MARKDOWN_LINK = /\]\((\/[^)\s]*)\)|^\[[^\]]+\]:\s*(\/\S+)/gm;

// Where site links can hide. Relative links only count in files the site
// renders: its own source and the news articles. The my-copilot path filter in
// .github/workflows/ci.yaml covers the same files, so a change to any of them
// runs this test.
const SCAN = {
  absolute: [
    "**/*.md",
    "cli/**/*.go",
    "scripts/**/*.{sh,mjs,ts,go,py}",
    "apps/my-copilot/src/**/*.{ts,tsx}",
    ".github/ISSUE_TEMPLATE/*",
    "apps/my-copilot/public/*.txt",
  ],
  code: "apps/my-copilot/src/**/*.{ts,tsx}",
  markdown: "docs/news/articles/*.md",
};

const IGNORED = /(^|\/)(node_modules|\.next|dist|coverage)\/|\.test\.tsx?$|\.stories\.tsx?$|_test\.go$/;
const glob = (pattern: string) =>
  fs
    .globSync(pattern, { cwd: REPO })
    .filter((f) => !IGNORED.test(f))
    .map((f) => path.join(REPO, f));

function parseLink(raw: string, source: string, base?: string): Found | undefined {
  let link = raw.replace(/[.,;:!?*]+$/, "");
  if (link.startsWith("#")) {
    if (!base || link === "#") return undefined;
    link = base + link;
  }
  if (link.startsWith("//") || /\$\{|%[sdv]/.test(link)) return undefined;
  const [beforeHash, anchor] = link.split("#");
  const [before, query] = beforeHash.split("?");
  let pathname = before || "/";
  if (pathname.length > 1) pathname = pathname.replace(/\/$/, "");
  const item = new URLSearchParams(query).get("item") ?? undefined;
  return { path: pathname, anchor: anchor ? decodeURIComponent(anchor) : undefined, item, source };
}

// The route a file in src/app belongs to, for same-page #links.
function routeOfFile(file: string): string | undefined {
  let dir = path.dirname(file);
  while (dir.startsWith(APP_DIR)) {
    const page = path.join(dir, "page.tsx");
    if (fs.existsSync(page)) return routePattern(page);
    dir = path.dirname(dir);
  }
  return undefined;
}

function scanRepo(): Found[] {
  const found: Found[] = [];
  const add = (f: Found | undefined) => f && found.push(f);

  for (const file of new Set(SCAN.absolute.flatMap(glob))) {
    for (const m of fs.readFileSync(file, "utf-8").matchAll(SITE_URL)) add(parseLink(m[1] ?? "/", rel(file)));
  }
  for (const file of glob(SCAN.code)) {
    const src = fs.readFileSync(file, "utf-8");
    const base = routeOfFile(file);
    for (const m of src.matchAll(CODE_LINK)) add(parseLink(m[1] ?? m[2], rel(file)));
    for (const m of src.matchAll(/(?:[hH]ref|to)\s*=\s*\{?\s*["'`](#[^"'`\s$]*)["'`]/g))
      add(parseLink(m[1], rel(file), base));
    // Table of contents entries are same-page links.
    if (base && src.includes("<TableOfContents")) {
      for (const m of src.matchAll(/\{\s*id:\s*"([^"]+)",\s*label:/g))
        add({ path: base, anchor: m[1], source: rel(file) });
    }
  }
  for (const file of glob(SCAN.markdown)) {
    const src = fs.readFileSync(file, "utf-8");
    for (const m of src.matchAll(MARKDOWN_LINK)) add(parseLink(m[1] ?? m[2], rel(file)));
    for (const m of src.matchAll(CODE_LINK)) add(parseLink(m[1] ?? m[2], rel(file)));
  }
  for (const item of sitemap()) add(parseLink(new URL(item.url).pathname, rel(path.join(APP_DIR, "sitemap.ts"))));
  return found;
}

// Everything that exists today: routes, dynamic pages, redirects and anchors.
function currentSite(): Found[] {
  const found: Found[] = [];
  for (const route of routes) {
    const source = rel(route.file);
    if (!route.pattern.includes("[")) {
      found.push({ path: route.pattern, source });
      for (const [anchor, file] of definedAnchors(route.file, true))
        found.push({ path: route.pattern, anchor, source: file });
    }
  }
  const slugs = (lang: "nb" | "en") =>
    fs
      .globSync("*.md", { cwd: path.join(REPO, "docs", "news", "articles") })
      .map((f) => f.replace(/\.md$/, ""))
      .filter((s) => DYNAMIC[lang === "nb" ? "/nyheter/[slug]" : "/en/news/[slug]"](s));
  const dynamicPages: Record<string, string[]> = {
    "/nyheter/[slug]": slugs("nb"),
    "/en/news/[slug]": slugs("en"),
    "/praksis/guide/[slug]": allGuides.map((g) => g.id),
    "/install/[type]": Object.keys(INSTALL_TYPES),
    "/videos/[id]": [],
  };
  for (const route of routes.filter((r) => r.pattern.includes("["))) {
    for (const value of dynamicPages[route.pattern] ?? []) {
      found.push({ path: route.pattern.replace(/\[[^\]]+\]/, value), source: rel(route.file) });
    }
  }
  for (const r of redirects) found.push({ path: r.source, source: "apps/my-copilot/next.config.ts" });
  return found;
}

function key(f: Found) {
  return f.anchor ? `${f.path}#${f.anchor}` : f.path;
}

function missingFromInventory(found: Found[]): Found[] {
  return found.filter((f) => {
    const entry = inventory[f.path];
    return !entry || (f.anchor !== undefined && !(f.anchor in entry.anchors));
  });
}

// Adds what was found to the inventory file. Never removes anything.
function updateInventory(found: Found[]) {
  const next: Record<string, Entry> = structuredClone(inventory);
  const addSource = (list: string[], source: string) => {
    if (!list.includes(source)) list.push(source);
    list.sort();
  };
  for (const f of found) {
    const entry = (next[f.path] ??= { sources: [], anchors: {} });
    if (f.anchor === undefined) addSource(entry.sources, f.source);
    else addSource((entry.anchors[f.anchor] ??= []), f.source);
  }
  const sorted = Object.fromEntries(
    Object.keys(next)
      .sort()
      .map((p) => [
        p,
        {
          sources: next[p].sources,
          anchors: Object.fromEntries(
            Object.keys(next[p].anchors)
              .sort()
              .map((a) => [a, next[p].anchors[a]])
          ),
        },
      ])
  );
  fs.writeFileSync(INVENTORY_FILE, JSON.stringify(sorted, null, 2) + "\n");
  Object.assign(inventory, sorted);
}

const found = [...currentSite(), ...scanRepo()];
if (process.env.UPDATE_LINK_INVENTORY) updateInventory(found);

// ── tests ────────────────────────────────────────────────────────────────────

describe("link inventory", () => {
  it("tells a broken link from a working one", () => {
    expect(resolvePath("/finnes-ikke")).toHaveProperty("error");
    expect(resolvePath("/nyheter/finnes-ikke")).toHaveProperty("error");
    expect(resolvePath("/praksis/guide/finnes-ikke")).toHaveProperty("error");
    expect(resolvePath("/praksis/guide/wrap-metoden")).toHaveProperty("file");
    const docs = resolvePath("/nav-pilot/docs"); // a redirect to /nav-pilot/referanse
    expect(docs).toHaveProperty("file");
    if ("file" in docs) {
      const ids = definedAnchors(docs.file);
      expect(ids.has("lokale-modeller")).toBe(true);
      expect(ids.has("finnes-ikke")).toBe(false);
    }
    const stats = resolvePath("/statistikk");
    if ("file" in stats) expect(definedAnchors(stats.file).has("modellkostnad-historikk")).toBe(true); // in a component
    const ordliste = resolvePath("/ordliste");
    if ("file" in ordliste) expect(definedAnchors(ordliste.file, true).has("ordbok-kategori-filter")).toBe(false); // a form field
    const abonnement = resolvePath("/abonnement");
    if ("file" in abonnement) {
      expect(definedAnchors(abonnement.file).has("din-plassering-i-nav")).toBe(true); // slug from LinkableHeading text
    }
  });

  it("has a slug check for every dynamic route", () => {
    const dynamic = routes.filter((r) => r.pattern.includes("[")).map((r) => r.pattern);
    expect(dynamic.filter((p) => !DYNAMIC[p])).toEqual([]);
  });

  it("only has redirects it can follow", () => {
    // resolvePath matches redirect sources exactly. Teach it params before adding one.
    expect(redirects.filter((r) => r.source.includes(":")).map((r) => r.source)).toEqual([]);
  });

  it("every inventoried path resolves", () => {
    const broken = Object.keys(inventory)
      .map((p) => ({ p, r: resolvePath(p) }))
      .filter(({ r }) => "error" in r)
      .map(({ p, r }) => `${p}: ${"error" in r ? r.error : ""} (linked from ${inventory[p].sources.join(", ")})`);
    expect(broken).toEqual([]);
  });

  // Wonderwall answers a path missing from autoLoginIgnorePaths with a login
  // before Next.js sees it, so an anonymous reader never gets the page or the
  // redirect (#1058). There is no exemption for private pages: they are listed
  // too, and src/proxy.ts sends readers without a token to the login.
  it("every inventoried path is let through by Wonderwall", () => {
    const blocked = Object.keys(inventory).filter((p) => !autoLoginIgnorePaths.some((a) => matches(p, a)));
    expect(autoLoginIgnorePaths).not.toContain("/**");
    expect(blocked, "Add them to autoLoginIgnorePaths in apps/my-copilot/.nais/app.yaml").toEqual([]);
  });

  it("every inventoried anchor exists on the page it lands on", () => {
    const anchorsByFile = new Map<string, Map<string, string>>();
    const anchorsOn = (file: string) => {
      if (!anchorsByFile.has(file)) anchorsByFile.set(file, definedAnchors(file));
      return anchorsByFile.get(file)!;
    };
    const broken: string[] = [];
    for (const [p, entry] of Object.entries(inventory)) {
      for (const [anchor, sources] of Object.entries(entry.anchors)) {
        const [targetPath, targetAnchor] = (LEGACY_ANCHORS[`${landingPath(p)}#${anchor}`] ?? `${p}#${anchor}`).split(
          "#"
        );
        const target = resolvePath(targetPath);
        if ("error" in target || !anchorsOn(target.file).has(targetAnchor)) {
          broken.push(
            `${p}#${anchor} → ${targetPath}#${targetAnchor}: anchor not found; add a redirect in next.config.ts or an entry in src/lib/legacy-anchors.ts (linked from ${sources.join(", ")})`
          );
        }
      }
    }
    expect(broken).toEqual([]);
  });

  it("every legacy anchor is in the inventory", () => {
    const unknown = Object.keys(LEGACY_ANCHORS).filter((k) => {
      const [p, a] = k.split("#");
      return !Object.entries(inventory).some(([ip, e]) => landingPath(ip) === p && a in e.anchors);
    });
    expect(
      unknown,
      "Key each entry on the path the browser lands on after next.config.ts redirects, with an anchor the inventory lists"
    ).toEqual([]);
  });

  // HashAnchorScroll redirects a legacy anchor before its element can appear,
  // so a key must never be a real id on its page.
  it("no legacy anchor is a real id on its page", () => {
    const clashes = Object.keys(LEGACY_ANCHORS).filter((k) => {
      const [p, a] = k.split("#");
      const target = resolvePath(p);
      return "file" in target && definedAnchors(target.file).has(a);
    });
    expect(clashes).toEqual([]);
  });

  it("every route, anchor and site link in the repo is in the inventory", () => {
    const missing = [...new Set(missingFromInventory(found).map((f) => `${key(f)} (${f.source})`))];
    expect(missing, "Run `pnpm link-inventory:update` and commit src/lib/link-inventory.json").toEqual([]);
  });

  // min-copilot.ansatt.nav.no only answers on naisdevice, so a link to it is
  // dead for everyone else. Text that names the domain without linking to it,
  // as older news articles do, is fine.
  it("links use ki-utvikling.nav.no, not the old domain", () => {
    const old = [...new Set(SCAN.absolute.flatMap(glob))]
      .filter((file) => fs.readFileSync(file, "utf-8").includes("//min-copilot.ansatt.nav.no"))
      .map(rel);
    expect(old, "Replace min-copilot.ansatt.nav.no with ki-utvikling.nav.no").toEqual([]);
  });

  // /verktoy?item=<id> opens one customization. An unknown id shows the
  // catalog with nothing open, so the link looks fine and is not.
  it("every /verktoy?item= link names an item that exists", () => {
    const items = [
      ...manifest.items.map((i) => ({ type: i.type, id: i.id })),
      ...mcpAllowlist.servers.map((s) => ({ type: "mcp", id: `mcp-${s.name}` })), // as in src/lib/mcp-registry.ts
    ];
    // "<type>:<id>" or, from older links, a bare id (#1036).
    const unknown = found
      .filter((f) => f.path === "/verktoy" && f.item !== undefined && !findByItemKey(items, f.item))
      .map((f) => `${f.item} (${f.source})`);
    expect([...new Set(unknown)]).toEqual([]);
  });

  // A news article renders at /nyheter/<slug>, so a repo-relative link like
  // ../../README.md resolves against that URL in the browser and breaks.
  it("news articles have no relative links", () => {
    const relative = glob(SCAN.markdown).flatMap((file) =>
      [...fs.readFileSync(file, "utf-8").matchAll(/\]\((\.\.?\/[^)\s]*)\)/g)].map((m) => `${m[1]} (${rel(file)})`)
    );
    expect(relative, "Link to the site page (/…) or a full GitHub URL instead").toEqual([]);
  });
});
