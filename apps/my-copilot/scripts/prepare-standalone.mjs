// The articles live in docs/news/articles at the repo root, outside the app,
// so Next's file tracing cannot reach them: Turbopack rejects a tracing glob
// that navigates above the project root. The image sidesteps this: the COPY in
// apps/my-copilot/Dockerfile brings the directory in, which is why production
// works and `pnpm start` did not.
//
// This mirrors that copy for a local run. It is a no-op when the source is
// missing, so it stays quiet in an image where the copy already happened.
import { cpSync, existsSync, rmSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const appDir = dirname(dirname(fileURLToPath(import.meta.url)));
const source = join(appDir, "..", "..", "docs", "news", "articles");
const target = join(appDir, ".next", "standalone", "docs", "news", "articles");

if (!existsSync(source)) process.exit(0);
if (!existsSync(join(appDir, ".next", "standalone"))) process.exit(0);

// A plain copy leaves files that were deleted from the source, so an article
// removed upstream would keep serving locally. Clear the target first.
rmSync(target, { recursive: true, force: true });
cpSync(source, target, { recursive: true });
console.log(`copied articles into the standalone output: ${target}`);
