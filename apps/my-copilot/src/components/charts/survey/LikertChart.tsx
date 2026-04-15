"use client";

import React from "react";
import { Bar } from "react-chartjs-2";
import { commonHorizontalBarOptions } from "@/lib/chart-utils";
import { Box, Heading } from "@navikt/ds-react";

interface LikertItem {
  label: string;
  helt_enig: number;
  enig: number;
  noytral: number;
  uenig: number;
  helt_uenig: number;
}

interface LikertChartProps {
  title: string;
  items: LikertItem[];
  total?: number;
}

const COLORS = {
  helt_enig: "rgba(16, 185, 129, 1)", // green
  enig: "rgba(16, 185, 129, 0.5)", // light green
  noytral: "rgba(156, 163, 175, 0.5)", // gray
  uenig: "rgba(239, 68, 68, 0.5)", // light red
  helt_uenig: "rgba(239, 68, 68, 1)", // red
};

// Round percentages so segments always sum to exactly 100
function roundToHundred(values: number[]): number[] {
  const floored = values.map(Math.floor);
  const remainder = Math.min(
    100 - floored.reduce((a, b) => a + b, 0),
    values.length,
  );
  const decimals = values.map((v, i) => ({ i, d: v - floored[i] }));
  decimals.sort((a, b) => b.d - a.d);
  for (let j = 0; j < remainder; j++) {
    floored[decimals[j].i]++;
  }
  return floored;
}

export const LikertChart: React.FC<LikertChartProps> = ({ title, items, total = 163 }) => {
  if (items.length === 0) {
    return null;
  }

  const percentages = items.map((item) => {
    const raw = [
      (item.helt_enig * 100) / total,
      (item.enig * 100) / total,
      (item.noytral * 100) / total,
      (item.uenig * 100) / total,
      (item.helt_uenig * 100) / total,
    ];
    return roundToHundred(raw);
  });

  const chartData = {
    labels: items.map((i) => i.label),
    datasets: [
      {
        label: "Helt enig",
        data: percentages.map((p) => p[0]),
        backgroundColor: COLORS.helt_enig,
        borderRadius: { topLeft: 4, bottomLeft: 4 },
      },
      {
        label: "Enig",
        data: percentages.map((p) => p[1]),
        backgroundColor: COLORS.enig,
      },
      {
        label: "Nøytral",
        data: percentages.map((p) => p[2]),
        backgroundColor: COLORS.noytral,
      },
      {
        label: "Uenig",
        data: percentages.map((p) => p[3]),
        backgroundColor: COLORS.uenig,
      },
      {
        label: "Helt uenig",
        data: percentages.map((p) => p[4]),
        backgroundColor: COLORS.helt_uenig,
        borderRadius: { topRight: 4, bottomRight: 4 },
      },
    ],
  };

  const options = {
    ...commonHorizontalBarOptions,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        position: "top" as const,
        labels: {
          usePointStyle: true,
          pointStyle: "circle",
          padding: 16,
          font: { size: 11 },
        },
      },
      tooltip: {
        ...commonHorizontalBarOptions.plugins.tooltip,
        callbacks: {
          label: (ctx: { dataset: { label?: string }; raw: unknown }) => `${ctx.dataset.label}: ${ctx.raw} %`,
        },
      },
    },
    scales: {
      x: {
        stacked: true,
        beginAtZero: true,
        max: 100,
        border: { display: false },
        grid: { color: "rgba(0, 0, 0, 0.06)", drawBorder: false },
        ticks: { color: "#6B7280", font: { size: 11 }, callback: (v: string | number) => `${v} %` },
      },
      y: {
        stacked: true,
        border: { display: false },
        grid: { display: false },
        ticks: { color: "#374151", font: { size: 11 } },
      },
    },
  };

  return (
    <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
      <Heading size="small" level="3" spacing>
        {title}
      </Heading>
      <div style={{ height: items.length * 50 + 60 }}>
        <Bar data={chartData} options={options} />
      </div>
    </Box>
  );
};
