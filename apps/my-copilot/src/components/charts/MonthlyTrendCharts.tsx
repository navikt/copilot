"use client";

import { Bar, Line } from "react-chartjs-2";
import type { ChartType, Plugin } from "chart.js";
import { BodyShort } from "@navikt/ds-react";
import { axColor, bottomLegend, commonLineOptions, seriesColor } from "@/lib/chart-utils";
import { daysInCalendarMonth } from "@/lib/month-utils";
import { monthLabel, type FamilyShares, type MonthAnnotation, type ShareSeries } from "@/lib/trends";
import type { CopilotPRMonth, CreditsPerUserMonth } from "@/lib/types";

/**
 * Shares over time. `stacked` draws 100 % stacked bars (groups that add up to the whole, as the server
 * delivers them); otherwise plain lines, where a null is a gap. `shadeBefore` greys out the months before a data break the series cannot cross.
 */
export function ShareChart({
  months,
  series,
  label,
  stacked,
  annotations,
  shadeBefore,
}: {
  months: string[];
  series: ShareSeries[];
  label: string;
  stacked: boolean;
  annotations: MonthAnnotation[];
  shadeBefore?: string;
}) {
  const options = withEvents(baseOptions(stacked, "Andel (%)"), months, annotations);
  const plugins = [annotationPlugin(months, annotations, shadeBefore)];
  const datasets = series.map((s, i) => ({
    label: s.label,
    data: s.shares,
    borderColor: seriesColor(i),
    backgroundColor: seriesColor(i),
  }));
  if (!stacked) {
    return (
      <div className="h-72">
        <Line
          role="img"
          aria-label={label}
          data={{ labels: months.map(monthLabel), datasets }}
          options={options}
          plugins={plugins}
        />
      </div>
    );
  }
  return (
    <div className="h-72">
      <Bar
        role="img"
        aria-label={label}
        data={{ labels: months.map(monthLabel), datasets }}
        options={{ ...options, scales: { ...options.scales, y: { ...options.scales.y, max: 100 } } }}
        plugins={plugins}
      />
    </div>
  );
}

/** Tooltip footer naming the events of the month under the finger, numbered as in «Hendelser». */
function withEvents<T extends ReturnType<typeof baseOptions>>(
  options: T,
  months: string[],
  annotations: MonthAnnotation[]
) {
  return {
    ...options,
    layout: { padding: { top: 18 } },
    plugins: {
      ...options.plugins,
      // The legend goes below, so the marker row above the plot area stays free.
      legend: bottomLegend,
      tooltip: {
        ...options.plugins.tooltip,
        callbacks: {
          footer: (items: { dataIndex: number }[]) => {
            const a = annotations.find((x) => x.month === months[items[0]?.dataIndex]);
            return a ? a.labels.map((l, i) => `${a.numbers[i]}. ${l}`) : [];
          },
        },
      },
    },
  };
}

