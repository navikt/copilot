"use client";

import { Line } from "react-chartjs-2";
import { BodyShort, HGrid, Heading, VStack } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@navikt/ds-react/Table";
import { bottomLegend, commonLineOptions, seriesColor } from "@/lib/chart-utils";
import { costEvents, eventAnnotations, eventSummary } from "@/lib/cost-events";
import { ChartFigure, NumbersFold, PlainTable } from "./chart-figure";
import { eventMarkers, withEvents } from "./chart-events";
import { formatUSD } from "@/lib/format";
import { monthName } from "@/lib/team-year";
import type { AdminMonth, CostForecast } from "@/lib/types";

const dayLong = (day: string) =>
  new Date(`${day}T00:00:00Z`).toLocaleDateString("nb-NO", { day: "numeric", month: "long", timeZone: "UTC" });
const dayLabel = (day: string) =>
  new Date(`${day}T00:00:00Z`).toLocaleDateString("nb-NO", { day: "numeric", month: "short", timeZone: "UTC" });

/** Cumulative net this month: actual, then the trend midpoint with a range that widens to ±range_pct at month end. */
export function monthSeries(data: CostForecast) {
  const trend = data.models.find((m) => m.model === "trend");
  const days = data.days ?? [];
  const lastActual = days.findLastIndex((d) => !d.forecast);
  const pct = (trend?.range_pct ?? 0) / 100;
  const end = days.at(-1)?.net_usd ?? 0;
  const span = days.length - 1 - lastActual;
  const runOut = trend?.allowance_runs_out;
  const series = monthSeriesBase(days, lastActual, trend, pct, end, span, runOut, data);
  // A projected daily series from the API, when present, replaces the midpoint and the widening range.
  const byDate = new Map((data.daily_forecast ?? []).map((p) => [p.date, p]));
  if (trend && byDate.size) {
    const pick = (k: "cumulative_net_usd" | "low" | "high", fallback: (number | null)[]) =>
      days.map((d, i) => (i > lastActual && byDate.has(d.day) ? byDate.get(d.day)![k] : fallback[i]));
    return {
      ...series,
      forecast: pick("cumulative_net_usd", series.forecast),
      low: pick("low", series.low),
      high: pick("high", series.high),
    };
  }
  return series;
}

function monthSeriesBase(
  days: NonNullable<CostForecast["days"]>,
  lastActual: number,
  trend: CostForecast["models"][number] | undefined,
  pct: number,
  end: number,
  span: number,
  runOut: string | undefined,
  data: CostForecast
) {
  return {
    labels: days.map((d) => dayLabel(d.day)),
    actual: days.map((d, i) => (i <= lastActual ? d.net_usd : null)),
    // Starts at the last actual day so the two lines join.
    forecast: days.map((d, i) => (trend && i >= lastActual && lastActual >= 0 ? d.net_usd : null)),
    low: days.map((d, i) =>
      trend && i >= lastActual ? Math.max(0, d.net_usd - (end * pct * (i - lastActual)) / (span || 1)) : null
    ),
    high: days.map((d, i) =>
      trend && i >= lastActual ? d.net_usd + (end * pct * (i - lastActual)) / (span || 1) : null
    ),
    runOut: days.map((d) => (d.day === runOut ? d.net_usd : null)),
    runOutDay: days.some((d) => d.day === runOut) ? runOut : undefined,
    budget: data.budget_status === "set" && data.budget_usd ? data.budget_usd : undefined,
    toDate: lastActual >= 0 ? days[lastActual] : undefined,
    trend,
  };
}

