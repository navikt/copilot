"use client";

import type { CustomizationDetail } from "@/lib/types";
import React, { useMemo, useState } from "react";
import { Bar } from "react-chartjs-2";
import { chartColors, commonHorizontalBarOptions } from "@/lib/chart-utils";
import { Box, Heading, BodyShort, HGrid, HStack, VStack, Chips } from "@navikt/ds-react";
import { getOfficialFileNames } from "@/lib/customizations";
import { sortCustomizationsByScope } from "@/lib/adoption-utils";

type OriginFilter = "all" | "official" | "custom";

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

const options = {
  ...commonHorizontalBarOptions,
  plugins: { ...commonHorizontalBarOptions.plugins, legend: { display: true, position: "bottom" as const } },
};

// Each file shows how many repos have it, and how many of those are active, side by side.
function CategoryChart({
  category,
  items,
  maxItems,
}: {
  category: string;
  items: CustomizationDetail[];
  maxItems: number;
}) {
  const top = sortCustomizationsByScope(items, "all").slice(0, maxItems);

  const chartData = {
    labels: top.map((item) => item.file_name),
    datasets: [
      {
        label: "Alle repoer",
        data: top.map((item) => item.repo_count),
        backgroundColor: chartColors[0],
        borderRadius: 4,
        barThickness: 10,
      },
      {
        label: "Aktive repoer",
        data: top.map((item) => item.active_repo_count),
        backgroundColor: chartColors[1],
        borderRadius: 4,
        barThickness: 10,
      },
    ],
  };

  return (
    <Box padding="space-16" borderRadius="8" borderWidth="1" borderColor="neutral-subtle" background="default">
      <Heading size="small" level="3" spacing>
        {categoryLabels[category] ?? category}
      </Heading>
      <div style={{ height: Math.max(180, top.length * 32 + 40) }}>
        <Bar data={chartData} options={options} />
      </div>
    </Box>
  );
}

const TopCustomizationsChart: React.FC<TopCustomizationsChartProps> = ({ data, maxItems = 10 }) => {
  const [originFilter, setOriginFilter] = useState<OriginFilter>("official");
  const officialNames = useMemo(() => getOfficialFileNames(), []);

  const filteredData =
    originFilter === "all"
      ? data
      : data.filter((item) => officialNames.has(item.file_name) === (originFilter === "official"));

  const grouped = filteredData.reduce<Record<string, CustomizationDetail[]>>((acc, item) => {
    (acc[item.category] ??= []).push(item);
    return acc;
  }, {});

  const categories = Object.keys(categoryLabels).filter((cat) => (grouped[cat] ?? []).length > 0);

  return (
    <VStack gap="space-16">
      <HStack gap="space-8" align="center">
        <BodyShort size="small" textColor="subtle" id="opprinnelse">
          Opprinnelse:
        </BodyShort>
        <Chips size="small" aria-labelledby="opprinnelse">
          <Chips.Toggle selected={originFilter === "all"} onClick={() => setOriginFilter("all")}>
            Alle
          </Chips.Toggle>
          <Chips.Toggle selected={originFilter === "official"} onClick={() => setOriginFilter("official")}>
            Fra navikt/copilot
          </Chips.Toggle>
          <Chips.Toggle selected={originFilter === "custom"} onClick={() => setOriginFilter("custom")}>
            Egne
          </Chips.Toggle>
        </Chips>
      </HStack>
      {categories.length === 0 ? (
        <BodyShort>Ingen tilpasninger passer filteret.</BodyShort>
      ) : (
        <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
          {categories.map((category) => (
            <CategoryChart key={category} category={category} items={grouped[category]} maxItems={maxItems} />
          ))}
        </HGrid>
      )}
    </VStack>
  );
};

export default TopCustomizationsChart;
