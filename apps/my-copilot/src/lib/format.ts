/**
 * Format a number with Norwegian locale formatting
 * Uses space as thousands separator
 * @param value - The number to format
 * @returns Formatted string with space as thousands separator
 */
export function formatNumber(value: number): string {
  return new Intl.NumberFormat("nb-NO").format(value);
}

/**
 * Format a percentage value
 * @param value - The percentage value (e.g., 25 for 25%)
 * @returns Formatted string with % symbol
 */
export function formatPercentage(value: number): string {
  return `${value}%`;
}

export function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString("nb-NO", {
    day: "numeric",
    month: "long",
    year: "numeric",
  });
}
