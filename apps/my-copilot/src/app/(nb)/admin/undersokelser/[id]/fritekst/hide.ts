/** Hides one free text through the route handler (not a server action, see README). */
export async function hideFreetext(surveyId: string, token: string): Promise<boolean> {
  try {
    const res = await fetch(`/api/survey/${encodeURIComponent(surveyId)}/freetext/hide`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token }),
      redirect: "manual",
    });
    return res.status === 204;
  } catch {
    return false;
  }
}
