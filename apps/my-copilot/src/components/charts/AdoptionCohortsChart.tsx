"use client";

import type { AdoptionCohortDay, AdoptionCohortTrendData } from "@/lib/types";
import React from "react";
import { Line } from "react-chartjs-2";
import { commonLineOptions, getBackgroundColor, chartWrapperClass, NO_DATA_MESSAGE } from "@/lib/chart-utils";

// Phase colors: muted gray → blue → purple → green
const phaseColors = [
  "rgba(156, 163, 175, 1)", // Phase 0 — Ingen AI-bruk (gray)
  "rgba(59, 130, 246, 1)", // Phase 1 — Kodeforslag (blue)
  "rgba(139, 92, 246, 1)", // Phase 2 — Én agent-flate (purple)
  "rgba(16, 185, 129, 1)", // Phase 3 — Flere agent-flater (green)
];

const phaseLabels = [
  "Fase 0: Ingen AI-bruk",
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
  const weekMap = new Map<string, { phase0: number[]; phase1: number[]; phase2: number[]; phase3: number[] }>();

  for (let i = 0; i < data.days.length; i++) {
    const date = new Date(data.days[i]);
    // ISO week: Monday-based — get the Monday of the week
    const day = date.getDay();
    const diff = date.getDate() - day + (day === 0 ? -6 : 1);
    const monday = new Date(date.setDate(diff));
    const weekLabel = monday.toISOString().slice(0, 10);

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
  const avg = (arr: number[]) => (arr.length === 0 ? 0 : Math.round(arr.reduce((a, b) => a + b, 0) / arr.length));

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
    result.total.push(p0 + p1 + p2 + p3);
  }

  return result;
}

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
        text: useWeekly ? "AI-adopsjon – ukesgjennomsnitt" : "AI-adopsjon – daglig fordeling",
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
    </div>
  );
};

export default AdoptionCohortsChart;
