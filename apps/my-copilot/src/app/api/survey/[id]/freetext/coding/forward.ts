import { NextResponse } from "next/server";
import { revalidatePath } from "next/cache";
import { getUser, getUserToken, isAdmin } from "@/lib/auth";
import { postSurveyCoding } from "@/lib/survey";

/**
 * Forwards a coding POST for /admin/undersokelser/[id]/fritekst/koding to copilot-survey, which checks the admin
 * group again and validates the codes. A route handler, not a server action: an action id dies with each deploy
 * (#1804).
 */
export async function forwardCoding(request: Request, id: string, path: "coding" | "coding/model", maxBytes: number) {
  // Like the page: everyone but admins gets 404.
  if (!isAdmin(await getUser(false))) return NextResponse.json({ error: "Not found" }, { status: 404 });
  const userToken = await getUserToken();
  if (!userToken) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  const body = await readLimited(request, maxBytes);
  if (body === null) return NextResponse.json({ error: "too large" }, { status: 413 });
  try {
    JSON.parse(body);
  } catch {
    return NextResponse.json({ error: "invalid" }, { status: 400 });
  }
  const res = await postSurveyCoding(userToken, id, path, body);
  if (!res) return NextResponse.json({ error: "failed" }, { status: 502 });
  if (res.ok) revalidatePath(`/admin/undersokelser/${id}/fritekst/koding`);
  return new NextResponse(await res.text(), {
    status: res.status,
    headers: { "Content-Type": "application/json", "Cache-Control": "no-store" },
  });
}

/** The body as UTF-8 text, or null once it passes maxBytes: stops reading there instead of buffering it all. */
async function readLimited(request: Request, maxBytes: number): Promise<string | null> {
  if (Number(request.headers.get("content-length") ?? 0) > maxBytes) return null;
  const reader = request.body?.getReader();
  if (!reader) return "";
  const chunks: Uint8Array[] = [];
  let size = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    size += value.byteLength;
    if (size > maxBytes) {
      await reader.cancel();
      return null;
    }
    chunks.push(value);
  }
  return new TextDecoder().decode(Buffer.concat(chunks));
}
