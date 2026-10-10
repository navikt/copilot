"use client";

import type { AdoptionCohortWeek, AdoptionCohortTrendData } from "@/lib/types";
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
  data: AdoptionCohortWeek[];
}

/**
 * Pivot the API's weekly rows into one series per phase. The API averages per week and
 * suppresses small cells; a missing phase in a week stays null, not 0.
 */
export function transformCohortData(data: AdoptionCohortWeek[]): AdoptionCohortTrendData {
  const weeks = [...new Set(data.map((r) => r.week))].sort();
  const result: AdoptionCohortTrendData = { weeks, phase0: [], phase1: [], phase2: [], phase3: [] };
  for (const week of weeks) {
    for (const phase of [0, 1, 2, 3] as const) {
      result[`phase${phase}`].push(data.find((r) => r.week === week && r.phase === phase)?.user_count ?? null);
    }
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
  const formatLabel = (dateStr: string) =>
    `Uke ${new Date(dateStr).toLocaleDateString("nb-NO", { day: "numeric", month: "short" })}`;

  const chartData = {
    labels: trend.weeks.map(formatLabel),
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
        text: "KI-adopsjon – ukesgjennomsnitt",
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
        Faser med færre enn fem brukere i snitt en uke er skjult.
      </BodyShort>
    </div>
  );
};

export default AdoptionCohortsChart;
