import { NextResponse } from "next/server";
import { revalidatePath } from "next/cache";
import { getUser, getUserToken, isAdmin } from "@/lib/auth";
import { hideSurveyFreetext } from "@/lib/survey";

/**
 * Hides one free text for /admin/undersokelser/[id]/fritekst. A route handler,
 * not a server action: an action id dies with each deploy (#1804).
 * copilot-survey checks the admin group again.
 */
export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }) {
  // Like the page: everyone but admins gets 404.
  if (!isAdmin(await getUser(false))) return NextResponse.json({ error: "Not found" }, { status: 404 });
  const userToken = await getUserToken();
  if (!userToken) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  const { id } = await params;
  const body = (await request.json().catch(() => null)) as { token?: unknown } | null;
  if (typeof body?.token !== "string") return NextResponse.json({ error: "invalid" }, { status: 400 });
  if (!(await hideSurveyFreetext(userToken, id, body.token))) {
    return NextResponse.json({ error: "failed" }, { status: 502 });
  }
  revalidatePath(`/admin/undersokelser/${id}/fritekst`);
  return new NextResponse(null, { status: 204 });
}
