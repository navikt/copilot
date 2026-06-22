"use client";

import React from "react";
import { Bar } from "react-chartjs-2";
import { BodyShort } from "@navikt/ds-react";
import { chartColors, getBackgroundColor } from "@/lib/chart-utils";
import { formatNumber } from "@/lib/format";
import type { DailyCredits } from "@/lib/types";

interface DailyCreditsChartProps {
  data: DailyCredits[];
}

const DailyCreditsChart: React.FC<DailyCreditsChartProps> = ({ data }) => {
  if (!data || data.length === 0) {
    return <BodyShort>Ingen daglig aktivitetsdata tilgjengelig.</BodyShort>;
  }

  const labels = data.map((d) => new Date(d.day).toLocaleDateString("nb-NO", { day: "numeric", month: "short" }));

  const totalCredits = data.reduce((s, d) => s + d.credits, 0);
  const hasCredits = totalCredits > 0;

  const chartData = {
    labels,
    datasets: [
      {
        type: "bar" as const,
        label: "AI-kreditter",
        data: data.map((d) => Math.round(d.credits)),
        backgroundColor: getBackgroundColor(chartColors[3], 0.5),
        borderColor: chartColors[3],
        borderWidth: 1,
        borderRadius: 2,
        yAxisID: "yCredits",
        order: 2,
      },
      {
        type: "line" as const,
        label: "Kodeforslag",
        data: data.map((d) => d.generations),
        borderColor: chartColors[0],
        backgroundColor: getBackgroundColor(chartColors[0], 0.1),
        borderWidth: 2,
        pointRadius: 3,
        tension: 0.3,
        yAxisID: "yActivity",
        order: 1,
      },
      {
        type: "line" as const,
        label: "Akseptert",
        data: data.map((d) => d.acceptances),
        borderColor: chartColors[1],
        backgroundColor: getBackgroundColor(chartColors[1], 0.1),
        borderWidth: 2,
        pointRadius: 3,
        tension: 0.3,
        yAxisID: "yActivity",
        order: 1,
      },
      {
        type: "line" as const,
        label: "CLI-forespørsler",
        data: data.map((d) => d.cli_requests),
        borderColor: chartColors[2],
        backgroundColor: getBackgroundColor(chartColors[2], 0.1),
        borderWidth: 2,
        pointRadius: 3,
        tension: 0.3,
        yAxisID: "yActivity",
        order: 1,
      },
    ],
  };

  const options = {
    responsive: true,
    maintainAspectRatio: true,
    interaction: { mode: "index" as const, intersect: false },
    plugins: {
      legend: {
        position: "top" as const,
        labels: { usePointStyle: true, pointStyle: "circle" as const, padding: 16, font: { size: 11 } },
      },
      tooltip: {
        backgroundColor: "rgba(0,0,0,0.8)",
        padding: 10,
        cornerRadius: 6,
        callbacks: {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          label: (ctx: any) => {
            const val = formatNumber(ctx.parsed.y);
            return ctx.dataset.label === "AI-kreditter" ? ` ${val} kreditter` : ` ${val}`;
          },
        },
      },
    },
    scales: {
      x: {
        grid: { display: false },
        ticks: { color: "#6B7280", font: { size: 10 }, maxRotation: 45 },
      },
      yActivity: {
        type: "linear" as const,
        position: "left" as const,
        beginAtZero: true,
        grid: { color: "rgba(0,0,0,0.06)" },
        ticks: { color: "#6B7280", font: { size: 11 } },
        title: { display: true, text: "Aktivitet", color: "#6B7280", font: { size: 10 } },
      },
      yCredits: {
        type: "linear" as const,
        position: "right" as const,
        beginAtZero: true,
        display: hasCredits,
        grid: { drawOnChartArea: false },
        ticks: {
          color: chartColors[3],
          font: { size: 11 },
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          callback: (v: any) => (v >= 1000 ? `${Math.round(v / 1000)}K` : v),
        },
        title: { display: hasCredits, text: "Kreditter", color: chartColors[3], font: { size: 10 } },
      },
    },
  };

  return (
    <div>
      {hasCredits && (
        <BodyShort size="small" className="text-gray-600" style={{ marginBottom: "var(--a-spacing-4)" }}>
          Totalt {formatNumber(Math.round(totalCredits))} AI-kreditter siste 30 dager
        </BodyShort>
      )}
      <div className="aspect-[8/1]">
        <Bar
          data={chartData as Parameters<typeof Bar>[0]["data"]}
          options={options as Parameters<typeof Bar>[0]["options"]}
        />
      </div>
      {!hasCredits && (
        <BodyShort size="small" className="text-gray-500" style={{ marginTop: "var(--a-spacing-4)" }}>
          Kredittellingen startet 19. juni 2026 — data vil vokse over tid.
        </BodyShort>
      )}
    </div>
  );
};

export default DailyCreditsChart;
