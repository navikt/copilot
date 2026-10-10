"use client";

import type { AdoptionCohortDay, AdoptionCohortTrendData } from "@/lib/types";
import React from "react";
import { BodyShort } from "@navikt/ds-react";
import { Line } from "react-chartjs-2";
import { commonLineOptions, getBackgroundColor, chartWrapperClass, NO_DATA_MESSAGE } from "@/lib/chart-utils";

// Phase colors: muted gray → blue → purple → green
const phaseColors = [
  "rgba(156, 163, 175, 1)", // Phase 0 — Ingen KI-bruk (gray)
  "rgba(59, 130, 246, 1)", // Phase 1 — Kodeforslag (blue)
  "rgba(139, 92, 246, 1)", // Phase 2 — Én agent-flate (purple)
  "rgba(16, 185, 129, 1)", // Phase 3 — Flere agent-flater (green)
];

const phaseLabels = [
  "Fase 0: Ingen KI-bruk",
  "Fase 1: Kodeforslag",
  "Fase 2: Én agentflate",
  "Fase 3: Flere agentflater",
];

interface AdoptionCohortsChartProps {
  data: AdoptionCohortDay[];
}

/**
 * Aggregate daily data into ISO weeks (Monday-based).
 * For each week, takes the average user count per phase.
 */
function aggregateToWeeks(data: AdoptionCohortTrendData): AdoptionCohortTrendData {
  const weekMap = new Map<
    string,
    { phase0: (number | null)[]; phase1: (number | null)[]; phase2: (number | null)[]; phase3: (number | null)[] }
  >();

  for (let i = 0; i < data.days.length; i++) {
    const date = new Date(data.days[i] + "T00:00:00Z");
    // ISO week: Monday-based — get the Monday of the week (using UTC to avoid TZ shifts)
    const day = date.getUTCDay();
    const diff = date.getUTCDate() - day + (day === 0 ? -6 : 1);
    date.setUTCDate(diff);
    const weekLabel = date.toISOString().slice(0, 10);

    if (!weekMap.has(weekLabel)) {
      weekMap.set(weekLabel, { phase0: [], phase1: [], phase2: [], phase3: [] });
    }
    const entry = weekMap.get(weekLabel)!;
    entry.phase0.push(data.phase0[i]);
    entry.phase1.push(data.phase1[i]);
    entry.phase2.push(data.phase2[i]);
    entry.phase3.push(data.phase3[i]);
  }

  const sortedWeeks = [...weekMap.keys()].sort();
  // Suppressed days (null) are left out of the average; a week with only suppressed days stays null.
  const avg = (arr: (number | null)[]) => {
    const shown = arr.filter((v): v is number => v !== null);
    return shown.length === 0 ? null : Math.round(shown.reduce((a, b) => a + b, 0) / shown.length);
  };
  const sum = (...values: (number | null)[]) => values.reduce<number>((a, b) => a + (b ?? 0), 0);

  const result: AdoptionCohortTrendData = {
    days: sortedWeeks,
    phase0: [],
    phase1: [],
    phase2: [],
    phase3: [],
    total: [],
  };

  for (const week of sortedWeeks) {
    const entry = weekMap.get(week)!;
    const p0 = avg(entry.phase0);
    const p1 = avg(entry.phase1);
    const p2 = avg(entry.phase2);
    const p3 = avg(entry.phase3);
    result.phase0.push(p0);
    result.phase1.push(p1);
    result.phase2.push(p2);
    result.phase3.push(p3);
    result.total.push(sum(p0, p1, p2, p3));
  }

  return result;
}

/**
 * Transform raw cohort data into chart-friendly trend data.
 */
export function transformCohortData(data: AdoptionCohortDay[]): AdoptionCohortTrendData {
  // A phase missing on a day was suppressed by the API (fewer than five users) and stays null, not 0.
  const dayMap = new Map<
    string,
    { phase0: number | null; phase1: number | null; phase2: number | null; phase3: number | null }
  >();

  for (const row of data) {
    if (!dayMap.has(row.day)) {
      dayMap.set(row.day, { phase0: null, phase1: null, phase2: null, phase3: null });
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
    result.total.push((entry.phase0 ?? 0) + (entry.phase1 ?? 0) + (entry.phase2 ?? 0) + (entry.phase3 ?? 0));
  }

  return result;
}

const WEEKLY_THRESHOLD_DAYS = 28;

const AdoptionCohortsChart: React.FC<AdoptionCohortsChartProps> = ({ data }) => {
  if (!data || data.length === 0) {
    return (
      <div className={chartWrapperClass}>
        <div className="text-center text-gray-500 py-8">{NO_DATA_MESSAGE}</div>
      </div>
    );
  }

  let trend = transformCohortData(data);
  const useWeekly = trend.days.length > WEEKLY_THRESHOLD_DAYS;
  if (useWeekly) {
    trend = aggregateToWeeks(trend);
  }

  const formatLabel = (dateStr: string) => {
    const d = new Date(dateStr);
    if (useWeekly) {
      return `Uke ${d.toLocaleDateString("nb-NO", { day: "numeric", month: "short" })}`;
    }
    return d.toLocaleDateString("nb-NO", { day: "numeric", month: "short" });
  };

  const chartData = {
    labels: trend.days.map(formatLabel),
    datasets: [
      {
        label: phaseLabels[3],
        data: trend.phase3,
        borderColor: phaseColors[3],
        backgroundColor: getBackgroundColor(phaseColors[3], 0.3),
        fill: true,
        tension: 0.4,
        order: 1,
      },
      {
        label: phaseLabels[2],
        data: trend.phase2,
        borderColor: phaseColors[2],
        backgroundColor: getBackgroundColor(phaseColors[2], 0.3),
        fill: true,
        tension: 0.4,
        order: 2,
      },
      {
        label: phaseLabels[1],
        data: trend.phase1,
        borderColor: phaseColors[1],
        backgroundColor: getBackgroundColor(phaseColors[1], 0.3),
        fill: true,
        tension: 0.4,
        order: 3,
      },
      {
        label: phaseLabels[0],
        data: trend.phase0,
        borderColor: phaseColors[0],
        backgroundColor: getBackgroundColor(phaseColors[0], 0.15),
        fill: true,
        tension: 0.4,
        order: 4,
      },
    ],
  };

  const options = {
    ...commonLineOptions,
    plugins: {
      ...commonLineOptions.plugins,
      title: {
        display: true,
        text: useWeekly ? "KI-adopsjon – ukesgjennomsnitt" : "KI-adopsjon – daglig fordeling",
        font: { size: 14, weight: "bold" as const },
        padding: { bottom: 16 },
      },
    },
    scales: {
      ...commonLineOptions.scales,
      y: {
        ...commonLineOptions.scales.y,
        stacked: true,
        title: { display: true, text: "Antall brukere" },
      },
      x: {
        ...commonLineOptions.scales.x,
        stacked: true,
      },
    },
  };

  return (
    <div className={chartWrapperClass}>
      <Line data={chartData} options={options} />
      <BodyShort size="small" className="text-gray-600" style={{ marginTop: "var(--a-spacing-2)" }}>
        Faser med færre enn fem brukere en dag er skjult.
      </BodyShort>
    </div>
  );
};

export default AdoptionCohortsChart;
