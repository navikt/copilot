// Every public page route must be listed in autoLoginIgnorePaths, or Wonderwall
// intercepts it and anonymous readers get a 401. Nothing else catches that: the
// build passes, the deploy succeeds, and the page is live and unreachable. It
// happened to /en/news in #682 and was only noticed by loading the site.
//
// The private routes below are deliberate. Add a route there when it should
// require a login, not to silence this check.
import { globSync } from "node:fs";
import { dirname, join, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { autoLoginIgnorePaths as allowed, matches } from "./auto-login-ignore-paths.mjs";

const appDir = dirname(dirname(fileURLToPath(import.meta.url)));
const PRIVATE_ROUTES = ["/abonnement", "/kostnad", "/overview", "/usage"];

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
