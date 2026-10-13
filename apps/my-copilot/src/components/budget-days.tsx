"use client";

import { useState } from "react";
import { BodyShort, HStack, Select, VStack } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import { ChartFigure, NumbersFold } from "@/components/charts/chart-figure";
import { EventChart } from "@/components/charts/EventChart";
import { budgetComparison } from "@/lib/budget-days";
import { type CostEvent, eventAnnotations, eventSummary } from "@/lib/cost-events";
import { formatDate, formatNumber, formatPercent } from "@/lib/format";
import { monthLabel } from "@/lib/trends";
import type { BudgetDay, SpendBand } from "@/lib/types";

import { BUDGET_BANDS as BANDS } from "@/components/high-consumers";

const source = (d: BudgetDay) => (d.snapshots ? `GitHub (${formatNumber(d.snapshots)})` : "Anslag");

/**
 * F7: users per share of their personal limit used, day by day through the month. Never individuals.
 * No month-end projection yet: add it from the per-person forecast once shownModels() lets per_user through.
 */
export function BudgetDaysView({ days, events = [] }: { days: BudgetDay[]; events?: CostEvent[] }) {
  const months = [...new Set(days.filter((d) => d.bands).map((d) => d.day.slice(0, 7)))];
  const [month, setMonth] = useState(months.at(-1) ?? "");
  if (!months.length) return <BodyShort>Ingen dager med nok brukere ennå.</BodyShort>;
  const shown = days.filter((d) => d.day.startsWith(month) && d.bands);
  const keys = shown.map((d) => d.day);
  const markers = eventAnnotations(events, keys);
  const share = (d: BudgetDay, b: SpendBand) => (b.users / d.users) * 100;
  const series = BANDS.map((label, i) => ({
    label,
    color: i,
    data: shown.map((d) => {
      const b = d.bands!.find((x) => x.first === i);
      return b ? share(d, b) : null;
    }),
    pointLabels: shown.map((d) => d.bands!.find((x) => x.first === i && x.last !== i)?.label ?? null),
  }));
  const line = budgetComparison(days, month);
  return (
    <VStack gap="space-16">
      <HStack gap="space-16" align="center">
        <Select
          label="Måned"
          hideLabel
          size="small"
          value={month}
          onChange={(e) => setMonth(e.target.value)}
          className="w-fit"
        >
          {months.map((m) => (
            <option key={m} value={m}>
              {monthLabel(m)}
            </option>
          ))}
        </Select>
        {line && <BodyShort>{line}.</BodyShort>}
      </HStack>
      <ChartFigure
        title="Hvor langt har brukerne kommet i budsjettet sitt?"
        summary={
          `Andel brukere per dag etter hvor mye av sin personlige grense de har brukt fra den 1. i ${monthLabel(month)}.` +
          (line ? ` ${line}.` : "") +
          eventSummary(markers)
        }
        chart={
          <EventChart
            kind="bar"
            stacked
            max={100}
            unit="pct"
            keys={keys}
            labels={keys.map((k) => String(Number(k.slice(8))))}
            annotations={markers}
            series={series}
          />
        }
        table={
          <Table size="small">
            <TableHeader>
              <TableRow>
                <TableHeaderCell scope="col">Dag</TableHeaderCell>
                <TableHeaderCell scope="col" align="right">
                  Brukere
                </TableHeaderCell>
                {BANDS.map((b) => (
                  <TableHeaderCell key={b} scope="col" align="right">
                    {b}
                  </TableHeaderCell>
                ))}
                <TableHeaderCell scope="col">Kilde</TableHeaderCell>
              </TableRow>
            </TableHeader>
            <TableBody>
              {shown.map((d) => (
                <TableRow key={d.day}>
                  <TableHeaderCell scope="row" className="whitespace-nowrap">
                    {formatDate(d.day)}
                  </TableHeaderCell>
                  <TableDataCell align="right">{formatNumber(d.users)}</TableDataCell>
                  {d.bands!.map((b) => (
                    <TableDataCell key={b.first} align="right" colSpan={b.last - b.first + 1}>
                      {b.first !== b.last && <span className="text-text-subtle">{b.label}: </span>}
                      {formatPercent(share(d, b))}
                    </TableDataCell>
                  ))}
                  <TableDataCell>{source(d)}</TableDataCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        }
      />
      <NumbersFold summary="Slik leser du tallene">
        <BodyShort size="small" textColor="subtle">
          Hver søyle er én dag. Den viser hvor stor del av brukerne som har brukt 0–10 %, 10–25 % og så videre av sin
          egen grense, regnet fra forbruk til listepris fra den 1. i måneden. Kredittene i lisensene er ikke trukket
          fra, fordi brukerne ikke ser dem. Bare brukere med Copilot-aktivitet i måneden er med, og brukere med grense 0
          er ikke med. Nedre grense hører til gruppen: nøyaktig 10 % telles i 10–25 %. Grupper med færre enn fem brukere
          er slått sammen med nabogruppen. Kolonnen «Kilde» viser om dagen bygger på GitHubs eget forbrukstall per
          person (antall brukere i parentes) eller på et anslag fra bruksdata. GitHubs tall finnes fra oktober 2026; før
          det er alle dager anslag. Tallene for juni 2026 starter 19. juni. Det er ingen prognose for månedsslutt ennå.
        </BodyShort>
      </NumbersFold>
    </VStack>
  );
}
