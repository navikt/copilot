"use client";

import { Scatter } from "react-chartjs-2";
import type { ChartDataset, TooltipItem } from "chart.js";
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
// are below 3:1 contrast, and the seventh (brown) is told apart from the rest by
// hue, not contrast, so the table under the chart carries the same data.
const SERIES = ["#2a78d6", "#eb6834", "#1baf7a", "#4a3aa7", "#eda100", "#e87ba4", "#8a5a2b"];
// Shape carries effort, so identity is never colour alone.
const EFFORT_SHAPE: Record<string, "circle" | "rect" | "triangle" | "rectRot"> = {
  low: "circle",
  medium: "rect",
  high: "triangle",
  default: "rectRot",
};

/** The points where no other run is both cheaper and at least as good, cheapest first. */
export function paretoFrontier<T extends { credits: number; passPercent: number }>(points: T[]): T[] {
  let best = -Infinity;
  return [...points]
    .sort((a, b) => a.credits - b.credits || b.passPercent - a.passPercent)
    .filter((p) => p.passPercent > best && ((best = p.passPercent), true));
}

/** Pass rate against median credits, one colour per model (fixed by `models` order), one shape per effort. */
export function PassRateChart({
  points,
  models,
  title,
  frontier = false,
}: {
  points: ChartPoint[];
  models: string[];
  title: string;
  frontier?: boolean;
}) {
  const datasets: ChartDataset<"scatter">[] = models
    .filter((model) => points.some((p) => p.model === model))
    .map((model) => {
      const color = SERIES[models.indexOf(model) % SERIES.length];
      // Line runs cheapest → dearest, so it never doubles back on the log axis.
      const own = points.filter((p) => p.model === model).sort((a, b) => a.credits - b.credits);
      return {
        label: model,
        data: own.map((p) => ({ x: p.credits, y: p.passPercent, effort: p.effort, n: p.n })),
        pointStyle: own.map((p) => EFFORT_SHAPE[p.effort] ?? "circle"),
        backgroundColor: color,
        borderColor: color,
        borderWidth: 1.5,
        pointBorderColor: "#ffffff",
        pointBorderWidth: 2,
        showLine: true,
        pointRadius: 7,
        pointHoverRadius: 9,
        pointHitRadius: 12,
      };
    });
  if (frontier) {
    datasets.push({
      label: "Beste avveining",
      data: paretoFrontier(points).map((p) => ({ x: p.credits, y: p.passPercent, effort: p.effort, n: p.n })),
      showLine: true,
      borderColor: "#475569",
      borderDash: [6, 4],
      borderWidth: 2,
      // Rings around the frontier points, so a one-point frontier still shows.
      pointRadius: 11,
      pointHoverRadius: 11,
      pointHitRadius: 0,
      pointBackgroundColor: "transparent",
      pointBorderColor: "#475569",
      pointBorderWidth: 1.5,
      backgroundColor: "#475569",
    });
  }

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
            // Static render, so screenshots are deterministic.
            animation: false,
            // Points on 100 % or the largest credit value would otherwise be cut in half.
            clip: false,
            layout: { padding: 12 },
            plugins: {
              legend: { position: "top", labels: { usePointStyle: true, pointStyle: "circle", color: "#475569" } },
              tooltip: {
                filter: (item) => item.dataset.label !== "Beste avveining",
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
                // Log scale: 1.6 and 154 credits both stay readable.
                type: "logarithmic",
                // Whole decades, so the cheapest and dearest points sit clear of the edges.
                min: 10 ** Math.floor(Math.log10(Math.min(...points.map((p) => p.credits)))),
                max: 10 ** Math.ceil(Math.log10(Math.max(...points.map((p) => p.credits)))),
                title: { display: true, text: "Median credits per kjøring", color: "#475569" },
                grid: { color: "rgba(0, 0, 0, 0.06)" },
                ticks: {
                  color: "#6B7280",
                  // Only 1-2-5 steps, so the axis is not crowded.
                  callback: (value) => {
                    const v = Number(value);
                    const lead = v / 10 ** Math.floor(Math.log10(v));
                    return [1, 2, 5].some((d) => Math.abs(lead - d) < 1e-9) ? v.toLocaleString("nb-NO") : "";
                  },
                },
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
        Form viser innsatsnivå (effort): sirkel = low, kvadrat = medium, trekant = high, rombe = standard. Linjen binder
        sammen nivåene for hver modell. Credits-aksen er logaritmisk.
        {frontier &&
          " Ringene og den stiplede linjen viser punktene der ingen annen kjøring er både billigere og minst like god."}{" "}
        Tallene står i tabellen under.
      </figcaption>
    </figure>
  );
}