/** Cumulative fakturert from 1 January: actual months, then the year forecast to 31 December with its range. */
export function yearSeries(months: AdminMonth[], data: CostForecast | null) {
  const points = data?.year_forecast ?? [];
  const lastActual = points.findLastIndex((p) => !p.forecast);
  const basis = new Map(months.map((m) => [m.month, m.basis]));
  const premium = (i: number) => basis.get(points[i]?.month) === "premium";
  return {
    points,
    labels: points.map((p) => monthName(p.month)),
    // The premium line runs one month into net so the two join.
    premium: points.map((p, i) =>
      i <= lastActual && (premium(i) || (i > 0 && premium(i - 1))) ? p.cumulative_net_usd : null
    ),
    actual: points.map((p, i) => (i <= lastActual && !premium(i) ? p.cumulative_net_usd : null)),
    forecast: points.map((p, i) => (i >= lastActual && lastActual >= 0 ? p.cumulative_net_usd : null)),
    low: points.map((p, i) => (i >= lastActual && lastActual >= 0 ? p.low : null)),
    high: points.map((p, i) => (i >= lastActual && lastActual >= 0 ? p.high : null)),
    end: data?.year_end,
    toDate: lastActual >= 0 ? points[lastActual] : undefined,
  };
}

/** The one line under the year chart. */
export const yearEndLine = (e: NonNullable<CostForecast["year_end"]>) =>
  `Forventet ved årets slutt: ${formatUSD(e.mid)} (${formatUSD(e.low)}–${formatUSD(e.high)})` +
  (e.label ? `, ${e.label}.` : ".");

/** The legend label for the range, in whole percent. */
export const rangeLabel = (pct: number) => `Spenn (±${Math.round(pct)} %)`;

const usdTicks = { ...commonLineOptions.scales.y.ticks, callback: (v: string | number) => formatUSD(Number(v)) };

function MonthChart({ data }: { data: CostForecast }) {
  const s = monthSeries(data);
  if (!s.toDate) return <BodyShort>Ingen dager med data denne måneden ennå.</BodyShort>;
  const summary =
    `Fakturert ${formatUSD(s.toDate.net_usd)} per ${dayLong(s.toDate.day)}.` +
    (s.trend
      ? ` Forventet faktura ved månedsslutt er ${formatUSD(s.trend.net_mid_usd)}, mellom ${formatUSD(s.trend.net_low_usd)} og ${formatUSD(s.trend.net_high_usd)}.`
      : "") +
    (s.budget ? ` Budsjettet i GitHub er ${formatUSD(s.budget)}.` : "") +
    (s.runOutDay
      ? data.allowance_used_up
        ? ` Kredittene i lisensene ble brukt opp ${dayLong(s.runOutDay)}.`
        : ` Kredittene i lisensene brukes etter prognosen opp ${dayLong(s.runOutDay)}.`
      : "");
  const line = (i: number, extra: object) => ({
    borderColor: seriesColor(i),
    backgroundColor: seriesColor(i),
    pointRadius: 0,
    borderWidth: 2,
    ...extra,
  });
  const datasets = [
    { label: "Fakturert", data: s.actual, ...line(0, {}) },
    ...(s.trend
      ? [
          { label: "Forventet faktura", data: s.forecast, ...line(0, { borderDash: [6, 4] }) },
          {
            label: rangeLabel(s.trend.range_pct),
            data: s.low,
            ...line(0, { borderWidth: 0, backgroundColor: seriesColor(0, 0.15) }),
          },
          // Fills down to the low edge; hidden from the legend so the range shows once.
          {
            label: "",
            data: s.high,
            ...line(0, { borderWidth: 0, backgroundColor: seriesColor(0, 0.15), fill: "-1" }),
          },
        ]
      : []),
    ...(s.budget
      ? [{ label: "Budsjett i GitHub", data: s.labels.map(() => s.budget), ...line(4, { borderDash: [2, 2] }) }]
      : []),
  ];
  const keys = (data.days ?? []).map((d) => d.day);
  const markers = eventAnnotations(costEvents(data), keys);
  const events = withEvents({ ...commonLineOptions, plugins: { ...commonLineOptions.plugins } }, keys, markers);
  const options = {
    ...events,
    spanGaps: false,
    plugins: {
      ...events.plugins,
      legend: {
        ...bottomLegend,
        labels: { ...bottomLegend.labels, filter: (item: { text: string }) => item.text !== "" },
      },
      tooltip: {
        ...events.plugins.tooltip,
        filter: (item: { dataset: { label?: string }; raw: unknown }) => !!item.dataset.label && item.raw != null,
        callbacks: {
          ...events.plugins.tooltip.callbacks,
          label: (ctx: { dataset: { label?: string }; parsed: { y: number }; dataIndex: number }) =>
            ctx.dataset.label?.startsWith("Spenn")
              ? ` ${ctx.dataset.label}: ${formatUSD(ctx.parsed.y)}–${formatUSD(s.high[ctx.dataIndex] ?? 0)}`
              : ` ${ctx.dataset.label}: ${formatUSD(ctx.parsed.y)}`,
        },
      },
    },
    scales: {
      ...commonLineOptions.scales,
      // Headroom keeps the budget line clear of the event markers above the plot.
      y: {
        ...commonLineOptions.scales.y,
        ticks: usdTicks,
        grace: "12%",
        suggestedMax: s.budget ? s.budget * 1.15 : undefined,
      },
      x: { ...commonLineOptions.scales.x, ticks: { ...commonLineOptions.scales.x.ticks, maxTicksLimit: 8 } },
    },
  };
  return (
    <ChartFigure
      title="Er vi på vei over budsjettet?"
      summary={
        summary +
        eventSummary(
          eventAnnotations(
            costEvents(data).filter((e) => !e.runOut),
            keys
          )
        )
      }
      chart={
        <Line
          data={{ labels: s.labels, datasets } as Parameters<typeof Line>[0]["data"]}
          options={options as Parameters<typeof Line>[0]["options"]}
          plugins={[eventMarkers<"line">(keys, markers)]}
        />
      }
    />
  );
}

