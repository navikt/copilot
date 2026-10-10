"use client";

import { Bar, Line } from "react-chartjs-2";
import type { Plugin } from "chart.js";
import { BodyShort } from "@navikt/ds-react";
import { chartColors, commonLineOptions } from "@/lib/chart-utils";
import { monthLabel, type FamilyShares, type MonthAnnotation } from "@/lib/trends";
import type { CopilotPRMonth, CreditsPerUserMonth } from "@/lib/types";

// Vertical rules for dated events. A data break is drawn solid and darker; other events dashed.
function annotationPlugin(months: string[], annotations: MonthAnnotation[]): Plugin {
  return {
    id: "monthAnnotations",
    afterDatasetsDraw(chart) {
      const { ctx, chartArea, scales } = chart;
      ctx.save();
      for (const a of annotations) {
        const index = months.indexOf(a.month);
        if (index < 0) continue;
        const x = scales.x.getPixelForValue(index);
        ctx.strokeStyle = a.dataBreak ? "rgba(17, 24, 39, 0.9)" : "rgba(107, 114, 128, 0.8)";
        ctx.lineWidth = a.dataBreak ? 2 : 1;
        ctx.setLineDash(a.dataBreak ? [] : [4, 4]);
        ctx.beginPath();
        ctx.moveTo(x, chartArea.top);
        ctx.lineTo(x, chartArea.bottom);
        ctx.stroke();
        ctx.fillStyle = ctx.strokeStyle;
        // Room for a label only on a wide chart; with many months only the data breaks get one.
        if (chartArea.width < 480 || (months.length > 12 && !a.dataBreak)) continue;
        ctx.font = "11px sans-serif";
        const text = a.labels.length > 1 ? `${a.labels.length} hendelser` : a.labels[0];
        ctx.fillText(text, Math.min(x + 4, chartArea.right - ctx.measureText(text).width), chartArea.top + 12);
      }
      ctx.restore();
    },
  };
}

function AnnotationList({ annotations }: { annotations: MonthAnnotation[] }) {
  if (!annotations.length) return null;
  return (
    <ul className="text-sm list-disc pl-5" aria-label="Hendelser markert i grafen">
      {annotations.map((a) => (
        <li key={a.month}>
          {monthLabel(a.month)}: {a.labels.join(", ")}
          {a.dataBreak && " (brudd i dataene)"}
        </li>
      ))}
    </ul>
  );
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
  const options = baseOptions(true, "Andel av netto kostnad (%)");
  return (
    <>
      <div className="h-80">
        <Bar
          role="img"
          aria-label="Andel av netto kostnad per modellfamilie per måned"
          data={{
            labels: data.months.map(monthLabel),
            datasets: data.series.map((s, i) => ({
              label: s.label,
              data: s.shares,
              backgroundColor: chartColors[i % chartColors.length],
            })),
          }}
          options={{ ...options, scales: { ...options.scales, y: { ...options.scales.y, max: 100 } } }}
          plugins={[annotationPlugin(data.months, annotations)]}
        />
      </div>
      <AnnotationList annotations={annotations} />
    </>
  );
}

export function CreditsPerUserChart({
  data,
  annotations,
}: {
  data: CreditsPerUserMonth[];
  annotations: MonthAnnotation[];
}) {
  const months = data.map((d) => d.month);
  return (
    <>
      <div className="h-80">
        <Line
          role="img"
          aria-label="AI Credits per aktiv bruker per måned, median og snitt"
          data={{
            labels: months.map(monthLabel),
            datasets: [
              {
                label: "Median",
                data: data.map((d) => Math.round(d.median)),
                borderColor: chartColors[0],
                backgroundColor: chartColors[0],
              },
              {
                label: "Snitt",
                data: data.map((d) => Math.round(d.mean)),
                borderColor: chartColors[3],
                backgroundColor: chartColors[3],
                borderDash: [6, 4],
              },
            ],
          }}
          options={baseOptions(false, "AI Credits per bruker")}
          plugins={[annotationPlugin(months, annotations)]}
        />
      </div>
      <AnnotationList annotations={annotations} />
    </>
  );
}

export function CopilotPRChart({ data, annotations }: { data: CopilotPRMonth[]; annotations: MonthAnnotation[] }) {
  const months = data.map((d) => d.month);
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
                data: data.map((d) => d.created_by_copilot),
                backgroundColor: chartColors[2],
              },
              {
                label: "Gjennomgått av code review",
                data: data.map((d) => d.reviewed_by_copilot),
                backgroundColor: chartColors[1],
              },
            ],
          }}
          options={baseOptions(false, "Pull requests")}
          plugins={[annotationPlugin(months, annotations)]}
        />
      </div>
      <AnnotationList annotations={annotations} />
      <BodyShort size="small" textColor="subtle">
        {data.some((d) => d.days < 28) && "Måneder med færre enn 28 dager med data er ikke hele måneder."}
      </BodyShort>
    </>
  );
}
