const COPILOT_API_URL = process.env.COPILOT_API_URL || "http://copilot-api";

export type DailyFact = { text: string; href: string; weekUsers?: number };

// The day's sentence from copilot-api. The API picks the template and does the
// rounding and suppression; this side only checks the shape. Any failure hides
// the fact and the weekly count.
export function parseDailyFact(body: unknown): DailyFact | null {
  if (!body || typeof body !== "object") return null;
  const { text, href, weekUsers } = body as Record<string, unknown>;
  if (typeof text !== "string" || !text || typeof href !== "string" || !href.startsWith("/innsikt")) return null;
  // Rounded to 50 by the API; anything else is not a number we may show.
  if (typeof weekUsers === "number" && weekUsers >= 50 && weekUsers % 50 === 0) return { text, href, weekUsers };
  return { text, href };
}

const DEV_FALLBACK_FACT: DailyFact = {
  text: "28 % av brukerne brukte en egendefinert agent den siste uka.",
  href: "/innsikt/tilpasninger",
  weekUsers: 550,
};

export async function getDailyFact(): Promise<DailyFact | null> {
  try {
    const res = await fetch(`${COPILOT_API_URL}/public/v1/fact`, {
      next: { revalidate: 3600 },
      signal: AbortSignal.timeout(5000),
    });
    if (res.ok) return parseDailyFact(await res.json());
  } catch (error) {
    console.error("Failed to fetch daily fact:", error);
  }
  return process.env.NODE_ENV === "development" ? DEV_FALLBACK_FACT : null;
}
