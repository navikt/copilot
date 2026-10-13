"use client";

import type { MonthlyTrend } from "@/lib/types";
import React from "react";
import { Bar } from "react-chartjs-2";
import { axColor, bottomLegend, chartBoxClass, seriesColor, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { VStack, HGrid, BodyShort, Box } from "@navikt/ds-react";
import { formatNumber, formatPercent } from "@/lib/format";
import { currentMonthUTC, selectCompleteMonths } from "@/lib/month-utils";

interface MonthlyTrendsChartProps {
  data: MonthlyTrend[];
}

const MonthlyTrendsChart: React.FC<MonthlyTrendsChartProps> = ({ data }) => {
  if (!data || data.length === 0) {
    return <BodyShort>{NO_DATA_MESSAGE}</BodyShort>;
  }

  const current = currentMonthUTC();
  const labels = data.map((d) => (d.month === current ? `${d.month} *` : d.month));

  // Summary: latest COMPLETE month vs previous (using shared utility)
  const { latestComplete, prevComplete: prev } = selectCompleteMonths(data, current);
  // Fall back to last data point if no month qualifies as complete
  const latest = latestComplete ?? data[data.length - 1];

  const usersChartData = {
    labels,
    datasets: [
      {
        label: "Unike brukere",
        data: data.map((d) => d.unique_users),
        backgroundColor: seriesColor(0, 0.6),
        borderColor: seriesColor(0),
        borderWidth: 1,
      },
      {
        label: "Agent-brukere",
        data: data.map((d) => d.agent_users),
        backgroundColor: seriesColor(1, 0.6),
        borderColor: seriesColor(1),
        borderWidth: 1,
      },
      {
        label: "Chat-brukere",
        data: data.map((d) => d.chat_users),
        backgroundColor: seriesColor(2, 0.6),
        borderColor: seriesColor(2),
        borderWidth: 1,
      },
      {
        label: "CLI-brukere",
        data: data.map((d) => d.cli_users),
        backgroundColor: seriesColor(3, 0.6),
        borderColor: seriesColor(3),
        borderWidth: 1,
      },
    ],
  };

  const activityChartData = {
    labels,
    datasets: [
      {
        label: "Kodeforslag",
        data: data.map((d) => d.code_generations),
        backgroundColor: seriesColor(4, 0.6),
        borderColor: seriesColor(4),
        borderWidth: 1,
      },
      {
        label: "Chat/agent-interaksjoner",
        data: data.map((d) => d.ide_interactions),
        backgroundColor: seriesColor(0, 0.6),
        borderColor: seriesColor(0),
        borderWidth: 1,
      },
      {
        label: "CLI-forespørsler",
        data: data.map((d) => d.cli_requests),
        backgroundColor: seriesColor(3, 0.6),
        borderColor: seriesColor(3),
        borderWidth: 1,
      },
    ],
  };

  const barOptions = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: { legend: bottomLegend },
    scales: {
      x: { grid: { display: false }, ticks: { color: () => axColor("text-neutral-subtle") } },
      y: {
        beginAtZero: true,
        grid: { color: () => axColor("border-neutral-subtle", 0.4) },
        ticks: { color: () => axColor("text-neutral-subtle") },
      },
    },
  };

  function pctChange(current: number, previous: number | undefined): string {
    if (!previous || previous === 0) return "";
    const change = Math.round(((current - previous) / previous) * 100);
    return change > 0 ? `+${formatPercent(change)}` : formatPercent(change);
  }

  return (
    <VStack gap="space-16">
      {/* Users and activity are the key figures above, so only the other two are shown here. */}
      <HGrid columns={{ xs: 1, sm: 2 }} gap="space-8">
        {(
          [
            ["Linjer lagt til", latest.lines_added, prev?.lines_added],
            ["CLI-brukere", latest.cli_users, prev?.cli_users],
          ] as const
        ).map(([label, value, before]) => (
          <Box key={label} background="neutral-soft" padding="space-12" borderRadius="8">
            <BodyShort size="small" textColor="subtle">
              {label}
            </BodyShort>
            <BodyShort weight="semibold">{formatNumber(value)}</BodyShort>
            {before ? (
              <BodyShort size="small" textColor="subtle">
                {pctChange(value, before)} fra forrige måned
              </BodyShort>
            ) : null}
          </Box>
        ))}
      </HGrid>
      <BodyShort size="small" textColor="subtle">
        Måneden merket med * er ikke ferdig.
      </BodyShort>

      {/* Charts */}
      <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
        <Box background="neutral-soft" padding="space-16" borderRadius="8">
          <VStack gap="space-8">
            <BodyShort weight="semibold">Brukere per funksjon</BodyShort>
            <div className={chartBoxClass}>
              <Bar data={usersChartData} options={barOptions} />
            </div>
          </VStack>
        </Box>
        <Box background="neutral-soft" padding="space-16" borderRadius="8">
          <VStack gap="space-8">
            <BodyShort weight="semibold">Aktivitet per type</BodyShort>
            <div className={chartBoxClass}>
              <Bar data={activityChartData} options={barOptions} />
            </div>
          </VStack>
        </Box>
      </HGrid>
    </VStack>
  );
};

export default MonthlyTrendsChart;
