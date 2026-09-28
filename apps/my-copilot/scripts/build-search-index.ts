// Writes public/search-index.json for the site search and public/news.json
// for nav-pilot news. Runs before `next dev` and `next build` (package.json),
// so both are static files.
import fs from "node:fs";
import { buildNewsFeed } from "../src/lib/news";
import { buildSearchIndex } from "../src/lib/search-index";

const out = new URL("../public/search-index.json", import.meta.url);
const index = buildSearchIndex();
fs.writeFileSync(out, JSON.stringify(index));
console.log(`search index: ${index.length} entries, ${(fs.statSync(out).size / 1024).toFixed(1)} KB`);

const feed = buildNewsFeed();
fs.writeFileSync(new URL("../public/news.json", import.meta.url), JSON.stringify(feed));
console.log(`news feed: ${feed.items.length} items, ${feed.items.filter((i) => i.cli).length} for nav-pilot`);
