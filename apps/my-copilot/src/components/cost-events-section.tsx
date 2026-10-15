import { BodyShort, Detail, VStack } from "@navikt/ds-react";
import { List, ListItem } from "@navikt/ds-react/List";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import { NumbersFold } from "@/components/charts/chart-figure";
import { InsightSection } from "@/components/insight-page";
import { getAdminActiveUsers, getAdminBudgetDays, getBillingModelDaily, getCostForecast } from "@/lib/cached-bigquery";
import {
  type CostEvent,
  costEvents,
  dayMonth,
  type EventEffect,
  eventEffects,
  eventMonths,
  type RunOutStatus,
} from "@/lib/cost-events";
import { formatPercent, formatUSD } from "@/lib/format";
import { monthName } from "@/lib/team-year";

const pct = (v: number) => formatPercent(v, v > 0 && Math.abs(v) < 10 ? 1 : 0);
const signedPct = (v: number) => `${v >= 0 ? "+" : "−"}${formatPercent(Math.abs(v))}`;
const signedPp = (v: number) =>
  `${v > 0 ? "+" : v < 0 ? "−" : ""}${Math.abs(v).toLocaleString("nb-NO", { maximumFractionDigits: 1 })} pp`;

function formatRunOutCell(month: string, date: string | null, status?: RunOutStatus): string {
  if (date) return dayMonth(date);
  if (status === "incomplete") return `Foreløpig ikke brukt opp (${monthName(month)})`;
  if (status === "unavailable") return "–";
  return `Ikke brukt opp (${monthName(month)})`;
}

export function CostEventsView({ effects }: { effects: EventEffect[] }) {
  if (effects.length === 0) {
    return <BodyShort>Ingen hendelser med nok data før og etter ennå.</BodyShort>;
  }
  return (
    <VStack gap="space-16">
      {effects.map((e) => (
        <VStack key={e.event.date + e.event.text} gap="space-8">
          <BodyShort>
            <strong>{e.event.label}.</strong> {e.summary}
          </BodyShort>
          <Table size="small">
            <TableHeader>
              <TableRow>
                <TableHeaderCell scope="col">Mål (14 dager, ukedagsjustert)</TableHeaderCell>
                <TableHeaderCell scope="col" align="right">
                  Før
                </TableHeaderCell>
                <TableHeaderCell scope="col" align="right">
                  Etter
                </TableHeaderCell>
                <TableHeaderCell scope="col" align="right">
                  Endring
                </TableHeaderCell>
              </TableRow>
            </TableHeader>
            <TableBody>
              {e.costPerUser && (
                <TableRow>
                  <TableHeaderCell scope="row">
                    {e.costPerUser.provisional
                      ? "Kostnad per person (foreløpig, listepris)"
                      : "Kostnad per fakturert person (listepris)"}
                  </TableHeaderCell>
                  <TableDataCell align="right">{formatUSD(e.costPerUser.beforeUsd)}</TableDataCell>
                  <TableDataCell align="right">{formatUSD(e.costPerUser.afterUsd)}</TableDataCell>
                  <TableDataCell align="right">{signedPct(e.costPerUser.changePct)}</TableDataCell>
                </TableRow>
              )}
              {e.atLimit90 && (
                <TableRow>
                  <TableHeaderCell scope="row">
                    Andel på 90 % eller mer av grensen{e.atLimit90.minimum ? " (minimum)" : ""}
                  </TableHeaderCell>
                  <TableDataCell align="right">
                    {pct(e.atLimit90.beforePct)} ({dayMonth(e.atLimit90.beforeDay)})
                  </TableDataCell>
                  <TableDataCell align="right">
                    {pct(e.atLimit90.afterPct)} ({dayMonth(e.atLimit90.afterDay)})
                  </TableDataCell>
                  <TableDataCell align="right">{signedPp(e.atLimit90.afterPct - e.atLimit90.beforePct)}</TableDataCell>
                </TableRow>
              )}
              {e.runOut && (
                <TableRow>
                  <TableHeaderCell scope="row">Dato kredittene i lisensene går tom</TableHeaderCell>
                  <TableDataCell align="right">
                    {formatRunOutCell(e.runOut.beforeMonth, e.runOut.beforeDate, e.runOut.beforeStatus)}
                  </TableDataCell>
                  <TableDataCell align="right">
                    {formatRunOutCell(e.runOut.afterMonth, e.runOut.afterDate, e.runOut.afterStatus)}
                  </TableDataCell>
                  <TableDataCell align="right">
                    {e.runOut.beforeDate && e.runOut.afterDate
                      ? (() => {
                          const diff = Number(e.runOut.afterDate.slice(8)) - Number(e.runOut.beforeDate.slice(8));
                          return diff === 0
                            ? "Samme dag"
                            : `${Math.abs(diff)} ${Math.abs(diff) === 1 ? "dag" : "dager"} ${diff < 0 ? "tidligere" : "senere"}`;
                        })()
                      : "–"}
                  </TableDataCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
          {e.modelMix.length > 0 && (
            <NumbersFold summary="Modellmiks før og etter">
              <List size="small">
                {e.modelMix.slice(0, 5).map((m) => (
                  <ListItem key={m.family}>
                    {m.label}: {pct(m.beforePct)} → {pct(m.afterPct)} ({signedPp(m.diffPp)})
                  </ListItem>
                ))}
              </List>
            </NumbersFold>
          )}
          {e.atLimit90?.minimum && (
            <Detail textColor="subtle">
              Minimum: grupper med færre enn fem personer er slått sammen med nabogruppen.
            </Detail>
          )}
        </VStack>
      ))}
    </VStack>
  );
}

export async function CostEventsSection({ token, events }: { token: string; events?: CostEvent[] }) {
  const forecast = await getCostForecast(token).catch(() => null);
  const allEvents = events ?? costEvents(forecast);
  const currentMonth = new Date().toISOString().slice(0, 7);
  const months = eventMonths(allEvents).filter((m) => m <= currentMonth);

  const [dailyPerMonth, active, budgetDays] = await Promise.all([
    Promise.all(months.map((m) => getBillingModelDaily(token, m).catch(() => ({ usage: [] })))),
    getAdminActiveUsers(token).catch(() => []),
    getAdminBudgetDays(token).catch(() => null),
  ]);

  const effects = eventEffects({
    events: allEvents,
    daily: dailyPerMonth.flatMap((d) => d.usage),
    active,
    budgetDays,
    forecast,
  });

  return (
    <InsightSection id="styringsgrep" title="Effekt av styringsgrep">
      <BodyShort>14 dager før mot 14 dager etter hver hendelse, justert for ukedag.</BodyShort>
      <CostEventsView effects={effects} />
    </InsightSection>
  );
}
