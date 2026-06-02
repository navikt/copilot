"use client";

import type { MonthlyTrend } from "@/lib/types";
import React from "react";
import { Bar } from "react-chartjs-2";
import { chartColors, getBackgroundColor, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { VStack, Heading, HGrid, BodyShort, Box } from "@navikt/ds-react";
import { formatNumber } from "@/lib/format";

interface MonthlyTrendsChartProps {
  data: MonthlyTrend[];
}

const MonthlyTrendsChart: React.FC<MonthlyTrendsChartProps> = ({ data }) => {
  if (!data || data.length === 0) {
    return <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>;
  }

  const labels = data.map((d) => {
    // Mark current partial month
    const now = new Date();
    const currentMonthStr = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
    return d.month === currentMonthStr ? `${d.month} *` : d.month;
  });

  // Summary: latest COMPLETE month vs previous
  const now = new Date();
  const currentMonthStr = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
  const completeMonths = data.filter((d) => d.month !== currentMonthStr);
  const latest = completeMonths.length > 0 ? completeMonths[completeMonths.length - 1] : data[data.length - 1];
  const prev = completeMonths.length > 1 ? completeMonths[completeMonths.length - 2] : null;

  const usersChartData = {
    labels,
    datasets: [
      {
        label: "Unike brukere",
        data: data.map((d) => d.unique_users),
        backgroundColor: getBackgroundColor(chartColors[0], 0.6),
        borderColor: chartColors[0],
        borderWidth: 1,
      },
      {
        label: "Agent-brukere",
        data: data.map((d) => d.agent_users),
        backgroundColor: getBackgroundColor(chartColors[1], 0.6),
        borderColor: chartColors[1],
        borderWidth: 1,
      },
      {
        label: "Chat-brukere",
        data: data.map((d) => d.chat_users),
        backgroundColor: getBackgroundColor(chartColors[2], 0.6),
        borderColor: chartColors[2],
        borderWidth: 1,
      },
      {
        label: "CLI-brukere",
        data: data.map((d) => d.cli_users),
        backgroundColor: getBackgroundColor(chartColors[3], 0.6),
        borderColor: chartColors[3],
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
        backgroundColor: getBackgroundColor(chartColors[4] || "#8b5cf6", 0.6),
        borderColor: chartColors[4] || "#8b5cf6",
        borderWidth: 1,
      },
      {
        label: "Chat/agent-interaksjoner",
        data: data.map((d) => d.ide_interactions),
        backgroundColor: getBackgroundColor(chartColors[0], 0.6),
        borderColor: chartColors[0],
        borderWidth: 1,
      },
      {
        label: "CLI-forespørsler",
        data: data.map((d) => d.cli_requests),
        backgroundColor: getBackgroundColor(chartColors[3], 0.6),
        borderColor: chartColors[3],
        borderWidth: 1,
      },
    ],
  };

  const barOptions = {
    responsive: true,
    maintainAspectRatio: true,
    plugins: {
      legend: {
        position: "top" as const,
        labels: { usePointStyle: true, pointStyle: "circle", padding: 16, font: { size: 11 } },
      },
    },
    scales: {
      x: { grid: { display: false } },
      y: { beginAtZero: true, grid: { color: "rgba(0,0,0,0.06)" } },
    },
  };

  function pctChange(current: number, previous: number | undefined): string {
    if (!previous || previous === 0) return "";
    const change = Math.round(((current - previous) / previous) * 100);
    return change > 0 ? `+${change}%` : `${change}%`;
  }

  return (
    <VStack gap="space-16">
      <Heading size="small" level="3">
        Månedlige trender
      </Heading>

      {/* Summary cards for latest month */}
      <HGrid columns={{ xs: 2, sm: 4 }} gap="space-8">
        <Box background="info-soft" padding="space-12" borderRadius="8">
          <div className="text-center">
            <div className="text-lg font-semibold">{formatNumber(latest.unique_users)}</div>
            <BodyShort size="small" className="text-gray-600">
              Brukere
            </BodyShort>
            {prev && (
              <BodyShort size="small" className="text-gray-500">
                {pctChange(latest.unique_users, prev.unique_users)}
              </BodyShort>
            )}
          </div>
        </Box>
        <Box background="success-soft" padding="space-12" borderRadius="8">
          <div className="text-center">
            <div className="text-lg font-semibold">
              {formatNumber(latest.code_generations + latest.ide_interactions + latest.cli_requests)}
            </div>
            <BodyShort size="small" className="text-gray-600">
              Aktivitet
            </BodyShort>
            {prev && (
              <BodyShort size="small" className="text-gray-500">
                {pctChange(
                  latest.code_generations + latest.ide_interactions + latest.cli_requests,
                  prev.code_generations + prev.ide_interactions + prev.cli_requests
                )}
              </BodyShort>
            )}
          </div>
        </Box>
        <Box background="warning-soft" padding="space-12" borderRadius="8">
          <div className="text-center">
            <div className="text-lg font-semibold">{formatNumber(latest.lines_added)}</div>
            <BodyShort size="small" className="text-gray-600">
              Linjer lagt til
            </BodyShort>
            {prev && (
              <BodyShort size="small" className="text-gray-500">
                {pctChange(latest.lines_added, prev.lines_added)}
              </BodyShort>
            )}
          </div>
        </Box>
        <Box background="accent-soft" padding="space-12" borderRadius="8">
          <div className="text-center">
            <div className="text-lg font-semibold">{formatNumber(latest.cli_users)}</div>
            <BodyShort size="small" className="text-gray-600">
              CLI-brukere
            </BodyShort>
            {prev && (
              <BodyShort size="small" className="text-gray-500">
                {pctChange(latest.cli_users, prev.cli_users)}
              </BodyShort>
            )}
          </div>
        </Box>
      </HGrid>

      {/* Charts */}
      <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
        <Box background="neutral-soft" padding="space-16" borderRadius="8">
          <VStack gap="space-8">
            <BodyShort weight="semibold">Brukere per funksjon</BodyShort>
            <div className="aspect-[2/1]">
              <Bar data={usersChartData} options={barOptions} />
            </div>
          </VStack>
        </Box>
        <Box background="neutral-soft" padding="space-16" borderRadius="8">
          <VStack gap="space-8">
            <BodyShort weight="semibold">Aktivitet per type</BodyShort>
            <div className="aspect-[2/1]">
              <Bar data={activityChartData} options={barOptions} />
            </div>
          </VStack>
        </Box>
      </HGrid>
    </VStack>
  );
};

export default MonthlyTrendsChart;
