import fs from "node:fs";
import path from "node:path";

// /cplt is the external landing page for the cplt open-source project. It lives
// under the (nb) route group but stays in English (see AGENTS.md).
const PAGE = path.resolve(__dirname, "app/(nb)/(nav-pilot)/cplt/page.tsx");

describe("/cplt", () => {
  it('keeps lang="en" on <main> and never lang="nb"', () => {
    const src = fs.readFileSync(PAGE, "utf-8");
    expect(src).toMatch(/<main[^>]*\blang="en"/);
    expect(src).not.toMatch(/<main[^>]*\blang="nb"/);
  });
});
