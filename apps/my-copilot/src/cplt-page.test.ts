import fs from "node:fs";
import path from "node:path";

// /cplt is the external landing page for the cplt open-source project. It lives
// under the English (en) route group, with no nav-pilot section menu, and stays
// in English (see AGENTS.md).
const PAGE = path.resolve(__dirname, "app/(en)/cplt/page.tsx");
// The WSL2 guide it links to for Windows.
const WINDOWS = path.resolve(__dirname, "app/(en)/cplt/windows/page.tsx");
// The configuration explorer renders on the page too.
const EXPLORER = path.resolve(__dirname, "components/cplt-config-explorer.tsx");

describe("/cplt", () => {
  it.each([PAGE, WINDOWS])('keeps lang="en" on <main> and never lang="nb" in %s', (page) => {
    const src = fs.readFileSync(page, "utf-8");
    expect(src).toMatch(/<main[^>]*\blang="en"/);
    expect(src).not.toMatch(/<main[^>]*\blang="nb"/);
  });

  // Aksel's CopyButton defaults to Norwegian («Kopier», «Kopiert!»); on this
  // English page every one names its labels.
  it("gives every copy button English labels", () => {
    const src = fs.readFileSync(PAGE, "utf-8") + fs.readFileSync(EXPLORER, "utf-8");
    const buttons = src.match(/<CopyButton\b[^>]*>/g) ?? [];
    expect(buttons.length).toBeGreaterThan(0);
    for (const b of buttons) {
      expect(b).toMatch(/title="Copy"/);
      expect(b).toMatch(/activeText="Copied!"/);
    }
  });
});
