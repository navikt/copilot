import { getUser, getUserToken } from "@/lib/auth";
import { getFileContributors } from "@/lib/cached-github";
import { getAllCustomizations } from "@/lib/customizations";
import type { Skill } from "@/lib/customization-types";
import { NextResponse } from "next/server";

const OWNER = "navikt";
const REPO = "copilot";

/**
 * Resolve file paths for a customization item.
 * Skills aggregate contributors across SKILL.md and all referenced files.
 */
function resolveFilePaths(itemId: string): string[] | null {
  const items = getAllCustomizations();
  const item = items.find((i) => i.id === itemId);
  if (!item) return null;

  // repoPath, not filePath: git knows where the file lives here, not where it
  // lands when installed (#718).
  //
  // An item with no file in this repo, an MCP server, contributes no path at
  // all. An empty one would reach the backend as `commits?path=` and come back
  // with every contributor to the repository, which is a wrong answer rather
  // than an empty one.
  const paths = item.repoPath ? [item.repoPath] : [];

  if (item.type === "skill") {
    const skill = item as Skill;
    if (skill.references) {
      paths.push(...skill.references.map((ref) => ref.path));
    }
  }

  return paths;
}

export async function GET(request: Request) {
  const user = await getUser(false);
  if (!user) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const token = await getUserToken();
  if (!token) {
    return NextResponse.json({ error: "Token not available" }, { status: 401 });
  }

  const { searchParams } = new URL(request.url);
  const itemId = searchParams.get("id");

  if (!itemId) {
    return NextResponse.json({ error: "Missing id parameter" }, { status: 400 });
  }

  const paths = resolveFilePaths(itemId);
  if (!paths) {
    return NextResponse.json({ error: "Unknown item" }, { status: 404 });
  }
  // An item with no file in this repo has no contributors to report. Asking the
  // backend with an empty list is what produced the repository's whole
  // contributor list for an MCP server (#718).
  if (paths.length === 0) {
    return NextResponse.json([]);
  }

  const { contributors, error } = await getFileContributors(token, OWNER, REPO, paths);

  if (error) {
    return NextResponse.json({ error }, { status: 502 });
  }

  return NextResponse.json(contributors);
}
