"use client";

import type { AdoptionCohortDay, AdoptionCohortTrendData } from "@/lib/types";
import React from "react";
import { Line } from "react-chartjs-2";
import { commonLineOptions, getBackgroundColor, chartWrapperClass, NO_DATA_MESSAGE } from "@/lib/chart-utils";

// Phase colors: muted gray → blue → purple → green
const phaseColors = [
  "rgba(156, 163, 175, 1)", // Phase 0 — No cohort (gray)
  "rgba(59, 130, 246, 1)", // Phase 1 — Code first (blue)
  "rgba(139, 92, 246, 1)", // Phase 2 — Agent first (purple)
  "rgba(16, 185, 129, 1)", // Phase 3 — Multi-agent (green)
];

const phaseLabels = ["Ingen kohort", "Kode først", "Agent først", "Multi-agent"];

interface AdoptionCohortsChartProps {
  data: AdoptionCohortDay[];
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

const AdoptionCohortsChart: React.FC<AdoptionCohortsChartProps> = ({ data }) => {
  if (!data || data.length === 0) {
    return (
      <div className={chartWrapperClass}>
        <div className="text-center text-gray-500 py-8">{NO_DATA_MESSAGE}</div>
      </div>
    );
  }

  const trend = transformCohortData(data);

  const chartData = {
    labels: trend.days,
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
        text: "AI-adopsjonskohorter over tid",
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