function YearChart({ months, data }: { months: AdminMonth[]; data: CostForecast }) {
  const s = yearSeries(months, data);
  if (!s.toDate || !s.end) return <BodyShort>Vi har for lite data til en prognose for året ennå.</BodyShort>;
  const summary =
    `Fakturert ${formatUSD(s.toDate.cumulative_net_usd)} fra januar til og med ${monthName(s.toDate.month)}.` +
    (s.premium.some((v) => v != null) ? " Fram til og med mai var grunnlaget premiumforespørsler." : "") +
    (s.end
      ? ` Forventet ved årets slutt er ${formatUSD(s.end.mid)}, mellom ${formatUSD(s.end.low)} og ${formatUSD(s.end.high)}.`
      : "");
  const line = (i: number, extra: object) => ({
    borderColor: seriesColor(i),
    backgroundColor: seriesColor(i),
    pointRadius: 0,
    borderWidth: 2,
    ...extra,
  });
  const datasets = [
    { label: "Fakturert (premiumforespørsler)", data: s.premium, ...line(5, {}) },
    { label: "Fakturert", data: s.actual, ...line(0, {}) },
    ...(s.end
      ? [
          { label: "Forventet faktura", data: s.forecast, ...line(0, { borderDash: [6, 4] }) },
          { label: "Spenn", data: s.low, ...line(0, { borderWidth: 0, backgroundColor: seriesColor(0, 0.15) }) },
          {
            label: "",
            data: s.high,
            ...line(0, { borderWidth: 0, backgroundColor: seriesColor(0, 0.15), fill: "-1" }),
          },
        ]
      : []),
  ];
  const keys = s.points.map((p) => p.month);
  const markers = eventAnnotations(costEvents(data), keys);
  const events = withEvents({ ...commonLineOptions, plugins: { ...commonLineOptions.plugins } }, keys, markers);
  const options = {
    ...events,
    spanGaps: false,
    plugins: {
      ...events.plugins,
      legend: {
        ...bottomLegend,
        labels: { ...bottomLegend.labels, filter: (item: { text: string }) => item.text !== "" },
      },
      tooltip: {
        ...events.plugins.tooltip,
        // The last actual month starts the forecast lines; show it once, as Fakturert.
        filter: (item: { dataset: { label?: string }; raw: unknown; dataIndex: number }) =>
          !!item.dataset.label &&
          item.raw != null &&
          (s.points[item.dataIndex]?.forecast || !["Forventet faktura", "Spenn"].includes(item.dataset.label)),
        callbacks: {
          ...events.plugins.tooltip.callbacks,
          label: (ctx: { dataset: { label?: string }; parsed: { y: number }; dataIndex: number }) =>
            ctx.dataset.label === "Spenn"
              ? ` Spenn: ${formatUSD(ctx.parsed.y)}–${formatUSD(s.high[ctx.dataIndex] ?? 0)}`
              : ` ${ctx.dataset.label}: ${formatUSD(ctx.parsed.y)}`,
        },
      },
    },
    scales: {
      ...commonLineOptions.scales,
      y: { ...commonLineOptions.scales.y, ticks: usdTicks, grace: "12%" },
    },
  };
  return (
    <VStack gap="space-8">
      <ChartFigure
        title="Hvor havner vi ved årets slutt?"
        summary={summary + eventSummary(markers)}
        chart={
          <Line
            data={{ labels: s.labels, datasets } as Parameters<typeof Line>[0]["data"]}
            options={options as Parameters<typeof Line>[0]["options"]}
            plugins={[eventMarkers<"line">(keys, markers)]}
          />
        }
      />
      <BodyShort size="small" textColor="subtle">
        {yearEndLine(s.end)}
      </BodyShort>
    </VStack>
  );
}

