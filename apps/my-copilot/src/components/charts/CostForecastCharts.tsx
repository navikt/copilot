"use client";

import { Bar, Line } from "react-chartjs-2";
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

/** Net per month this year; the forecast month gets the trend range instead. */
export function yearSeries(months: AdminMonth[], data: CostForecast | null) {
  const trend = data?.models.find((m) => m.model === "trend");
  const shown = months.filter((m) => m.month <= (data?.month ?? m.month));
  const isForecast = (m: AdminMonth) => !!trend && m.month === data?.month;
  const net = (m: AdminMonth, basis: "premium" | "net") =>
    !isForecast(m) && m.basis === basis ? m.enterprise_net_usd : null;
  return {
    months: shown,
    labels: shown.map((m) => monthName(m.month)),
    premium: shown.map((m) => net(m, "premium")),
    net: shown.map((m) => net(m, "net")),
    range: shown.map((m) => (isForecast(m) ? ([trend!.net_low_usd, trend!.net_high_usd] as [number, number]) : null)),
    mid: shown.map((m) => (isForecast(m) ? trend!.net_mid_usd : null)),
    trend,
    forecastMonth: trend ? data?.month : undefined,
  };
}

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
            label: `Spenn (±${s.trend.range_pct.toLocaleString("nb-NO")} %)`,
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

function YearChart({ months, data }: { months: AdminMonth[]; data: CostForecast | null }) {
  const s = yearSeries(months, data);
  if (!s.months.length) return <BodyShort>Ingen måneder med faktura ennå i år.</BodyShort>;
  const billed = s.months.filter((m, i) => s.premium[i] != null || s.net[i] != null);
  const summary =
    `Fakturert per måned. ${billed.map((m) => `${monthName(m.month)} ${formatUSD(m.enterprise_net_usd ?? 0)}`).join(", ")}.` +
    (s.premium.some((v) => v != null) ? " Fram til og med mai var grunnlaget premiumforespørsler." : "") +
    (s.trend && s.forecastMonth
      ? ` Forventet faktura for ${monthName(s.forecastMonth)} er ${formatUSD(s.trend.net_mid_usd)}, mellom ${formatUSD(s.trend.net_low_usd)} og ${formatUSD(s.trend.net_high_usd)}.`
      : "");
  const bar = (i: number, alpha: number, extra: object = {}) => ({
    backgroundColor: seriesColor(i, alpha),
    borderColor: seriesColor(i),
    borderWidth: 1,
    borderRadius: 2,
    ...extra,
  });
  const datasets = [
    { type: "bar", label: "Fakturert (premiumforespørsler)", data: s.premium, ...bar(5, 0.6) },
    { type: "bar", label: "Fakturert", data: s.net, ...bar(0, 0.8) },
    ...(s.trend
      ? [
          {
            type: "bar",
            label: "Forventet faktura (foreløpig, spenn)",
            data: s.range,
            ...bar(0, 0.2, { borderDash: [4, 3], borderSkipped: false }),
          },
          {
            type: "line",
            label: "Forventet faktura (midtpunkt)",
            data: s.mid,
            showLine: false,
            pointStyle: "line",
            pointRadius: 14,
            borderWidth: 2,
            borderColor: seriesColor(0),
            backgroundColor: seriesColor(0),
          },
        ]
      : []),
  ];
  const keys = s.months.map((m) => m.month);
  const markers = eventAnnotations(costEvents(data), keys);
  const events = withEvents({ ...commonLineOptions, plugins: { ...commonLineOptions.plugins } }, keys, markers);
  const options = {
    ...events,
    // Bars from different datasets share the slot of their month: only one is set per month.
    datasets: { bar: { skipNull: true, grouped: true } },
    plugins: {
      ...events.plugins,
      tooltip: {
        ...events.plugins.tooltip,
        filter: (item: { raw: unknown }) => item.raw != null,
        callbacks: {
          ...events.plugins.tooltip.callbacks,
          label: (ctx: { dataset: { label?: string }; raw: unknown; parsed: { y: number } }) =>
            Array.isArray(ctx.raw)
              ? ` ${ctx.dataset.label}: ${formatUSD(ctx.raw[0])}–${formatUSD(ctx.raw[1])}`
              : ` ${ctx.dataset.label}: ${formatUSD(ctx.parsed.y)}`,
        },
      },
    },
    scales: { ...commonLineOptions.scales, y: { ...commonLineOptions.scales.y, ticks: usdTicks } },
  };
  return (
    <ChartFigure
      title="Hvor mye har vi betalt i år?"
      summary={summary + eventSummary(markers)}
      chart={
        <Bar
          data={{ labels: s.labels, datasets } as Parameters<typeof Bar>[0]["data"]}
          options={options as Parameters<typeof Bar>[0]["options"]}
          plugins={[eventMarkers<"bar">(keys, markers)]}
        />
      }
    />
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
              Hittil i år
            </Heading>
            <PlainTable
              rows={[
                ["Måned", "Fakturert"],
                ...s.months.map((m, i) =>
                  s.range[i]
                    ? [`${monthName(m.month)} (foreløpig)`, `${formatUSD(s.range[i]![0])}–${formatUSD(s.range[i]![1])}`]
                    : [
                        monthName(m.month) + (m.basis === "premium" ? " (premiumforespørsler)" : ""),
                        m.enterprise_net_usd != null ? formatUSD(m.enterprise_net_usd) : "Ikke fakturert ennå",
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
