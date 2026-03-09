import { redirect } from "next/navigation";
import { NextRequest } from "next/server";

// Allowed VS Code agent installation URL patterns
// Only allows vscode: or vscode-insiders: scheme with chat-agent/install path
// and a url parameter pointing to raw.githubusercontent.com/navikt/
// Note: Next.js automatically URL-decodes query params, so we match decoded URLs
const ALLOWED_PATTERNS = [
  /^vscode:chat-agent\/install\?url=https:\/\/raw\.githubusercontent\.com\/navikt\//,
  /^vscode-insiders:chat-agent\/install\?url=https:\/\/raw\.githubusercontent\.com\/navikt\//,
];

/**
 * Redirect handler for VS Code agent installation.
 *
 * GitHub's image caching via camo.githubusercontent.com breaks direct vscode: protocol links
 * in markdown badges. This route acts as an HTTPS intermediary that redirects to the
 * vscode: protocol URL.
 *
 * Security: Only allows redirects to vscode: URLs that install from navikt GitHub repos.
 *
 * Usage:
 *   https://min-copilot.ansatt.nav.no/install/agent?url=vscode:chat-agent/install?url=...
 *
 * The `url` parameter should be the complete vscode: protocol URL (URL-encoded).
 *
 * @see https://github.com/navikt/copilot/issues/67
 */
export async function GET(request: NextRequest) {
  const url = request.nextUrl.searchParams.get("url");

  if (!url) {
    return new Response("Missing 'url' parameter", { status: 400 });
  }

  // Validate against allowed patterns (prevents open redirect attacks)
  const isAllowed = ALLOWED_PATTERNS.some((pattern) => pattern.test(url));
  if (!isAllowed) {
    return new Response("Invalid URL. Only navikt GitHub agent installations are allowed.", { status: 400 });
  }

  // Redirect to the vscode: protocol URL
  redirect(url);
}
