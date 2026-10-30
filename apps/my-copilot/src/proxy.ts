import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { introspectToken, parseBearerToken } from "@/lib/introspect";
import {
  PRIVATE_API_PATHS,
  PRIVATE_PAGE_PATHS,
  PUBLIC_API_PATHS,
  isPrivateApiPath,
  isPrivatePath,
} from "@/lib/private-paths";

export { PRIVATE_PAGE_PATHS, PRIVATE_API_PATHS, PUBLIC_API_PATHS, isPrivatePath };

export async function proxy(request: NextRequest) {
  // In development without Texas configured, skip auth (mock user mode)
  if (process.env.NODE_ENV === "development" && !process.env.NAIS_TOKEN_INTROSPECTION_ENDPOINT) {
    return NextResponse.next();
  }

  // Next routes /%61pi/usage to the /api/usage handler, but nextUrl.pathname
  // keeps the escape. Match on the decoded path, as the router does.
  let pathname = request.nextUrl.pathname;
  try {
    pathname = decodeURIComponent(pathname);
  } catch {
    // Malformed escape: Next has no route for it either; the raw path decides.
  }

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
    "/innsikt/forbruk/:path*",
    "/innsikt/bruk/:path*",
    "/innsikt/tilpasninger/:path*",
    "/innsikt/nav-pilot/:path*",
    "/innsikt/trender/:path*",
    "/innsikt/undersokelser/:path*",
    "/admin/:path*",
    "/adopsjon/:path*",
    "/kostnad/:path*",
    "/abonnement/:path*",
    "/nav-pilot/undersokelse/:path*",
    "/api/:path*",
  ],
};
