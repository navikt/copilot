import type { AdoptionCohortDay, AdoptionCohortTrendData } from "@/lib/types";

/**
 * Transform raw cohort data into chart-friendly trend data.
 */
export function transformCohortData(data: AdoptionCohortDay[]): AdoptionCohortTrendData {
  const dayMap = new Map<string, { phase0: number; phase1: number; phase2: number; phase3: number }>();

  for (const row of data) {
    if (!dayMap.has(row.day)) {
      dayMap.set(row.day, { phase0: 0, phase1: 0, phase2: 0, phase3: 0 });
    }
    const entry = dayMap.get(row.day)!;
    const key = `phase${row.phase}` as keyof typeof entry;
    if (key in entry) {
      entry[key] = row.user_count;
    }
  }

  const sortedDays = [...dayMap.keys()].sort();
  const result: AdoptionCohortTrendData = {
    days: sortedDays,
    phase0: [],
    phase1: [],
    phase2: [],
    phase3: [],
    total: [],
  };

  for (const day of sortedDays) {
    const entry = dayMap.get(day)!;
    result.phase0.push(entry.phase0);
    result.phase1.push(entry.phase1);
    result.phase2.push(entry.phase2);
    result.phase3.push(entry.phase3);
    result.total.push(entry.phase0 + entry.phase1 + entry.phase2 + entry.phase3);
  }

  return result;
}
