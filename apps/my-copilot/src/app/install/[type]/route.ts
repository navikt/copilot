import { NextRequest } from "next/server";

import { handleInstallRedirect, INSTALL_TYPES, InstallType } from "@/lib/install-redirect";

/**
 * Redirect handler for VS Code installations (agents, instructions, prompts).
 *
 * GitHub's image caching via camo.githubusercontent.com breaks direct vscode: protocol links
 * in markdown badges. This route acts as an HTTPS intermediary that redirects to the
 * vscode: protocol URL.
 *
 * Security: Only allows redirects to vscode: URLs that install from navikt GitHub repos.
 *
 * Usage:
 *   https://ki-utvikling.nav.no/install/agent?url=vscode:chat-agent/install?url=...
 *   https://ki-utvikling.nav.no/install/instructions?url=vscode:chat-instructions/install?url=...
 *   https://ki-utvikling.nav.no/install/prompt?url=vscode:chat-prompt/install?url=...
 *
 * The `url` parameter should be the complete vscode: protocol URL (URL-encoded).
 *
 * @see https://github.com/navikt/copilot/issues/67
 */
export async function GET(request: NextRequest, { params }: { params: Promise<{ type: string }> }) {
  const { type } = await params;

  // Validate the type parameter
  if (!Object.hasOwn(INSTALL_TYPES, type)) {
    const validTypes = Object.keys(INSTALL_TYPES).join(", ");
    return new Response(`Invalid install type '${type}'. Valid types: ${validTypes}`, { status: 400 });
  }

  const url = request.nextUrl.searchParams.get("url");
  return handleInstallRedirect(url, type as InstallType);
}
