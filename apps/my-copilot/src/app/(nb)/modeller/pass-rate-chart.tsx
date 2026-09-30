"use client";

import { Scatter } from "react-chartjs-2";
import type { TooltipItem } from "chart.js";
// Registers the Chart.js scales and elements the site uses.
import "@/lib/chart-utils";

export interface ChartPoint {
  model: string;
  effort: string;
  credits: number;
  passPercent: number;
  n: number;
}

// Categorical slots from the dataviz reference palette, in fixed order. The
// first four pass every check for all pairs on a scatter; slots five and six
// are below 3:1 contrast, so the table under the chart carries the same data.
const SERIES = ["#2a78d6", "#eb6834", "#1baf7a", "#4a3aa7", "#eda100", "#e87ba4"];
// Shape carries effort, so identity is never colour alone.
const EFFORT_SHAPE: Record<string, "circle" | "rect" | "triangle" | "rectRot"> = {
  low: "circle",
  medium: "rect",
  high: "triangle",
  default: "rectRot",
};

/** Pass rate against median credits, one colour per model (fixed by `models` order), one shape per effort. */
export function PassRateChart({ points, models, title }: { points: ChartPoint[]; models: string[]; title: string }) {
  const datasets = models
    .filter((model) => points.some((p) => p.model === model))
    .map((model) => {
      const color = SERIES[models.indexOf(model) % SERIES.length];
      const own = points.filter((p) => p.model === model);
      return {
        label: model,
        data: own.map((p) => ({ x: p.credits, y: p.passPercent, effort: p.effort, n: p.n })),
        pointStyle: own.map((p) => EFFORT_SHAPE[p.effort] ?? "circle"),
        backgroundColor: color,
        borderColor: "#ffffff",
        borderWidth: 2,
        pointRadius: 7,
        pointHoverRadius: 9,
        pointHitRadius: 12,
      };
    });

  return (
    <figure
      className="bg-white rounded-lg border border-gray-200"
      style={{ padding: "var(--ax-space-16)" }}
      aria-label={title}
    >
      <div style={{ position: "relative", height: "20rem" }}>
        <Scatter
          aria-label={title}
          role="img"
          data={{ datasets }}
          options={{
            responsive: true,
            maintainAspectRatio: false,
            // Points on 100 % or the largest credit value would otherwise be cut in half.
            clip: false,
            layout: { padding: 12 },
            plugins: {
              legend: { position: "top", labels: { usePointStyle: true, pointStyle: "circle", color: "#475569" } },
              tooltip: {
                callbacks: {
                  label: (ctx: TooltipItem<"scatter">) => {
                    const raw = ctx.raw as { x: number; y: number; effort: string; n: number };
                    return `${ctx.dataset.label} (${raw.effort}): ${Math.round(raw.y)} % bestått, ${raw.x.toLocaleString("nb-NO")} credits, n=${raw.n}`;
                  },
                },
              },
            },
            scales: {
              x: {
                beginAtZero: true,
                grace: "10%",
                title: { display: true, text: "Median credits per kjøring", color: "#475569" },
                grid: { color: "rgba(0, 0, 0, 0.06)" },
                ticks: { color: "#6B7280" },
              },
              y: {
                min: 0,
                max: 100,
                title: { display: true, text: "Bestått (%)", color: "#475569" },
                grid: { color: "rgba(0, 0, 0, 0.06)" },
                ticks: { color: "#6B7280" },
              },
            },
          }}
        />
      </div>
      <figcaption className="text-sm" style={{ color: "#475569", marginTop: "var(--ax-space-8)" }}>
        Form viser effort: sirkel = low, kvadrat = medium, trekant = high, rombe = standard. Tallene står i tabellen
        under.
      </figcaption>
    </figure>
  );
}
