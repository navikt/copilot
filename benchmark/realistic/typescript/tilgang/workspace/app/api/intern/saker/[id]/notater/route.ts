import { getToken } from "@navikt/oasis";
import { leggTilNotat } from "../../../../../../lib/saker.ts";

export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
): Promise<Response> {
  if (!getToken(request)) {
    return Response.json({ error: "Mangler token" }, { status: 401 });
  }
  const { tekst } = (await request.json()) as { tekst?: unknown };
  if (typeof tekst !== "string" || tekst.trim() === "") {
    return Response.json({ error: "Notatet mangler tekst" }, { status: 400 });
  }
  const sak = leggTilNotat(Number((await params).id), tekst);
  if (!sak) {
    return Response.json({ error: "Fant ikke saken" }, { status: 404 });
  }
  return Response.json(sak, { status: 201 });
}
