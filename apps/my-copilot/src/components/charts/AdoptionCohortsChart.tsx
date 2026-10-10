"use client";

import type { AdoptionCohortWeek, AdoptionCohortTrendData } from "@/lib/types";
import React from "react";
import { BodyShort } from "@navikt/ds-react";
import { Line } from "react-chartjs-2";
import {
  axColor,
  bottomLegend,
  chartBoxClass,
  commonLineOptions,
  chartWrapperClass,
  NO_DATA_MESSAGE,
} from "@/lib/chart-utils";

// Phase colours (Aksel tokens): muted grey → blue → purple → green
const phaseTokens = ["neutral-500", "accent-600", "meta-purple-600", "success-600"];
const phaseColor =
  (phase: number, alpha = 1) =>
  () =>
    axColor(phaseTokens[phase], alpha);

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
        <BodyShort>{NO_DATA_MESSAGE}</BodyShort>
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
        borderColor: phaseColor(3),
        backgroundColor: phaseColor(3, 0.3),
        fill: true,
        tension: 0.4,
        order: 1,
      },
      {
        label: phaseLabels[2],
        data: trend.phase2,
        borderColor: phaseColor(2),
        backgroundColor: phaseColor(2, 0.3),
        fill: true,
        tension: 0.4,
        order: 2,
      },
      {
        label: phaseLabels[1],
        data: trend.phase1,
        borderColor: phaseColor(1),
        backgroundColor: phaseColor(1, 0.3),
        fill: true,
        tension: 0.4,
        order: 3,
      },
      {
        label: phaseLabels[0],
        data: trend.phase0,
        borderColor: phaseColor(0),
        backgroundColor: phaseColor(0, 0.15),
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
      legend: bottomLegend,
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
      <div className={chartBoxClass}>
        <Line data={chartData} options={options} />
      </div>
      <BodyShort size="small" textColor="subtle">
        Faser med færre enn fem brukere i snitt en uke er skjult.
      </BodyShort>
    </div>
  );
};

export default AdoptionCohortsChart;
