// The news articles (docs/news/articles) and the golden-harness summary
// (docs/golden-baselines/summary.json) live at the repo root, outside the app,
// so Next's file tracing cannot reach them: Turbopack rejects a tracing glob
// that navigates above the project root. The image sidesteps this: the COPYs in
// apps/my-copilot/Dockerfile bring them in, which is why production works and
// `pnpm start` did not.
//
// This mirrors those copies for a local run. Each is a no-op when its source is
// missing, so it stays quiet in an image where the copy already happened.
import { cpSync, existsSync, rmSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const appDir = dirname(dirname(fileURLToPath(import.meta.url)));
if (!existsSync(join(appDir, ".next", "standalone"))) process.exit(0);

for (const rel of [join("docs", "news", "articles"), join("docs", "golden-baselines", "summary.json")]) {
  const source = join(appDir, "..", "..", rel);
  const target = join(appDir, ".next", "standalone", rel);
  // A plain copy leaves files that were deleted from the source, so an article
  // removed upstream would keep serving locally. Clear the target first.
  if (!existsSync(source)) continue;
  rmSync(target, { recursive: true, force: true });
  cpSync(source, target, { recursive: true });
  console.log(`copied ${rel} into the standalone output: ${target}`);
}
