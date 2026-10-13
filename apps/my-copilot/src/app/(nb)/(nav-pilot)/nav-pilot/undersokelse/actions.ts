"use server";

import { getUserToken } from "@/lib/auth";
import { submitAnswers, type Answers, type SubmitResult } from "@/lib/survey";

export async function sendSurvey(surveyId: string, answers: Answers): Promise<SubmitResult> {
  const token = await getUserToken();
  if (!token) return { status: "error" };
  try {
    return await submitAnswers(token, surveyId, answers);
  } catch (err) {
    console.error("survey submit failed", err instanceof Error ? err.message : String(err));
    return { status: "error" };
  }
}
