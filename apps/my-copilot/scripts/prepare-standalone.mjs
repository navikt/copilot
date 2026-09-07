// The articles live in docs/news/articles at the repo root, outside the app,
// so Next's file tracing cannot reach them: Turbopack rejects a tracing glob
// that navigates above the project root. The container image sidesteps this by
// copying the directory in (Dockerfile line 15), which is why production works
// and `pnpm start` did not.
//
// This mirrors that copy for a local run. It is a no-op when the source is
// missing, so it stays quiet in an image where the copy already happened.
import { cpSync, existsSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const appDir = dirname(dirname(fileURLToPath(import.meta.url)));
const source = join(appDir, "..", "..", "docs", "news", "articles");
const target = join(appDir, ".next", "standalone", "docs", "news", "articles");

if (!existsSync(source)) process.exit(0);
if (!existsSync(join(appDir, ".next", "standalone"))) process.exit(0);

cpSync(source, target, { recursive: true });
console.log(`copied articles into the standalone output: ${target}`);
