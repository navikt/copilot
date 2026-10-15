import { redirect } from "next/navigation";

/**
 * Supported VS Code installation types and their corresponding paths.
 */
export const INSTALL_TYPES = {
  agent: "chat-agent",
  instructions: "chat-instructions",
  prompt: "chat-prompt",
} as const;

export type InstallType = keyof typeof INSTALL_TYPES;

/**
 * Validates if the URL is an allowed VS Code installation URL.
 *
 * Security: Only allows redirects to vscode: URLs that:
 * 1. Use vscode: or vscode-insiders: scheme
 * 2. Have the correct installation path for the type
 * 3. Install from raw.githubusercontent.com/navikt/ (our GitHub org)
 * 4. Do not contain path traversal sequences
 *
 * @param url - The URL to validate (already URL-decoded by Next.js)
 * @param type - The installation type (agent, instructions, prompt)
 * @returns true if the URL is allowed, false otherwise
 */
export function isAllowedInstallUrl(url: string, type: InstallType): boolean {
  const chatPath = INSTALL_TYPES[type];
  const prefixes = [`vscode:${chatPath}/install?`, `vscode-insiders:${chatPath}/install?`];
  const prefix = prefixes.find((p) => url.startsWith(p));
  if (!prefix) {
    return false;
  }

  const query = url.slice(prefix.length);
  const params = new URLSearchParams(query);
  const urls = params.getAll("url");
  if (urls.length !== 1 || Array.from(params.keys()).length !== 1) {
    return false;
  }

  const innerUrl = urls[0];
  const forbidden = /\.\.|%2e|%2f|%5c|\\/i;
  if (forbidden.test(query) || forbidden.test(innerUrl)) {
    return false;
  }

  let parsed: URL;
  try {
    parsed = new URL(innerUrl);
  } catch {
    return false;
  }

  const segments = parsed.pathname.split("/").filter(Boolean);
  return (
    parsed.protocol === "https:" &&
    parsed.hostname === "raw.githubusercontent.com" &&
    parsed.port === "" &&
    parsed.username === "" &&
    parsed.password === "" &&
    parsed.search === "" &&
    parsed.hash === "" &&
    parsed.pathname.startsWith("/navikt/") &&
    !parsed.pathname.includes("//") &&
    segments[0] === "navikt" &&
    segments.length >= 3
  );
}

/**
 * Handles the redirect for VS Code installation URLs.
 *
 * @param url - The URL parameter from the request (may be null)
 * @param type - The installation type
 * @returns Response with error, or redirects (never returns normally)
 */
export function handleInstallRedirect(url: string | null, type: InstallType): Response {
  if (!url) {
    return new Response("Missing 'url' parameter", { status: 400 });
  }

  if (!isAllowedInstallUrl(url, type)) {
    return new Response(`Invalid URL. Only navikt GitHub ${type} installations are allowed.`, { status: 400 });
  }

  // Redirect to the vscode: protocol URL
  // This will throw NEXT_REDIRECT, so it never returns
  redirect(url);
}
