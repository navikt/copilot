// Writes public/search-index.json for the site search. Runs before `next dev`
// and `next build` (package.json), so the index is a static file.
import fs from "node:fs";
import { buildSearchIndex } from "../src/lib/search-index";

const out = new URL("../public/search-index.json", import.meta.url);
const index = buildSearchIndex();
fs.writeFileSync(out, JSON.stringify(index));
console.log(`search index: ${index.length} entries, ${(fs.statSync(out).size / 1024).toFixed(1)} KB`);
