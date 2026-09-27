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
// which matches everything below /x but not /x itself.
export function matches(route, pattern) {
  if (pattern === route) return true;
  if (pattern.endsWith("/**")) return route.startsWith(pattern.slice(0, -2));
  return false;
}
