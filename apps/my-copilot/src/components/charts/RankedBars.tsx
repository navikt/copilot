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
  /** A second line under the label, e.g. the source of a skill. */
  note?: string;
}

interface ValueLabelOptions {
  texts: string[];
}

const UNKNOWN = "Ukjent";
const VALUE_FONT = "600 12px sans-serif";
const CHAR_PX = 7;

// Axis labels are right-aligned, so text wider than the axis is clipped on the left. Cut to what fits.
const cut = (text: string, axisPx: number) => {
  const max = Math.max(6, Math.floor(axisPx / CHAR_PX));
  return text.length > max ? `${text.slice(0, max - 1)}…` : text;
};

const axisWidth = (chartWidth: number) => Math.min(140, Math.round(chartWidth * 0.4));

// Draws the number at the end of each bar, so nobody has to hover or read an axis. The right padding is measured from
// the actual texts, so seven-digit counts are not clipped and short ones do not waste space.
const valueLabels: Plugin<"bar", ValueLabelOptions> = {
  id: "valueLabels",
  beforeLayout(chart, _args, { texts }) {
    chart.ctx.save();
    chart.ctx.font = VALUE_FONT;
    const widest = Math.max(0, ...texts.map((t) => chart.ctx.measureText(t).width));
    chart.ctx.restore();
    chart.options.layout = { ...chart.options.layout, padding: { right: Math.ceil(widest) + 12 } };
  },
  afterDatasetsDraw(chart, _args, { texts }) {
    const { ctx } = chart;
    const bars = chart.getDatasetMeta(0).data as BarElement[];
    ctx.save();
    ctx.textBaseline = "middle";
    ctx.textAlign = "left";
    ctx.font = VALUE_FONT;
    ctx.fillStyle = axColor("text-neutral");
    texts.forEach((text, i) => {
      const bar = bars[i];
      const x = bar && text !== UNKNOWN ? bar.x : chart.scales.x.getPixelForValue(0);
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
  const texts = items.map((i) => (i.value == null ? UNKNOWN : formatNumber(i.value)));
  const twoLines = items.some((i) => i.note);
  const shown = rows.length - 1 > items.length ? items.length : null;
  const options = {
    ...commonHorizontalBarOptions,
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
          font: { size: twoLines ? 12 : 13 },
          color: () => axColor("text-neutral"),
          callback(
            this: { chart: { width: number }; getLabelForValue: (v: number) => string | string[] },
            value: string | number
          ) {
            const raw = this.getLabelForValue(Number(value));
            const px = axisWidth(this.chart.width) - 8;
            return Array.isArray(raw) ? raw.map((r) => cut(r, px)) : cut(raw, px);
          },
        },
        // Never more than 40 % of the chart, so the bars keep their room on a phone.
        afterFit: (axis: { width: number; chart: { width: number } }) => {
          axis.width = Math.min(axis.width, axisWidth(axis.chart.width));
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
        height={Math.max(96, items.length * (twoLines ? 44 : 32) + 16)}
        tableLabel={shown ? `Vis alle ${rows.length - 1} rader, diagrammet viser de ${shown} øverste` : undefined}
        table={rows}
        chart={
          <Bar
            data={{
              labels: items.map((i) => (i.note ? [i.label, i.note] : i.label)),
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
