import { deriveEdges, termId } from "./term-graph";
import { terms } from "./terms";

const names = new Set(terms.map((t) => t.term));
const edges = deriveEdges(terms);
const pairs = new Set(edges.map((e) => `${terms[e.from].term} → ${terms[e.to].term}`));

describe("term graph", () => {
  it("related entries name existing terms", () => {
    for (const t of terms) for (const r of t.related ?? []) expect(names, `${t.term} → ${r}`).toContain(r);
  });

  it("never links a term to itself", () => {
    expect(edges.every((e) => e.from !== e.to)).toBe(true);
  });

  it("derives known pairs from the definitions", () => {
    expect(pairs).toContain("Agent mode → Agent"); // «Agenten»
    expect(pairs).toContain("Kontekstvindu → Token"); // «tokens»
    expect(pairs).toContain("Tool calling → MCP (Model Context Protocol)"); // «MCP»
    expect(pairs).toContain("Next Edit Suggestions (NES) → Inline suggestion");
    expect(pairs).toContain("Copilot Edits → Agent mode");
  });

  it("lets the longer term win", () => {
    // «agent mode» in Copilot Edits is Agent mode, not Agent.
    expect(pairs).not.toContain("Copilot Edits → Agent");
  });

  it("gives every term a unique anchor", () => {
    expect(new Set(terms.map((t) => termId(t.term))).size).toBe(terms.length);
  });
});
