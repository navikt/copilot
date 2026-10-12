"use client";

import { Bar, Line } from "react-chartjs-2";
import { commonLineOptions, seriesColor } from "@/lib/chart-utils";
import { formatPercent, formatUSD } from "@/lib/format";
import type { MonthAnnotation } from "@/lib/trends";
import { eventMarkers, withEvents } from "./chart-events";

export interface EventSeries {
  label: string;
  data: (number | null)[];
  /** Index into the series palette. */
  color: number;
  /** Per point, replaces the series label in the tooltip (a merged band, say). */
  pointLabels?: (string | null)[];
  dashed?: boolean;
  /** Starts switched off in the legend; the reader can turn it on. */
  hidden?: boolean;
}

/**
 * Bars (stacked when `stacked`) or lines over categories, with event markers above the plot. Props are plain
 * data so a server component can pass them. Wrap it in ChartFigure for the title, summary and table.
 */
export function EventChart({
  kind,
  keys,
  labels,
  series,
  unit,
  stacked = false,
  max,
  annotations = [],
}: {
  kind: "bar" | "line";
  /** Category keys the annotations refer to, in axis order. */
  keys: string[];
  labels: string[];
  series: EventSeries[];
  unit: "usd" | "pct";
  stacked?: boolean;
  max?: number;
  annotations?: MonthAnnotation[];
}) {
  const format = (v: number) => (unit === "usd" ? formatUSD(v) : formatPercent(v));
  const datasets = series.map((s) => ({
    label: s.label,
    data: s.data,
    borderColor: seriesColor(s.color),
    backgroundColor: seriesColor(s.color, kind === "bar" ? 0.8 : 1),
    borderWidth: kind === "bar" ? 0 : 2,
    pointRadius: kind === "line" ? 3 : 0,
    ...(s.dashed ? { borderDash: [6, 4] } : {}),
    hidden: s.hidden,
  }));
  const base = {
    ...commonLineOptions,
    scales: {
      x: { ...commonLineOptions.scales.x, stacked },
      y: {
        ...commonLineOptions.scales.y,
        stacked,
        ...(max ? { max } : {}),
        ticks: { ...commonLineOptions.scales.y.ticks, callback: (v: string | number) => format(Number(v)) },
      },
    },
    plugins: {
      ...commonLineOptions.plugins,
      tooltip: {
        ...commonLineOptions.plugins.tooltip,
        filter: (item: { raw: unknown }) => item.raw != null,
        callbacks: {
          label: (ctx: { datasetIndex: number; dataIndex: number; parsed: { y: number } }) => {
            const s = series[ctx.datasetIndex];
            return ` ${s.pointLabels?.[ctx.dataIndex] ?? s.label}: ${format(ctx.parsed.y)}`;
          },
        },
      },
    },
  };
  const options = withEvents(base, keys, annotations);
  if (kind === "line") {
    return (
      <Line
        data={{ labels, datasets }}
        options={{ ...options, spanGaps: false } as Parameters<typeof Line>[0]["options"]}
        plugins={[eventMarkers<"line">(keys, annotations)]}
      />
    );
  }
  return (
    <Bar
      data={{ labels, datasets }}
      options={options as Parameters<typeof Bar>[0]["options"]}
      plugins={[eventMarkers<"bar">(keys, annotations)]}
    />
  );
}
