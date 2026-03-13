"use client";

import type { CustomizationDetail } from "@/lib/types";
import React, { useMemo, useState } from "react";
import { Bar } from "react-chartjs-2";
import { chartColors, commonHorizontalBarOptions, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { Box, Heading, BodyShort, HGrid, HStack, Chips } from "@navikt/ds-react";
import { getOfficialFileNames } from "@/lib/customizations";

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
};

const categoryColors: Record<string, string> = {
  agents: chartColors[0],
  skills: chartColors[1],
  instructions: chartColors[2],
  prompts: chartColors[3],
};

function CategoryChart({
  category,
  items,
  maxItems,
}: {
  category: string;
  items: CustomizationDetail[];
  maxItems: number;
}) {
  const top = items.slice(0, maxItems);
  const color = categoryColors[category] ?? chartColors[5];

  const chartData = {
    labels: top.map((item) => item.file_name),
    datasets: [
      {
        data: top.map((item) => item.repo_count),
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
  const [originFilter, setOriginFilter] = useState<OriginFilter>("all");
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

  const categories = ["agents", "skills", "instructions", "prompts"];

  return (
    <div>
      <HStack gap="space-8" align="center" className="mb-[--a-spacing-16]">
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
      <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
        {categories.map((category) => (
          <CategoryChart key={category} category={category} items={grouped[category] ?? []} maxItems={maxItems} />
        ))}
      </HGrid>
    </div>
  );
};

export default TopCustomizationsChart;
