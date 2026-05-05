import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

const PRIVATE_PAGE_PATHS = ["/statistikk", "/adopsjon", "/kostnad", "/abonnement", "/kalkulator"];

const PRIVATE_API_PATHS = ["/api/copilot", "/api/adoption"];

export function isPrivatePath(pathname: string): boolean {
  return (
    PRIVATE_PAGE_PATHS.some((p) => pathname === p || pathname.startsWith(p + "/")) ||
    PRIVATE_API_PATHS.some((p) => pathname === p || pathname.startsWith(p + "/"))
  );
}

function isPrivateApiPath(pathname: string): boolean {
  return PRIVATE_API_PATHS.some((p) => pathname === p || pathname.startsWith(p + "/"));
}

export function middleware(request: NextRequest) {
  const pathname = request.nextUrl.pathname;

  if (!isPrivatePath(pathname)) {
    return NextResponse.next();
  }

  // If user is authenticated (Wonderwall sets Authorization header), pass through
  const hasAuth = request.headers.get("Authorization");
  if (hasAuth) {
    return NextResponse.next();
  }

  // Private API routes without auth: 401
  if (isPrivateApiPath(pathname)) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  // Private page routes without auth: redirect to Wonderwall login
  const loginUrl = new URL("/oauth2/login", request.url);
  loginUrl.searchParams.set("redirect", pathname + request.nextUrl.search);
  return NextResponse.redirect(loginUrl);
}

export const config = {
  matcher: [
    "/statistikk/:path*",
    "/adopsjon/:path*",
    "/kostnad/:path*",
    "/abonnement/:path*",
    "/kalkulator/:path*",
    "/api/copilot/:path*",
    "/api/adoption/:path*",
  ],
};
