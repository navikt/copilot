"use server";

import { revalidatePath } from "next/cache";
import { getUserToken, requireAdmin } from "@/lib/auth";
import { hideSurveyFreetext } from "@/lib/survey";

/** Hides one free text. copilot-survey checks the admin group again. */
export async function hideFreetext(surveyId: string, token: string): Promise<boolean> {
  await requireAdmin();
  const userToken = await getUserToken();
  if (!userToken) return false;
  const ok = await hideSurveyFreetext(userToken, surveyId, token);
  if (ok) revalidatePath(`/admin/undersokelser/${surveyId}/fritekst`);
  return ok;
}
