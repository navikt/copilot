const COPILOT_API_URL = process.env.COPILOT_API_URL || "http://copilot-api";

export type DailyFact = { text: string; href: string };

// The day's sentence from copilot-api. The API picks the template and does the
// rounding and suppression; this side only checks the shape. Any failure hides
// the strip.
export function parseDailyFact(body: unknown): DailyFact | null {
  if (!body || typeof body !== "object") return null;
  const { text, href } = body as Record<string, unknown>;
  if (typeof text !== "string" || !text || typeof href !== "string" || !href.startsWith("/innsikt")) return null;
  return { text, href };
}

export async function getDailyFact(): Promise<DailyFact | null> {
  try {
    const res = await fetch(`${COPILOT_API_URL}/public/v1/fact`, {
      next: { revalidate: 3600 },
      signal: AbortSignal.timeout(5000),
    });
    return res.ok ? parseDailyFact(await res.json()) : null;
  } catch (error) {
    console.error("Failed to fetch daily fact:", error);
    return null;
  }
}
