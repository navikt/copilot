"use client";

import { Bar, Line } from "react-chartjs-2";
import { BodyShort } from "@navikt/ds-react";
import { commonLineOptions, seriesColor } from "@/lib/chart-utils";
import { eventMarkers, withEvents } from "@/components/charts/chart-events";
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
  const plugins = [eventMarkers(months, annotations, shadeBefore)];
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
        plugins={[eventMarkers(data.months, annotations)]}
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
        plugins={[eventMarkers(months, annotations)]}
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
          plugins={[eventMarkers(months, annotations)]}
        />
      </div>
      <BodyShort size="small" textColor="subtle">
        {data.some((d) => d.days < daysInCalendarMonth(d.month)) &&
          "Måneder uten data for alle dagene er ikke hele måneder."}
      </BodyShort>
    </>
  );
}
