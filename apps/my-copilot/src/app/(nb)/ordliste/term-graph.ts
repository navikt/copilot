import type { Term } from "./terms";

export interface TermEdge {
  from: number;
  to: number;
  source: "text" | "related";
}

export function termId(term: string): string {
  return (
    "begrep-" +
    term
      .toLowerCase()
      .replace(/æ/g, "ae")
      .replace(/ø/g, "o")
      .replace(/å/g, "a")
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-|-$/g, "")
  );
}

// "MCP (Model Context Protocol)" is mentioned as "MCP"; "Next Edit Suggestions (NES)" also as "NES".
// A one-word base with a parenthesis ("Allowlist (MCP)") keeps only the base: "MCP" is another term.
function aliases(term: string): string[] {
  const m = term.match(/^(.+?)\s*\((.+)\)$/);
  if (!m) return [term];
  const [, base, paren] = m;
  return /^[A-Z]+$/.test(paren) && base.includes(" ") ? [term, base, paren] : [term, base];
}

const escape = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

// A → B when A's definition mentions B. Longest names match first and are blanked out,
// so "Agent mode" in a definition does not also count as "Agent".
export function deriveEdges(terms: Term[]): TermEdge[] {
  const patterns = terms
    .flatMap((t, i) => aliases(t.term).map((name) => ({ i, name })))
    .sort((a, b) => b.name.length - a.name.length)
    .map(({ i, name }) => ({
      i,
      // Bokmål endings; a term ending in -e takes -r, -n, -ne ("agentpakke" → "agentpakker", "agentpakkene").
      re: new RegExp(
        `(?<![\\p{L}\\d])${escape(name)}(?:${/e$/i.test(name) ? "r|n|ne|ns|" : ""}en|et|er|ene|ens|s)?(?![\\p{L}\\d])`,
        "giu"
      ),
    }));
  const index = new Map(terms.map((t, i) => [t.term, i]));
  const seen = new Set<string>();
  const edges: TermEdge[] = [];
  const add = (from: number, to: number | undefined, source: TermEdge["source"]) => {
    const key = `${from}-${to}`;
    if (to === undefined || from === to || seen.has(key)) return;
    seen.add(key);
    edges.push({ from, to, source });
  };
  terms.forEach((t, from) => {
    let text = t.definition;
    for (const { i, re } of patterns) {
      text = text.replace(re, (hit) => {
        add(from, i, "text");
        return " ".repeat(hit.length);
      });
    }
  });
  terms.forEach((t, from) => t.related?.forEach((name) => add(from, index.get(name), "related")));
  return edges;
}
