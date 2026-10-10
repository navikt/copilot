"use client";

import { Bar, Line } from "react-chartjs-2";
import { axColor, bottomLegend, commonLineOptions } from "@/lib/chart-utils";
import { formatNumber, formatUSD } from "@/lib/format";
import { monthLabel } from "@/lib/trends";
import type { SpendBandMonth, SpendForecastMonth } from "@/lib/types";

// Colour by the band's upper end: 0 %, under 25, 25–50, 50–75, 75–90, 90–100, over 100 %.
const BAND_TOKENS = [
  "neutral-400",
  "accent-300",
  "accent-500",
  "accent-700",
  "warning-500",
  "warning-700",
  "danger-600",
];

export function SpendBandChart({ months }: { months: (SpendBandMonth & { projected?: boolean })[] }) {
  const labels = months.map((m) => monthLabel(m.month) + (m.projected ? " (prognose)" : ""));
  // One dataset per band as the API returns it, so a merged band has its own legend entry with its real range.
  const keys = [...new Map(months.flatMap((m) => m.bands ?? []).map((b) => [`${b.first}-${b.last}`, b])).values()].sort(
    (a, b) => a.first - b.first || a.last - b.last
  );
  const datasets = keys.map((k) => ({
    label: k.label,
    data: months.map((m) => m.bands?.find((b) => b.first === k.first && b.last === k.last)?.users ?? null),
    backgroundColor: () => axColor(BAND_TOKENS[k.last]),
  }));
  return (
    <div className="h-80">
      <Bar
        role="img"
        aria-label="Brukere per andel av forbruksgrensen ved månedsslutt, per måned"
        data={{ labels, datasets }}
        options={{
          ...commonLineOptions,
          scales: {
            x: { ...commonLineOptions.scales.x, stacked: true },
            y: { ...commonLineOptions.scales.y, stacked: true, title: { display: true, text: "Brukere" } },
          },
          plugins: {
            ...commonLineOptions.plugins,
            legend: bottomLegend,
            tooltip: {
              ...commonLineOptions.plugins.tooltip,
              filter: (item) => item.raw !== null,
            },
          },
        }}
      />
    </div>
  );
}

export function SpendForecastChart({
  totals,
  forecast,
}: {
  totals: SpendForecastMonth[];
  forecast: SpendForecastMonth[];
}) {
  const all = [...totals, ...forecast];
  const known = totals.length;
  // The forecast lines start at the last known (projected) month, so they join the actual line.
  const future = (pick: (f: SpendForecastMonth) => number) => all.map((f, i) => (i >= known - 1 ? pick(f) : null));
  const color = () => axColor("accent-600");
  return (
    <div className="h-80">
      <Line
        role="img"
        aria-label="Totalt forbruk per måned, fakturert og fremskrevet tre måneder, med lav og høy prognose"
        data={{
          labels: all.map((f) => monthLabel(f.month)),
          datasets: [
            {
              label: "Fakturert (denne måneden fremskrevet)",
              data: all.map((f, i) => (i < known ? f.mid_usd : null)),
              borderColor: color,
              backgroundColor: color,
            },
            {
              label: "Lav",
              data: future((f) => f.low_usd),
              borderColor: () => axColor("neutral-400"),
              backgroundColor: () => axColor("neutral-400"),
              borderWidth: 1,
              pointRadius: 0,
            },
            {
              label: "Høy",
              data: future((f) => f.high_usd),
              borderColor: () => axColor("neutral-400"),
              backgroundColor: () => axColor("accent-500", 0.15),
              borderWidth: 1,
              pointRadius: 0,
              fill: "-1",
            },
            {
              label: "Trend",
              data: future((f) => f.mid_usd),
              borderColor: color,
              backgroundColor: color,
              borderDash: [6, 4],
            },
          ],
        }}
        options={{
          ...commonLineOptions,
          scales: {
            ...commonLineOptions.scales,
            y: {
              ...commonLineOptions.scales.y,
              title: { display: true, text: "USD" },
              ticks: { ...commonLineOptions.scales.y.ticks, callback: (v) => formatNumber(Number(v)) },
            },
          },
          plugins: {
            ...commonLineOptions.plugins,
            legend: bottomLegend,
            tooltip: {
              ...commonLineOptions.plugins.tooltip,
              filter: (item) => item.raw !== null,
              callbacks: { label: (item) => `${item.dataset.label}: ${formatUSD(Number(item.raw))}` },
            },
          },
        }}
      />
    </div>
  );
}
