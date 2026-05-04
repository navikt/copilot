import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

const PRIVATE_PATHS = ["/statistikk", "/adopsjon", "/kostnad", "/abonnement", "/kalkulator"];

const PRIVATE_API_PATHS = ["/api/copilot", "/api/adoption"];

const INTERNAL_HOST = process.env.INTERNAL_HOST ?? "min-copilot.ansatt.nav.no";

export function isPrivatePath(pathname: string): boolean {
  return (
    PRIVATE_PATHS.some((p) => pathname === p || pathname.startsWith(p + "/")) ||
    PRIVATE_API_PATHS.some((p) => pathname === p || pathname.startsWith(p + "/"))
  );
}

function isInternalHost(host: string): boolean {
  const normalized = host.split(":")[0];
  return normalized === INTERNAL_HOST;
}

export function middleware(request: NextRequest) {
  const host = request.headers.get("host") ?? "";
  const pathname = request.nextUrl.pathname;

  // On public domain, redirect private routes to internal domain
  if (!isInternalHost(host) && isPrivatePath(pathname)) {
    const url = new URL(pathname, `https://${INTERNAL_HOST}`);
    url.search = request.nextUrl.search;
    return NextResponse.redirect(url);
  }

  return NextResponse.next();
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
