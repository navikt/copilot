import type { ChartType, Plugin } from "chart.js";
import { axColor, bottomLegend } from "@/lib/chart-utils";
import type { MonthAnnotation } from "@/lib/trends";

/**
 * Tooltip footer naming the events under the finger: numbered as in a «Hendelser» list, or the full label
 * when the marker carries its own short text. Leaves room above the plot for the marker row(s).
 */
export function withEvents<
  T extends { plugins: { tooltip: object & { callbacks?: object } }; layout?: { padding?: object } },
>(options: T, keys: string[], annotations: MonthAnnotation[]) {
  return {
    ...options,
    layout: { padding: { top: annotations.some((a) => a.text) ? 34 : 18 } },
    plugins: {
      ...options.plugins,
      // The legend goes below, so the marker row above the plot area stays free.
      legend: bottomLegend,
      tooltip: {
        ...options.plugins.tooltip,
        callbacks: {
          ...options.plugins.tooltip.callbacks,
          footer: (items: { dataIndex: number }[]) => {
            const a = annotations.find((x) => x.month === keys[items[0]?.dataIndex]);
            if (!a) return [];
            return a.text ? a.labels : a.labels.map((l, i) => `${a.numbers[i]}. ${l}`);
          },
        },
      },
    },
  };
}

/**
 * Small markers above the plot area, never over the data: a number, or `text` when set. `keys` are the
 * category labels' keys (months, or days). A data break also gets a thin grey band on the boundary, and
 * `shadeBefore` greys the categories before it. Text markers that would overlap move up a row.
 */
export function eventMarkers<T extends ChartType>(
  keys: string[],
  annotations: MonthAnnotation[],
  shadeBefore?: string
): Plugin<T> {
  return {
    id: "monthAnnotations",
    beforeDatasetsDraw(chart) {
      const { ctx, chartArea, scales } = chart;
      const step = keys.length > 1 ? scales.x.getPixelForValue(1) - scales.x.getPixelForValue(0) : chartArea.width;
      const boundary = (key: string) => scales.x.getPixelForValue(keys.indexOf(key)) - step / 2;
      ctx.save();
      for (const a of annotations) {
        if (!a.dataBreak || keys.indexOf(a.month) <= 0) continue;
        ctx.fillStyle = axColor("neutral-500", 0.35);
        ctx.fillRect(boundary(a.month) - 2, chartArea.top, 4, chartArea.height);
      }
      ctx.restore();
    },
    afterDatasetsDraw(chart) {
      const { ctx, chartArea, scales } = chart;
      ctx.save();
      // Wash out the months before the break, so they read as a separate period, labelled in the marker row.
      if (shadeBefore && keys.indexOf(shadeBefore) > 0) {
        const step = keys.length > 1 ? scales.x.getPixelForValue(1) - scales.x.getPixelForValue(0) : 0;
        const x = scales.x.getPixelForValue(keys.indexOf(shadeBefore)) - step / 2;
        ctx.fillStyle = axColor("bg-default", 0.55);
        ctx.fillRect(chartArea.left, chartArea.top, x - chartArea.left, chartArea.height);
        ctx.fillStyle = axColor("text-neutral-subtle");
        ctx.font = "11px sans-serif";
        ctx.textBaseline = "middle";
        const text = "Før API-bytte";
        if (ctx.measureText(text).width + 8 < x - chartArea.left)
          ctx.fillText(text, chartArea.left + 2, chartArea.top - 9);
      }
      ctx.font = "bold 10px sans-serif";
      ctx.textAlign = "center";
      ctx.textBaseline = "middle";
      let lastRight = -Infinity;
      let row = 0;
      for (const a of [...annotations].sort((p, q) => keys.indexOf(p.month) - keys.indexOf(q.month))) {
        const index = keys.indexOf(a.month);
        if (index < 0) continue;
        const text =
          a.text ?? (a.numbers.length > 2 ? `${a.numbers[0]}–${a.numbers[a.numbers.length - 1]}` : a.numbers.join(","));
        const w = Math.max(14, ctx.measureText(text).width + 8);
        // Keep the pill inside the canvas, and step up a row when it would cover the one before.
        const x = Math.min(Math.max(scales.x.getPixelForValue(index), w / 2), chart.width - w / 2);
        row = a.text && x - w / 2 < lastRight + 2 ? 1 - row : 0;
        lastRight = x + w / 2;
        const y = chartArea.top - 9 - row * 16;
        // A text marker also gets a thin dashed line down to its place on the x-axis.
        if (a.text) {
          const lx = scales.x.getPixelForValue(index);
          ctx.save();
          ctx.strokeStyle = axColor("neutral-500", 0.6);
          ctx.setLineDash([3, 3]);
          ctx.beginPath();
          ctx.moveTo(lx, y + 7);
          ctx.lineTo(lx, chartArea.bottom);
          ctx.stroke();
          ctx.restore();
        }
        ctx.fillStyle = axColor(a.dataBreak ? "neutral-700" : "neutral-500");
        ctx.beginPath();
        ctx.roundRect(x - w / 2, y - 7, w, 14, 7);
        ctx.fill();
        ctx.fillStyle = axColor("text-neutral-contrast");
        ctx.fillText(text, x, y);
      }
      ctx.restore();
    },
  };
}
