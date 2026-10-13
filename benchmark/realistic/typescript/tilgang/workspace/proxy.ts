import { getToken } from "@navikt/oasis";
import { NextResponse, type NextRequest } from "next/server";

// Rask sjekk før route handlerne: forespørsler uten token avvises med en gang.
// Selve valideringen av tokenet gjør hver route handler med requireSaksbehandler().
export function proxy(request: NextRequest): NextResponse {
  if (!getToken(request)) {
    return NextResponse.json({ error: "Mangler token" }, { status: 401 });
  }
  return NextResponse.next();
}

export const config = {
  matcher: ["/api/saker/:path*"],
};
