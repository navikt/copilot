import { getToken, validateToken } from "@navikt/oasis";

/**
 * Validerer Azure-tokenet til saksbehandleren. Returnerer et 401-svar som route
 * handleren skal sende videre, eller null når tokenet er gyldig.
 */
export async function requireSaksbehandler(request: Request): Promise<Response | null> {
  const token = getToken(request);
  if (!token) {
    return Response.json({ error: "Mangler token" }, { status: 401 });
  }
  const validation = await validateToken(token);
  if (!validation.ok) {
    return Response.json({ error: "Ugyldig token" }, { status: 401 });
  }
  return null;
}
