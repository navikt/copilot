"use client";

import type { CustomizationDetail, AdoptionScope } from "@/lib/types";
import React, { useMemo, useState } from "react";
import { Bar } from "react-chartjs-2";
import { chartColors, commonHorizontalBarOptions, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { Box, Heading, BodyShort, HGrid, HStack, VStack, Chips, ToggleGroup } from "@navikt/ds-react";
import { getOfficialFileNames } from "@/lib/customizations";
import { getCustomizationRepoCount, sortCustomizationsByScope } from "@/lib/adoption-utils";

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

const categoryColors: Record<string, string> = {
  agents: chartColors[0],
  skills: chartColors[1],
  instructions: chartColors[2],
  prompts: chartColors[3],
  agentic_workflows: chartColors[4],
  agents_skills: chartColors[5],
};

function CategoryChart({
  category,
  items,
  maxItems,
  scope,
}: {
  category: string;
  items: CustomizationDetail[];
  maxItems: number;
  scope: AdoptionScope;
}) {
  const sorted = sortCustomizationsByScope(items, scope);
  const top = sorted.slice(0, maxItems);
  const color = categoryColors[category] ?? chartColors[5];

  const chartData = {
    labels: top.map((item) => item.file_name),
    datasets: [
      {
        data: top.map((item) => getCustomizationRepoCount(item, scope)),
        backgroundColor: color,
        borderRadius: 4,
        barThickness: 18,
      },
    ],
  };

  const height = Math.max(150, top.length * 32);

  return (
    <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
      <Heading size="small" level="4" spacing>
        {categoryLabels[category] ?? category}
      </Heading>
      {top.length === 0 ? (
        <BodyShort className="text-gray-500">Ingen data</BodyShort>
      ) : (
        <div style={{ height }}>
          <Bar data={chartData} options={commonHorizontalBarOptions} />
        </div>
      )}
    </Box>
  );
}

const TopCustomizationsChart: React.FC<TopCustomizationsChartProps> = ({ data, maxItems = 10 }) => {
  const [originFilter, setOriginFilter] = useState<OriginFilter>("official");
  const [scope, setScope] = useState<AdoptionScope>("active");
  const officialNames = useMemo(() => getOfficialFileNames(), []);

  const filteredData = useMemo(() => {
    if (originFilter === "all") return data;
    return data.filter((item) => {
      const isOfficial = officialNames.has(item.file_name);
      return originFilter === "official" ? isOfficial : !isOfficial;
    });
  }, [data, originFilter, officialNames]);

  if (!data || data.length === 0) {
    return (
      <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
        <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>
      </Box>
    );
  }

  const grouped = filteredData.reduce<Record<string, CustomizationDetail[]>>((acc, item) => {
    (acc[item.category] ??= []).push(item);
    return acc;
  }, {});

  const categories = ["agents", "skills", "instructions", "prompts", "agentic_workflows", "agents_skills"]
    .filter((cat) => (grouped[cat] ?? []).length > 0);

  return (
    <VStack gap="space-16">
      <HStack gap="space-16" align="center" justify="space-between" wrap>
        <HStack gap="space-8" align="center">
          <BodyShort size="small" className="text-gray-500">
            Opprinnelse:
          </BodyShort>
          <Chips size="small">
            <Chips.Toggle selected={originFilter === "all"} onClick={() => setOriginFilter("all")}>
              Alle
            </Chips.Toggle>
            <Chips.Toggle selected={originFilter === "official"} onClick={() => setOriginFilter("official")}>
              Offisielle
            </Chips.Toggle>
            <Chips.Toggle selected={originFilter === "custom"} onClick={() => setOriginFilter("custom")}>
              Egne
            </Chips.Toggle>
          </Chips>
        </HStack>
        <ToggleGroup size="small" value={scope} onChange={(val) => setScope(val as AdoptionScope)}>
          <ToggleGroup.Item value="active">Aktive repoer</ToggleGroup.Item>
          <ToggleGroup.Item value="all">Alle repoer</ToggleGroup.Item>
        </ToggleGroup>
      </HStack>
      <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
        {categories.map((category) => (
          <CategoryChart
            key={category}
            category={category}
            items={grouped[category] ?? []}
            maxItems={maxItems}
            scope={scope}
          />
        ))}
      </HGrid>
    </VStack>
  );
};

export default TopCustomizationsChart;
