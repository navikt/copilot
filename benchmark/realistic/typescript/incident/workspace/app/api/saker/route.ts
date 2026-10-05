import { listSaker } from "../../../lib/saker.ts";

export async function GET(request: Request): Promise<Response> {
  const status = new URL(request.url).searchParams.get("status") ?? "alle";
  const saker = listSaker(status);
  return Response.json(saker);
}
