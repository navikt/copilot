// Every public page route must be listed in autoLoginIgnorePaths, or Wonderwall
// intercepts it and anonymous readers get a 401. Nothing else catches that: the
// build passes, the deploy succeeds, and the page is live and unreachable. It
// happened to /en/news in #682 and was only noticed by loading the site.
//
// PRIVATE_ROUTES in auto-login-ignore-paths.mjs is deliberate. Add a route there
// when it should require a login, not to silence this check. Each one must be
// guarded by src/proxy.ts, both its path lists and config.matcher.
import { globSync, readFileSync } from "node:fs";
import { dirname, join, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { autoLoginIgnorePaths as allowed, matches, PRIVATE_ROUTES } from "./auto-login-ignore-paths.mjs";

const appDir = dirname(dirname(fileURLToPath(import.meta.url)));

const proxy = readFileSync(join(appDir, "src", "proxy.ts"), "utf-8");
const listed = (name) => {
  const body = proxy.match(new RegExp(`${name}\\W*\\[([^\\]]*)\\]`))?.[1] ?? "";
  return [...body.matchAll(/"([^"]+)"/g)].map((m) => m[1].replace("/:path*", ""));
};
const covers = (prefixes, route) => prefixes.some((p) => route === p || route.startsWith(p + "/"));
const guarded = [...listed("PRIVATE_PAGE_PATHS"), ...listed("PRIVATE_API_PATHS")];
const matched = listed("matcher");
const unguarded = PRIVATE_ROUTES.filter((r) => !covers(guarded, r) || !covers(matched, r));
if (unguarded.length > 0) {
  console.error("These private routes are not guarded by src/proxy.ts:");
  for (const route of unguarded) console.error(`  ${route}`);
  console.error("\nAdd them to the path lists and config.matcher in src/proxy.ts.");
  process.exit(1);
}

function routeFor(pageFile) {
  const rel = relative(join(appDir, "src", "app"), dirname(pageFile));
  const segments = rel.split(sep).filter((s) => s && !(s.startsWith("(") && s.endsWith(")")));
  return "/" + segments.join("/");
}

const pages = globSync("src/app/**/page.tsx", { cwd: appDir })
  .map((p) => join(appDir, p))
  .filter((p) => !p.includes("[")); // dynamic segments are covered by a /** entry

const missing = pages
  .map(routeFor)
  .filter((route) => !PRIVATE_ROUTES.includes(route))
  .filter((route) => !allowed.some((pattern) => matches(route, pattern)));

if (missing.length > 0) {
  console.error("These routes are public pages but are not in autoLoginIgnorePaths:");
  for (const route of [...new Set(missing)].sort()) console.error(`  ${route}`);
  console.error("\nAdd them to apps/my-copilot/.nais/app.yaml, or to PRIVATE_ROUTES in this");
  console.error("script if they are meant to require a login.");
  process.exit(1);
}

console.log(`✅ all ${pages.length} page routes are reachable without a login, or listed as private`);
