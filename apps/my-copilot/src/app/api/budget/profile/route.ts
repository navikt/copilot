import { budgetProxy } from "../proxy";

export const GET = () => budgetProxy("/profile");

export async function PUT(request: Request) {
  const body = (await request.json().catch(() => ({}))) as Record<string, unknown>;
  // Only the known fields go on; the backend rejects anything else.
  return budgetProxy("/profile", { method: "PUT", body: JSON.stringify({ teamId: body.teamId, role: body.role }) });
}
