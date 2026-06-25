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
      `git --no-pager log --format="%H|%s|%ai|%aN" -50 -- 'skills/' '.github/agents/' '.github/instructions/' '.github/prompts/'`,
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
        `git diff-tree --no-commit-id --name-only -r ${hash} -- 'skills/' '.github/agents/' '.github/instructions/' '.github/prompts/'`,
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
        if (file === item.filePath) return true;
        if (item.type === "skill" && file.startsWith(path.dirname(item.filePath) + "/")) return true;
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
