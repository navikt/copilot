// The site search (V11 in docs/nav-pilot-dokumentasjon-forslag.md): plain
// text matching in the browser over public/search-index.json, which
// scripts/build-search-index.ts writes before `next dev` and `next build`.

/** One hit: a page, a heading on a page, or a news article. */
export type SearchEntry = { href: string; title: string; context: string };

export const SEARCH_INDEX_URL = "/search-index.json";

/** Lower case without diacritics, so «malte» finds «Målte». Æ and ø have none and stay. */
export const fold = (s: string) =>
  s
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase();

/**
 * Entries where every word in the query is in the title or the context.
 * The title starting with the query ranks first, then the title containing it,
 * then the title containing every word. Ties keep the index order: pages,
 * headings, then news with the newest first.
 */
export function searchEntries(entries: SearchEntry[], query: string): SearchEntry[] {
  const q = fold(query).trim().replace(/\s+/g, " ");
  if (!q) return [];
  const words = q.split(" ");
  const rank = (e: SearchEntry) => {
    const title = fold(e.title);
    if (!words.every((w) => title.includes(w) || fold(e.context).includes(w))) return -1;
    if (title.startsWith(q)) return 0;
    if (title.includes(q)) return 1;
    return words.every((w) => title.includes(w)) ? 2 : 3;
  };
  return entries
    .map((e) => ({ e, r: rank(e) }))
    .filter((x) => x.r >= 0)
    .sort((a, b) => a.r - b.r)
    .map((x) => x.e);
}