// Small numbered markers above the plot area, never over the data. A data break also gets a thin grey band
// on the month boundary, and `shadeBefore` greys the months before it.
function annotationPlugin<T extends ChartType>(
  months: string[],
  annotations: MonthAnnotation[],
  shadeBefore?: string
): Plugin<T> {
  return {
    id: "monthAnnotations",
    beforeDatasetsDraw(chart) {
      const { ctx, chartArea, scales } = chart;
      const step = months.length > 1 ? scales.x.getPixelForValue(1) - scales.x.getPixelForValue(0) : chartArea.width;
      const boundary = (month: string) => scales.x.getPixelForValue(months.indexOf(month)) - step / 2;
      ctx.save();
      for (const a of annotations) {
        if (!a.dataBreak || months.indexOf(a.month) <= 0) continue;
        ctx.fillStyle = axColor("neutral-500", 0.35);
        ctx.fillRect(boundary(a.month) - 2, chartArea.top, 4, chartArea.height);
      }
      ctx.restore();
    },
    afterDatasetsDraw(chart) {
      const { ctx, chartArea, scales } = chart;
      ctx.save();
      // Wash out the months before the break, so they read as a separate period, labelled in the marker row.
      if (shadeBefore && months.indexOf(shadeBefore) > 0) {
        const step = months.length > 1 ? scales.x.getPixelForValue(1) - scales.x.getPixelForValue(0) : 0;
        const x = scales.x.getPixelForValue(months.indexOf(shadeBefore)) - step / 2;
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
      for (const a of annotations) {
        const index = months.indexOf(a.month);
        if (index < 0) continue;
        const x = scales.x.getPixelForValue(index);
        const text = a.numbers.length > 2 ? `${a.numbers[0]}–${a.numbers[a.numbers.length - 1]}` : a.numbers.join(",");
        const w = Math.max(14, ctx.measureText(text).width + 8);
        ctx.fillStyle = axColor(a.dataBreak ? "neutral-700" : "neutral-500");
        ctx.beginPath();
        ctx.roundRect(x - w / 2, chartArea.top - 16, w, 14, 7);
        ctx.fill();
        ctx.fillStyle = axColor("text-neutral-contrast");
        ctx.fillText(text, x, chartArea.top - 9);
      }
      ctx.restore();
    },
  };
}

const baseOptions = (stacked: boolean, yTitle: string) => ({
  ...commonLineOptions,
  maintainAspectRatio: false,
  scales: {
    x: { ...commonLineOptions.scales.x, stacked },
    y: { ...commonLineOptions.scales.y, stacked, title: { display: true, text: yTitle } },
  },
});

export function FamilyShareChart({ data, annotations }: { data: FamilyShares; annotations: MonthAnnotation[] }) {
  const options = withEvents(baseOptions(true, "Andel av netto kostnad (%)"), data.months, annotations);
  return (
    <div className="h-80">
      <Bar
        role="img"
        aria-label="Andel av netto kostnad per modellfamilie per måned"
        data={{
          labels: data.months.map(monthLabel),
          datasets: data.series.map((s, i) => ({
            label: s.label,
            data: s.shares,
            backgroundColor: seriesColor(i),
          })),
        }}
        options={{ ...options, scales: { ...options.scales, y: { ...options.scales.y, max: 100 } } }}
        plugins={[annotationPlugin(data.months, annotations)]}
      />
    </div>
  );
}

export function CreditsPerUserChart({
  months,
  data,
  annotations,
}: {
  /** Every month on the axis; a month without a row is a gap. */
  months: string[];
  data: CreditsPerUserMonth[];
  annotations: MonthAnnotation[];
}) {
  const row = (m: string) => data.find((d) => d.month === m);
  return (
    <div className="h-80">
      <Line
        role="img"
        aria-label="AI Credits per aktiv bruker per måned, median og snitt"
        data={{
          labels: months.map(monthLabel),
          datasets: [
            {
              label: "Median",
              data: months.map((m) => (row(m) ? Math.round(row(m)!.median) : null)),
              borderColor: seriesColor(0),
              backgroundColor: seriesColor(0),
            },
            {
              label: "Snitt",
              data: months.map((m) => (row(m) ? Math.round(row(m)!.mean) : null)),
              borderColor: seriesColor(3),
              backgroundColor: seriesColor(3),
              borderDash: [6, 4],
            },
          ],
        }}
        options={withEvents(baseOptions(false, "AI Credits per bruker"), months, annotations)}
        plugins={[annotationPlugin(months, annotations)]}
      />
    </div>
  );
}

export function CopilotPRChart({
  months,
  data,
  annotations,
}: {
  /** Every month on the axis; a month without a row is a gap. */
  months: string[];
  data: CopilotPRMonth[];
  annotations: MonthAnnotation[];
}) {
  const row = (m: string) => data.find((d) => d.month === m);
  return (
    <>
      <div className="h-80">
        <Bar
          role="img"
          aria-label="Pull requests laget av Copilot og gjennomgått av Copilot per måned"
          data={{
            labels: months.map(monthLabel),
            datasets: [
              {
                label: "Laget av coding agent",
                data: months.map((m) => row(m)?.created_by_copilot ?? null),
                backgroundColor: seriesColor(2),
              },
              {
                label: "Gjennomgått av code review",
                data: months.map((m) => row(m)?.reviewed_by_copilot ?? null),
                backgroundColor: seriesColor(1),
              },
            ],
          }}
          options={withEvents(baseOptions(false, "Pull requests"), months, annotations)}
          plugins={[annotationPlugin(months, annotations)]}
        />
      </div>
      <BodyShort size="small" textColor="subtle">
        {data.some((d) => d.days < daysInCalendarMonth(d.month)) &&
          "Måneder uten data for alle dagene er ikke hele måneder."}
      </BodyShort>
    </>
  );
}
