import { execSync } from "child_process";
import { existsSync } from "fs";
import path from "path";
import { getAllCustomizations } from "./customizations";
import type { AnyCustomization } from "./customization-types";

export interface RecentUpdate {
  item: AnyCustomization;
  commitMessage: string;
  date: string;
  author: string;
}

function findRepoRoot(): string {
  const cwd = process.cwd();
  const local = path.join(cwd, ".github");
  const monorepo = path.join(cwd, "..", "..", ".github");

  if (existsSync(local)) return cwd;
  if (existsSync(monorepo)) return path.resolve(cwd, "..", "..");
  return cwd;
}

export function getRecentlyUpdatedCustomizations(limit = 5): RecentUpdate[] {
  const repoRoot = findRepoRoot();
  const allItems = getAllCustomizations();

  let logOutput: string;
  try {
    logOutput = execSync(
      // Repo paths here too, not only in the diff-tree below (#718). The
      // .github/ trees are where these artifacts lived before #330 moved them
      // to the root in June, so this pathspec matched 80 commits where the
      // current one matches 37: the difference is pre-move history that can
      // never match an item's repoPath, filling the 50-commit window with
      // commits that yield nothing and crowding out the ones that do.
      `git --no-pager log --format="%H|%s|%ai|%aN" -50 -- 'skills/' 'agents/' 'instructions/' 'prompts/'`,
      { cwd: repoRoot, encoding: "utf-8", timeout: 5000 }
    );
  } catch {
    return [];
  }

  const lines = logOutput.trim().split("\n").filter(Boolean);
  const seen = new Set<string>();
  const results: RecentUpdate[] = [];

  for (const line of lines) {
    if (results.length >= limit) break;

    const [hash, message, date, author] = line.split("|");
    if (!hash || !message) continue;

    let changedFiles: string[];
    try {
      changedFiles = execSync(
        // Repo paths, not install paths. These trees moved to the root in #330
        // and the pathspec kept the .github/ prefix, so this matched nothing
        // for agents, instructions and prompts: an agent change has never
        // appeared in recent updates (#718). Skills worked only because their
        // path had no prefix to be wrong about.
        `git diff-tree --no-commit-id --name-only -r ${hash} -- 'skills/' 'agents/' 'instructions/' 'prompts/'`,
        {
          cwd: repoRoot,
          encoding: "utf-8",
          timeout: 3000,
        }
      )
        .trim()
        .split("\n")
        .filter(Boolean);
    } catch {
      continue;
    }

    for (const file of changedFiles) {
      const matched = allItems.find((item) => {
        if (!item.repoPath) return false;
        if (file === item.repoPath) return true;
        if (item.type === "skill" && file.startsWith(path.dirname(item.repoPath) + "/")) return true;
        return false;
      });

      if (matched && !seen.has(matched.id)) {
        seen.add(matched.id);
        results.push({
          item: matched,
          commitMessage: cleanCommitMessage(message),
          date: date.split(" ")[0],
          author,
        });
        if (results.length >= limit) break;
      }
    }
  }

  return results;
}

function cleanCommitMessage(msg: string): string {
  return msg
    .replace(/^(feat|fix|docs|refactor|style|chore|test)\([^)]*\):\s*/, "")
    .replace(/^(feat|fix|docs|refactor|style|chore|test):\s*/, "")
    .replace(/\s*\(#\d+\)\s*$/, "")
    .trim();
}
