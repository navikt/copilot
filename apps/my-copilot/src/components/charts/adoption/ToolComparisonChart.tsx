"use client";

import type { AdoptionSummary } from "@/lib/types";
import React from "react";
import { Bar } from "react-chartjs-2";
import { chartColors, commonHorizontalBarOptions, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { BodyShort, Box, Heading } from "@navikt/ds-react";
import { extractToolComparison } from "@/lib/adoption-utils";

interface ToolComparisonChartProps {
  data: AdoptionSummary | null;
}

const toolColors: Record<string, string> = {
  "Kun Copilot": chartColors[0],
  Cursor: chartColors[2],
  Claude: chartColors[3],
  Windsurf: chartColors[5],
};

const ToolComparisonChart: React.FC<ToolComparisonChartProps> = ({ data }) => {
  if (!data) {
    return (
      <Box padding="space-16" borderRadius="8" borderWidth="1" borderColor="neutral-subtle" background="default">
        <BodyShort>{NO_DATA_MESSAGE}</BodyShort>
      </Box>
    );
  }

  const tools = extractToolComparison(data);

  if (tools.length === 0) return <BodyShort>Ingen repoer har KI-tilpasninger ennå.</BodyShort>;

  const chartData = {
    labels: tools.map((t) => t.label),
    datasets: [
      {
        data: tools.map((t) => t.value),
        backgroundColor: tools.map((t) => toolColors[t.label] ?? chartColors[4]),
        borderRadius: 4,
        barThickness: 24,
      },
    ],
  };

  const height = Math.max(120, tools.length * 40);

  return (
    <Box padding="space-16" borderRadius="8" borderWidth="1" borderColor="neutral-subtle" background="default">
      <Heading size="small" level="3" spacing>
        Repoer per KI-verktøy
      </Heading>
      <div style={{ height }}>
        <Bar data={chartData} options={commonHorizontalBarOptions} />
      </div>
      <BodyShort size="small" textColor="subtle">
        Et repo kan ha flere verktøy. Under Cursor telles et repo to ganger hvis det har både .cursorrules og
        .cursor/rules/.
      </BodyShort>
    </Box>
  );
};

export default ToolComparisonChart;
