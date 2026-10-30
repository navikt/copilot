// No server imports: the Innblikk client component uses isPrivatePath too.
export const PRIVATE_PAGE_PATHS = [
  "/statistikk",
  "/innsikt/team",
  "/innsikt/forbruk",
  "/innsikt/bruk",
  "/innsikt/tilpasninger",
  "/innsikt/nav-pilot",
  "/innsikt/trender",
  "/innsikt/undersokelser",
  "/admin",
  "/adopsjon",
  "/kostnad",
  "/abonnement",
  "/nav-pilot/undersokelse",
];

// Every /api route requires a valid token, so a new route is guarded by default.
// src/proxy.test.ts fails if any route.ts under src/app is neither covered nor allowlisted.
export const PRIVATE_API_PATHS = ["/api", "/statistikk/json"];

// Public API routes: exempt from the token check. They must also be listed in
// autoLoginIgnorePaths in .nais/app.yaml, or Wonderwall blocks them. None today.
export const PUBLIC_API_PATHS: string[] = [];

const under = (paths: string[], pathname: string) => paths.some((p) => pathname === p || pathname.startsWith(p + "/"));

export function isPrivatePath(pathname: string): boolean {
  return under(PRIVATE_PAGE_PATHS, pathname) || isPrivateApiPath(pathname);
}

export function isPrivateApiPath(pathname: string): boolean {
  return (
    !under(PUBLIC_API_PATHS, pathname) && PRIVATE_API_PATHS.some((p) => pathname === p || pathname.startsWith(p + "/"))
  );
}
