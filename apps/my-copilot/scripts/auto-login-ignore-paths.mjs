// The paths Wonderwall lets through without a login, read from .nais/app.yaml.
// Shared by check-public-routes.mjs and src/link-inventory.test.ts.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const appDir = dirname(dirname(fileURLToPath(import.meta.url)));
const yaml = readFileSync(join(appDir, ".nais", "app.yaml"), "utf-8");
const block = yaml.match(/autoLoginIgnorePaths:\n((?:\s*(?:#.*|- .*)\n)+)/);
if (!block) throw new Error("could not find autoLoginIgnorePaths in .nais/app.yaml");

export const autoLoginIgnorePaths = [...block[1].matchAll(/^\s*- (\S+)/gm)].map((m) => m[1]);

// Wonderwall semantics for the two forms used here: an exact path, or /x/**,
// which matches /x and everything below it.
export function matches(route, pattern) {
  if (pattern === route) return true;
  if (!pattern.endsWith("/**")) return false;
  const base = pattern.slice(0, -3);
  return route === base || route.startsWith(base + "/");
}

// Routes that must require a login. Wonderwall lets them through too (see
// app.yaml), so src/proxy.ts is the only gate; check-public-routes.mjs and
// src/proxy.test.ts fail if it stops guarding one.
export const PRIVATE_ROUTES = [
  "/abonnement",
  "/kostnad",
  "/statistikk",
  "/statistikk/json",
  "/innsikt/team",
  "/adopsjon",
];
