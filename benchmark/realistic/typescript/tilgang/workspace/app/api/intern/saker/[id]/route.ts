import { getToken } from "@navikt/oasis";
import { finnInternSak } from "../../../../../lib/saker.ts";

export async function GET(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
): Promise<Response> {
  if (!getToken(request)) {
    return Response.json({ error: "Mangler token" }, { status: 401 });
  }
  const sak = finnInternSak(Number((await params).id));
  if (!sak) {
    return Response.json({ error: "Fant ikke saken" }, { status: 404 });
  }
  return Response.json(sak);
}
