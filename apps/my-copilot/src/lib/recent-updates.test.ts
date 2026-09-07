import { describe, it, expect } from "vitest";
import manifest from "./copilot-manifest.json";
import { readFileSync } from "node:fs";
import path from "node:path";

// #718: filePath is the install path (".github/agents/…"), and it was also used
// to match paths out of `git diff-tree`, which names files where they live in
// this repo ("agents/…"). Nothing matched, so a change to an agent, instruction
// or prompt has never appeared in recent updates. Skills worked by accident:
// their path has no prefix to be wrong about.
describe("manifest paths", () => {
  const items = manifest.items as Array<{
    id: string;
    type: string;
    filePath: string;
    repoPath: string;
  }>;

  it("gives every artifact a repoPath without the install prefix", () => {
    const withFile = items.filter((i) => i.filePath !== "");
    expect(withFile.length).toBeGreaterThan(0);
    for (const item of withFile) {
      expect(item.repoPath, `${item.id} has no repoPath`).toBeTruthy();
      expect(item.repoPath.startsWith(".github/"), `${item.id} repoPath carries the install prefix`).toBe(false);
    }
  });

  it("keeps filePath as the install path for the kinds that have one", () => {
    // The two fields are different on purpose. If they ever become identical
    // for agents, someone has collapsed them and the install links break.
    const agent = items.find((i) => i.type === "agent");
    expect(agent).toBeDefined();
    expect(agent!.filePath).toMatch(/^\.github\/agents\//);
    expect(agent!.repoPath).toMatch(/^agents\//);
  });

  it("leaves repoPath empty for items with no file in this repo", () => {
    for (const item of items.filter((i) => i.filePath === "")) {
      expect(item.repoPath, `${item.id} invented a repoPath`).toBe("");
    }
  });
});

// The two pathspecs have to agree. Fixing only the diff-tree one left the log
// selecting candidate commits by the old .github/ paths, so a commit touching
// only agents/ was never considered and the corrected diff-tree never ran on it
// (#718, caught in review).
describe("recent-updates pathspecs", () => {
  const src = readFileSync(path.join(process.cwd(), "src/lib/recent-updates.ts"), "utf-8");

  it("selects and inspects commits by the same repo paths", () => {
    const specs = [...src.matchAll(/-- ((?:'[^']+' ?)+)/g)].map((m) => m[1].trim());
    expect(specs.length, "expected both the log and the diff-tree pathspec").toBe(2);
    expect(specs[0]).toBe(specs[1]);
  });

  it("uses no install-path prefix in either", () => {
    for (const spec of [...src.matchAll(/-- ((?:'[^']+' ?)+)/g)].map((m) => m[1])) {
      expect(spec, "a pathspec still names the pre-#330 .github/ trees").not.toContain(".github/");
    }
  });
});
