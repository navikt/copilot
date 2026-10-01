import { listSaker } from "../../../lib/saker.ts";

export async function GET(): Promise<Response> {
  return Response.json(listSaker());
}
