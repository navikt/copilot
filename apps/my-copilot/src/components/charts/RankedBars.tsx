"use client";

import { Bar } from "react-chartjs-2";
import type { BarElement, Plugin } from "chart.js";
import { BodyShort, Heading, VStack } from "@navikt/ds-react";
import { axColor, commonHorizontalBarOptions, seriesColor } from "@/lib/chart-utils";
import { ChartFigure } from "@/components/charts/chart-figure";
import { formatNumber } from "@/lib/format";

/** `value: null` is unknown, drawn as «Ukjent» and never as a bar of zero. */
export interface RankedItem {
  label: string;
  value: number | null;
  /** Shown after the number, e.g. the source of a skill. */
  note?: string;
}

interface ValueLabelOptions {
  texts: string[];
}

const UNKNOWN = "Ukjent";

// Draws the number at the end of each bar, so nobody has to hover or read an axis.
const valueLabels: Plugin<"bar", ValueLabelOptions> = {
  id: "valueLabels",
  afterDatasetsDraw(chart, _args, { texts }) {
    const { ctx } = chart;
    const bars = chart.getDatasetMeta(0).data as BarElement[];
    ctx.save();
    ctx.textBaseline = "middle";
    ctx.textAlign = "left";
    ctx.font = "600 12px sans-serif";
    ctx.fillStyle = axColor("text-neutral");
    texts.forEach((text, i) => {
      const bar = bars[i];
      const x = bar && !text.startsWith(UNKNOWN) ? bar.x : chart.scales.x.getPixelForValue(0);
      ctx.fillText(text, x + 6, bar?.y ?? 0);
    });
    ctx.restore();
  },
};

/**
 * One metric as horizontal bars in the order given, each with its number at the bar end and a visible takeaway above.
 * Pass the same `max` to panels that share a unit; leave it out when the metrics differ, so each scale is its own.
 */
export default function RankedBars({
  title,
  takeaway,
  summary,
  items,
  color = 0,
  max,
  rows,
}: {
  title: string;
  /** One sentence the numbers support. Visible; `summary` is the screen reader text. */
  takeaway: string;
  summary: string;
  items: RankedItem[];
  color?: number;
  max?: number;
  /** The full numbers for the fold, header first. */
  rows: string[][];
}) {
  const texts = items.map((i) => (i.value == null ? UNKNOWN : formatNumber(i.value)) + (i.note ? ` · ${i.note}` : ""));
  const options = {
    ...commonHorizontalBarOptions,
    layout: { padding: { right: items.some((i) => i.note) ? 120 : 56 } },
    plugins: {
      ...commonHorizontalBarOptions.plugins,
      valueLabels: { texts },
      tooltip: { ...commonHorizontalBarOptions.plugins.tooltip, enabled: false },
    },
    scales: {
      ...commonHorizontalBarOptions.scales,
      y: {
        ...commonHorizontalBarOptions.scales.y,
        ticks: {
          ...commonHorizontalBarOptions.scales.y.ticks,
          font: { size: 13 },
          color: () => axColor("text-neutral"),
        },
      },
      x: { ...commonHorizontalBarOptions.scales.x, display: false, ...(max ? { max } : {}) },
    },
  };
  return (
    <VStack gap="space-8" className="min-w-0">
      <Heading level="3" size="xsmall">
        {title}
      </Heading>
      <BodyShort size="small" textColor="subtle">
        {takeaway}
      </BodyShort>
      <ChartFigure
        summary={summary}
        height={Math.max(96, items.length * 32 + 16)}
        table={rows}
        chart={
          <Bar
            data={{
              labels: items.map((i) => i.label),
              datasets: [
                {
                  data: items.map((i) => i.value),
                  backgroundColor: seriesColor(color),
                  borderRadius: 4,
                  barThickness: 16,
                },
              ],
            }}
            options={options}
            plugins={[valueLabels]}
          />
        }
      />
    </VStack>
  );
}