/** Per day this month, latest first: forecast rows are subtle and carry the range. */
function DayTable({ data }: { data: CostForecast }) {
  const s = monthSeries(data);
  const days = (data.days ?? []).map((d, i) => ({ ...d, low: s.low[i], high: s.high[i] })).reverse();
  return (
    <VStack gap="space-4">
      <Heading level="4" size="xsmall">
        Denne måneden, siste dag først
      </Heading>
      <BodyShort size="small" textColor="subtle">
        Grå rader er prognose.
      </BodyShort>
      <div className="max-h-96 overflow-y-auto">
        <Table size="small">
          <TableHeader className="sticky top-0 bg-(--ax-bg-default)">
            <TableRow>
              <TableHeaderCell scope="col">Dag</TableHeaderCell>
              <TableHeaderCell scope="col" align="right">
                Fakturert
              </TableHeaderCell>
              <TableHeaderCell scope="col" align="right">
                Lav
              </TableHeaderCell>
              <TableHeaderCell scope="col" align="right">
                Høy
              </TableHeaderCell>
            </TableRow>
          </TableHeader>
          <TableBody>
            {days.map((d) => (
              <TableRow key={d.day} className={d.forecast ? "text-text-subtle italic" : undefined}>
                <TableHeaderCell scope="row" className="whitespace-nowrap font-normal">
                  {dayLabel(d.day)}
                </TableHeaderCell>
                <TableDataCell align="right">{formatUSD(d.net_usd)}</TableDataCell>
                <TableDataCell align="right">{d.forecast && d.low != null ? formatUSD(d.low) : ""}</TableDataCell>
                <TableDataCell align="right">{d.forecast && d.high != null ? formatUSD(d.high) : ""}</TableDataCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </VStack>
  );
}

/** «Denne måneden» and «Hittil i år», side by side on wide screens, with one full-width «Vis tallene» under both. */
export function CostForecastCharts({ data, months }: { data: CostForecast; months: AdminMonth[] }) {
  const s = yearSeries(months, data);
  const premium = new Set(months.filter((m) => m.basis === "premium").map((m) => m.month));
  return (
    <VStack gap="space-8">
      <HGrid columns={{ xs: 1, md: 2 }} gap="space-24" className="print:grid-cols-2">
        <MonthChart data={data} />
        <YearChart months={months} data={data} />
      </HGrid>
      <NumbersFold>
        <HGrid columns={{ xs: 1, md: 2 }} gap="space-24">
          <DayTable data={data} />
          <VStack gap="space-4">
            <Heading level="4" size="xsmall">
              Hele året
            </Heading>
            <PlainTable
              rows={[
                ["Måned", "Fakturert hittil i år"],
                ...s.points.map((p) =>
                  p.forecast
                    ? [
                        `${monthName(p.month)} (prognose)`,
                        `${formatUSD(p.cumulative_net_usd)} (${formatUSD(p.low)}–${formatUSD(p.high)})`,
                      ]
                    : [
                        monthName(p.month) + (premium.has(p.month) ? " (premiumforespørsler)" : ""),
                        formatUSD(p.cumulative_net_usd),
                      ]
                ),
              ]}
            />
          </VStack>
        </HGrid>
      </NumbersFold>
    </VStack>
  );
}
