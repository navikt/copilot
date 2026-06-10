/** Cyclic accent colors for episodes. Provides visual differentiation. */
const ACCENTS = ["#66d4cf", "#9af0a8", "#ffd485", "#c6a8ff", "#7cc7ff", "#ff9db1"] as const;

/**
 * Get the accent color for an episode number.
 * Cycles through ACCENTS array; unknown/non-numeric episodes get first color.
 */
export function accentForEpisode(episode: string | undefined): string {
  const n = Number.parseInt(episode ?? "", 10);
  if (Number.isFinite(n) && n > 0) {
    return ACCENTS[(n - 1) % ACCENTS.length];
  }
  return ACCENTS[0];
}
