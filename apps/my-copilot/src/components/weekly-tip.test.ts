import { readFileSync } from "node:fs";
import { join } from "node:path";

// WeeklyTip picks its tip from the clock. As a client component it ran again in
// the browser, picked another tip whenever the browser's week differed from the
// server's, and the front page failed hydration (React #418).
describe("WeeklyTip", () => {
  it("stays a server component", () => {
    const source = readFileSync(join(__dirname, "weekly-tip.tsx"), "utf8");
    expect(source).not.toMatch(/^\s*["']use client["']/m);
  });
});
