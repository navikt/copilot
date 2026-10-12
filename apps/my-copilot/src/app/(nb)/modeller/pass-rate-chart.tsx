"use client";

import { useEffect, useState } from "react";
import { Scatter } from "react-chartjs-2";
import type { Chart, ChartDataset, Plugin, TooltipItem } from "chart.js";
// Registers the Chart.js scales and elements the site uses.
import "@/lib/chart-utils";

export interface ChartPoint {
  model: string;
  effort: string;
  credits: number;
  passPercent: number;
  /** Wilson 95 % interval in percent. */
  low: number;
  high: number;
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
const FRONTIER = "Beste avveining";
// Below this width the direct labels have no room, so the legend takes over.
const LABEL_MIN_WIDTH = 640;
// With this many repeats the interval is narrow enough that the bar adds nothing.
const CI_MAX_N = 30;

/** The points where no other run is both cheaper and at least as good, cheapest first. */
export function paretoFrontier<T extends { credits: number; passPercent: number }>(points: T[]): T[] {
  let best = -Infinity;
  return [...points]
    .sort((a, b) => a.credits - b.credits || b.passPercent - a.passPercent)
    .filter((p) => p.passPercent > best && ((best = p.passPercent), true));
}

/** Spreads label centres so no two are closer than gap, keeping their order. */
export function spreadLabels(ys: number[], gap: number): number[] {
  const order = ys.map((y, i) => [y, i] as const).sort((a, b) => a[0] - b[0]);
  const out = [...ys];
  let prev = -Infinity;
  for (const [y, i] of order) prev = out[i] = Math.max(y, prev + gap);
  return out;
}

const fade = (hex: string, on: boolean) => (on ? hex : `${hex}33`);
type Raw = { x: number; y: number; effort: string; n: number; low: number; high: number };

/** Faint interval bars and, when wide enough, model names at the dearest end of each line. */
const overlay: Plugin<"scatter"> = {
  id: "passOverlay",
  afterDatasetsDraw(chart: Chart<"scatter">) {
    const opts = (chart.options.plugins as unknown as { passOverlay: OverlayOptions }).passOverlay;
    const { ctx, scales } = chart;
    const labels: { y: number; x: number; text: string; color: string }[] = [];
    chart.data.datasets.forEach((ds, i) => {
      if (ds.label === FRONTIER || !chart.isDatasetVisible(i)) return;
      const on = !opts.highlight || opts.highlight === ds.label;
      const color = fade(String(ds.borderColor).slice(0, 7), on);
      const data = ds.data as unknown as Raw[];
      if (opts.ci || (opts.highlight === ds.label && data.length)) {
        ctx.save();
        ctx.strokeStyle = color;
        ctx.globalAlpha = 0.45;
        ctx.lineWidth = 2;
        for (const p of data) {
          if (p.n >= CI_MAX_N) continue;
          const x = scales.x.getPixelForValue(p.x);
          ctx.beginPath();
          ctx.moveTo(x, scales.y.getPixelForValue(p.low));
          ctx.lineTo(x, scales.y.getPixelForValue(p.high));
          ctx.stroke();
        }
        ctx.restore();
      }
      const last = data.at(-1);
      if (opts.labels && last) {
        labels.push({
          x: scales.x.getPixelForValue(last.x) + 12,
          y: scales.y.getPixelForValue(last.y),
          text: String(ds.label),
          color: on ? "#1f2937" : "#9ca3af",
        });
      }
    });
    const ys = spreadLabels(
      labels.map((l) => l.y),
      13
    );
    ctx.save();
    ctx.font = "12px sans-serif";
    ctx.textBaseline = "middle";
    labels.forEach((l, i) => {
      ctx.fillStyle = l.color;
      ctx.fillText(l.text, l.x, ys[i]);
    });
    ctx.restore();
  },
};
interface OverlayOptions {
  ci: boolean;
  labels: boolean;
  highlight: string | null;
}

/** Pass rate against median credits, one colour per model (fixed by `models` order), one shape per effort. */
export function PassRateChart({
  points,
  models,
  title,
  xRange,
  frontier = false,
  ci = false,
  highlight: selected = null,
}: {
  points: ChartPoint[];
  models: string[];
  title: string;
  /** Shared across the suites, so the charts can be compared. */
  xRange: [number, number];
  frontier?: boolean;
  ci?: boolean;
  /** Model picked in «Fremhev modell»; hover overrides it while the pointer is on a point. */
  highlight?: string | null;
}) {
  const [hovered, setHovered] = useState<string | null>(null);
  const highlight = hovered ?? selected;
  const [wide, setWide] = useState(true);
  useEffect(() => {
    const query = window.matchMedia(`(min-width: ${LABEL_MIN_WIDTH}px)`);
    const update = () => setWide(query.matches);
    update();
    query.addEventListener("change", update);
    return () => query.removeEventListener("change", update);
  }, []);

  const datasets: ChartDataset<"scatter">[] = models
    .filter((model) => points.some((p) => p.model === model))
    .map((model) => {
      const base = SERIES[models.indexOf(model) % SERIES.length];
      const color = fade(base, !highlight || highlight === model);
      // Line runs cheapest → dearest, so it never doubles back on the log axis.
      const own = points.filter((p) => p.model === model).sort((a, b) => a.credits - b.credits);
      return {
        label: model,
        data: own.map((p) => ({ x: p.credits, y: p.passPercent, effort: p.effort, n: p.n, low: p.low, high: p.high })),
        pointStyle: own.map((p) => EFFORT_SHAPE[p.effort] ?? "circle"),
        backgroundColor: color,
        borderColor: color,
        borderWidth: highlight === model ? 2.5 : 1.5,
        pointBorderColor: "#ffffff",
        pointBorderWidth: 2,
        showLine: true,
        pointRadius: 7,
        pointHoverRadius: 9,
        pointHitRadius: 12,
        order: highlight === model ? 0 : 1,
      };
    });
  if (frontier) {
    datasets.push({
      label: FRONTIER,
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
  const overlayOptions: OverlayOptions = { ci, labels: wide, highlight };

  return (
    <figure
      className="bg-white rounded-lg border border-gray-200"
      style={{ padding: "var(--ax-space-16)" }}
      aria-label={title}
    >
      <div style={{ position: "relative", height: "20rem" }} onMouseLeave={() => setHovered(null)}>
        <Scatter
          aria-label={title}
          role="img"
          data={{ datasets }}
          plugins={[overlay]}
          options={{
            responsive: true,
            maintainAspectRatio: false,
            // Static render, so screenshots are deterministic.
            animation: false,
            // Points on 100 % or the largest credit value would otherwise be cut in half.
            clip: false,
            // Room on the right for the direct labels.
            layout: { padding: { top: 12, bottom: 12, left: 12, right: wide ? 120 : 12 } },
            onHover: (_event, elements, chart) => {
              const label = elements.length ? chart.data.datasets[elements[0].datasetIndex].label : null;
              setHovered(label === FRONTIER ? null : (label ?? null));
            },
            plugins: {
              // @ts-expect-error: options for the local overlay plugin above.
              passOverlay: overlayOptions,
              legend: {
                display: !wide,
                position: "top",
                labels: { usePointStyle: true, pointStyle: "circle", color: "#475569" },
              },
              tooltip: {
                filter: (item) => item.dataset.label !== FRONTIER,
                callbacks: {
                  label: (ctx: TooltipItem<"scatter">) => {
                    const raw = ctx.raw as Raw;
                    return `${ctx.dataset.label} (${raw.effort}): ${Math.round(raw.y)} % bestått (95 %-intervall ${Math.round(raw.low)}–${Math.round(raw.high)} %), ${raw.x.toLocaleString("nb-NO")} credits, n=${raw.n}`;
                  },
                },
              },
            },
            scales: {
              x: {
                // Log scale: 1.6 and 154 credits both stay readable.
                type: "logarithmic",
                min: xRange[0],
                max: xRange[1],
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
        Slik leser du diagrammet: jo høyere og lenger til venstre et punkt står, jo flere sjekker besto modellen og jo
        mindre kostet den. Form viser innsatsnivå (effort): sirkel = low, kvadrat = medium, trekant = high, rombe =
        standard. Linjen binder sammen nivåene for hver modell. Credits-aksen er logaritmisk og lik i alle diagrammene.
        {ci && " De svake loddrette strekene viser 95 %-intervallet for andelen bestått."}
        {frontier &&
          " Ringene og den stiplede linjen viser punktene der ingen annen kjøring er både billigere og minst like god."}{" "}
        Tallene står i tabellen under.
      </figcaption>
    </figure>
  );
}
