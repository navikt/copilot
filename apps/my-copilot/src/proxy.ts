import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { introspectToken, parseBearerToken } from "@/lib/introspect";

export const PRIVATE_PAGE_PATHS = [
  "/statistikk",
  "/innsikt/team",
  "/adopsjon",
  "/kostnad",
  "/abonnement",
  "/nav-pilot/undersokelse",
];

export const PRIVATE_API_PATHS = ["/api/copilot", "/api/adoption", "/statistikk/json"];

export function isPrivatePath(pathname: string): boolean {
  return (
    PRIVATE_PAGE_PATHS.some((p) => pathname === p || pathname.startsWith(p + "/")) ||
    PRIVATE_API_PATHS.some((p) => pathname === p || pathname.startsWith(p + "/"))
  );
}

function isPrivateApiPath(pathname: string): boolean {
  return PRIVATE_API_PATHS.some((p) => pathname === p || pathname.startsWith(p + "/"));
}

export async function proxy(request: NextRequest) {
  // In development without Texas configured, skip auth (mock user mode)
  if (process.env.NODE_ENV === "development" && !process.env.NAIS_TOKEN_INTROSPECTION_ENDPOINT) {
    return NextResponse.next();
  }

  const pathname = request.nextUrl.pathname;

  if (!isPrivatePath(pathname)) {
    return NextResponse.next();
  }

  // Wonderwall sets the Authorization header, but does not strip one the client
  // sent itself, so the token is validated with Texas rather than trusted (#1070).
  // Pages still call getUser(), which introspects again: two Texas calls per
  // private request, accepted since Texas is a local sidecar.
  const token = parseBearerToken(request.headers.get("Authorization"));
  if (token && (await introspectToken(token))) {
    return NextResponse.next();
  }

  // Private API routes without a valid token: 401
  if (isPrivateApiPath(pathname)) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  // Private page routes without a valid token: redirect to Wonderwall login
  const loginUrl = new URL("/oauth2/login", request.url);
  loginUrl.searchParams.set("redirect", pathname + request.nextUrl.search);
  return NextResponse.redirect(loginUrl);
}

export const config = {
  matcher: [
    "/statistikk/:path*",
    "/innsikt/team/:path*",
    "/adopsjon/:path*",
    "/kostnad/:path*",
    "/abonnement/:path*",
    "/nav-pilot/undersokelse/:path*",
    "/api/copilot/:path*",
    "/api/adoption/:path*",
  ],
};
