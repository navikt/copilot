"use client";

import type { CustomizationDetail } from "@/lib/types";
import React, { useMemo } from "react";
import { Bar } from "react-chartjs-2";
import { bottomLegend, commonHorizontalBarOptions, seriesColor } from "@/lib/chart-utils";
import { Box, Heading, BodyShort, HGrid } from "@navikt/ds-react";
import { getOfficialFileNames } from "@/lib/customizations";
import { sortCustomizationsByScope } from "@/lib/adoption-utils";

interface TopCustomizationsChartProps {
  data: CustomizationDetail[];
  maxItems?: number;
}

const categoryLabels: Record<string, string> = {
  agents: "Agenter",
  skills: "Skills",
  instructions: "Instruksjoner",
  prompts: "Prompts",
  agentic_workflows: "Agentic Workflows",
  agents_skills: "Installerte skills",
};

/**
 * One bar per file, stacked by origin. A file is either from navikt/copilot or the team's own, so each bar has one
 * coloured part and the stack is «alle». The other series is null, not 0, so no empty segment is drawn.
 */
export function originDatasets(items: CustomizationDetail[], officialNames: Set<string>) {
  const series = (label: string, official: boolean, colour: number) => ({
    label,
    data: items.map((item) => (officialNames.has(item.file_name) === official ? item.repo_count : null)),
    backgroundColor: seriesColor(colour),
    borderRadius: 4,
    barThickness: 12,
    stack: "origin",
  });
  return [series("Fra navikt/copilot", true, 0), series("Egne", false, 3)];
}

// Each file shows how many repos have it, coloured by origin; the tooltip adds how many of them are active.
function CategoryChart({
  category,
  items,
  maxItems,
  officialNames,
}: {
  category: string;
  items: CustomizationDetail[];
  maxItems: number;
  officialNames: Set<string>;
}) {
  const top = sortCustomizationsByScope(items, "all").slice(0, maxItems);

  const options = {
    ...commonHorizontalBarOptions,
    plugins: {
      ...commonHorizontalBarOptions.plugins,
      legend: bottomLegend,
      tooltip: {
        ...commonHorizontalBarOptions.plugins.tooltip,
        filter: (ctx: { raw: unknown }) => ctx.raw !== null,
        callbacks: {
          label: (ctx: { dataIndex: number; dataset: { label?: string } }) => {
            const item = top[ctx.dataIndex];
            return `${ctx.dataset.label}: ${item.repo_count} repoer, ${item.active_repo_count} av dem aktive`;
          },
        },
      },
    },
    scales: {
      ...commonHorizontalBarOptions.scales,
      x: { ...commonHorizontalBarOptions.scales.x, stacked: true },
      y: { ...commonHorizontalBarOptions.scales.y, stacked: true },
    },
  };

  return (
    <Box padding="space-16" borderRadius="8" borderWidth="1" borderColor="neutral-subtle" background="default">
      <Heading size="small" level="3" spacing>
        {categoryLabels[category] ?? category}
      </Heading>
      <div style={{ height: Math.max(200, top.length * 28 + 60) }}>
        <Bar
          data={{ labels: top.map((item) => item.file_name), datasets: originDatasets(top, officialNames) }}
          options={options}
        />
      </div>
    </Box>
  );
}

const TopCustomizationsChart: React.FC<TopCustomizationsChartProps> = ({ data, maxItems = 10 }) => {
  const officialNames = useMemo(() => getOfficialFileNames(), []);

  const grouped = data.reduce<Record<string, CustomizationDetail[]>>((acc, item) => {
    (acc[item.category] ??= []).push(item);
    return acc;
  }, {});

  const categories = Object.keys(categoryLabels).filter((cat) => (grouped[cat] ?? []).length > 0);

  if (categories.length === 0) return <BodyShort>Ingen tilpasninger ennå.</BodyShort>;
  return (
    <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
      {categories.map((category) => (
        <CategoryChart
          key={category}
          category={category}
          items={grouped[category]}
          maxItems={maxItems}
          officialNames={officialNames}
        />
      ))}
    </HGrid>
  );
};

export default TopCustomizationsChart;
