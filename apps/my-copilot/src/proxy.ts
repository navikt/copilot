import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { isAuthenticated } from "./lib/auth";
import { recordPageView } from "./lib/metrics";

// Named export for Next.js 16 proxy convention
export async function proxy(request: NextRequest) {
  const isAuth = await isAuthenticated();
  if (!isAuth) {
    return NextResponse.redirect(new URL("/oauth2/login", request.url));
  }
  recordPageView(request.nextUrl.pathname);
}

// Also export as default for backward compatibility
export default proxy;

export const config = {
  matcher: [
    /*
     * Match all request paths except for the ones starting with:
     * - _next/static (static files)
     * - _next/image (image optimization files)
     * - favicon.ico, sitemap.xml, robots.txt (metadata files)
     * - /health (health check endpoint)
     * - /metrics (metrics endpoint)
     */
    "/((?!_next/static|_next/image|favicon.ico|sitemap.xml|robots.txt|health|metrics).*)",
  ],
};
