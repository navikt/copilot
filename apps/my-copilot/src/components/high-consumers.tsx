import { BodyShort, Heading, VStack } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import { InsightSection } from "@/components/insight-page";
import ErrorState from "@/components/error-state";
import { getAdminHighConsumers } from "@/lib/cached-bigquery";
import { formatNumber, formatPercent, formatUSD } from "@/lib/format";
import { monthLabel } from "@/lib/trends";
import type { HighConsumerMonth, SpendBand } from "@/lib/types";
import { type CostEvent, eventAnnotations, eventSummary } from "@/lib/cost-events";
import { ChartFigure, NumbersFold } from "@/components/charts/chart-figure";
import { EventChart } from "@/components/charts/EventChart";

/** Base bands from copilot-api (budgetEdges); a merged band spans several columns. */
export const BUDGET_BANDS = ["0–10 %", "10–25 %", "25–50 %", "50–75 %", "75–90 %", "90–100 %", "Over 100 %"];

/** F8: high consumers as groups and users per share of the personal limit. Never individuals. */
export function HighConsumersView({ months, events = [] }: { months: HighConsumerMonth[]; events?: CostEvent[] }) {
  if (!months.length) return <BodyShort>Ingen fakturerte måneder ennå.</BodyShort>;
  const tiers = [1, 5, 10];
  // Month to date is not comparable to full months: it gets its own panel below the chart.
  const withBands = months.filter((m) => m.budget_bands && m.basis !== "hittil");
  const mtd = months.find((m) => m.basis === "hittil");
  const billed = months.filter((m) => m.basis !== "hittil");
  const keys = withBands.map((m) => m.month);
  const markers = eventAnnotations(events, keys);
  const mtdLabel =
    mtd?.last_day &&
    `Hittil i måneden (${Number(mtd.last_day.slice(8))} av ${new Date(Date.UTC(Number(mtd.month.slice(0, 4)), Number(mtd.month.slice(5)), 0)).getUTCDate()} dager)`;
  const rowLabel = (m: HighConsumerMonth) => (m.basis === "hittil" && mtdLabel ? mtdLabel : monthLabel(m.month));
  const share = (m: HighConsumerMonth, b: SpendBand) => (b.users / m.users) * 100;
  // A merged band is drawn in its first base band, and its own label goes in the tooltip.
  const series = BUDGET_BANDS.map((label, i) => ({
    label,
    color: i,
    data: withBands.map((m) => {
      const b = m.budget_bands!.find((x) => x.first === i);
      return b ? share(m, b) : null;
    }),
    pointLabels: withBands.map((m) => m.budget_bands!.find((x) => x.first === i && x.last !== i)?.label ?? null),
  }));
  // Users the bands do not cover (no limit, say) get their own segment, so each bar adds up to 100 %.
  const unknown = withBands.map((m) => {
    const rest = m.users - m.budget_bands!.reduce((s, b) => s + b.users, 0);
    return rest >= 5 ? (rest / m.users) * 100 : null;
  });
  const hasUnknown = unknown.some((v) => v != null);
  if (hasUnknown) series.push({ label: "Ukjent grense", color: 7, data: unknown, pointLabels: [] });
  const latest = withBands.at(-1);
  const over = (m: HighConsumerMonth) =>
    m.budget_bands!.filter((b) => b.first >= 5).reduce((s, b) => s + share(m, b), 0);
  const bandTable = (rows: HighConsumerMonth[]) => (
    <Table size="small">
      <TableHeader>
        <TableRow>
          <TableHeaderCell scope="col">Måned</TableHeaderCell>
          <TableHeaderCell scope="col" align="right">
            Grense
          </TableHeaderCell>
          {BUDGET_BANDS.map((b) => (
            <TableHeaderCell key={b} scope="col" align="right">
              {b}
            </TableHeaderCell>
          ))}
          {hasUnknown && (
            <TableHeaderCell scope="col" align="right">
              Ukjent grense
            </TableHeaderCell>
          )}
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((m) => {
          const rest = m.users - m.budget_bands!.reduce((s, b) => s + b.users, 0);
          return (
            <TableRow key={m.month + m.basis}>
              <TableHeaderCell scope="row" className="whitespace-nowrap">
                {rowLabel(m)}
              </TableHeaderCell>
              <TableDataCell align="right">
                {m.basis === "hittil" ? "Egen grense" : formatUSD(m.limit_usd)}
                {m.limit_changed ? " (endret)" : ""}
              </TableDataCell>
              {m.budget_bands!.map((b) => (
                <TableDataCell key={b.first} align="right" colSpan={b.last - b.first + 1}>
                  {b.first !== b.last && <span className="text-text-subtle">{b.label}: </span>}
                  {formatPercent(share(m, b))}
                </TableDataCell>
              ))}
              {hasUnknown && (
                <TableDataCell align="right">{rest >= 5 ? formatPercent((rest / m.users) * 100) : "–"}</TableDataCell>
              )}
            </TableRow>
          );
        })}
      </TableBody>
    </Table>
  );
  return (
    <VStack gap="space-16">
      {latest && (
        <ChartFigure
          title="Hvem bruker mest av budsjettet sitt?"
          summary={
            `Andel brukere etter hvor mye av den personlige grensen de brukte, hele måneder. ${monthLabel(latest.month)}: ${formatPercent(over(latest))} brukte 90 % eller mer av grensen.` +
            eventSummary(markers)
          }
          chart={
            <EventChart
              kind="bar"
              stacked
              max={100}
              unit="pct"
              keys={keys}
              labels={withBands.map((m) => monthLabel(m.month))}
              annotations={markers}
              series={series}
            />
          }
          table={bandTable(withBands)}
        />
      )}
      {mtd?.budget_bands && mtdLabel && (
        <VStack gap="space-8">
          <Heading level="3" size="xsmall">
            {mtdLabel}
          </Heading>
          <BodyShort>
            {formatPercent(over(mtd))} har brukt 90 % eller mer av grensen sin så langt. Tallene kan ikke sammenlignes
            med hele måneder.
          </BodyShort>
          <NumbersFold summary="Fordeling hittil i måneden">{bandTable([mtd])}</NumbersFold>
        </VStack>
      )}
      <NumbersFold summary="Slik leser du tallene">
        <VStack gap="space-16">
          <BodyShort size="small" textColor="subtle">
            Andel brukere etter hvor mye av den personlige grensen de brukte i måneden, regnet fra forbruk til
            listepris. Fra juli måles hver dag mot grensen som gjaldt den dagen, og vi bruker den høyeste andelen i
            måneden. Slik teller en bruker som nådde 400 USD før grensen ble hevet til 800 USD 22. september, som 100 %,
            ikke 50 %. Grensen er GitHubs eget tall eller unntaket for personen der det finnes, ellers standardgrensen.
            Juni måles mot standardgrensen ved månedsslutt, fordi forbruket per dag først finnes fra 19. juni. Brukere
            på fakturaen uten bruksdata vises som ukjent grense. Grupper med færre enn fem brukere er slått sammen med
            nabogruppen. «Endret» betyr at grensen endret seg i løpet av måneden.
            {mtd &&
              ` Hittil i måneden: forbruk til listepris fra den 1. mot hver brukers egen grense, uten prognose. Brukere med grense 0 er ikke med. Bare brukere med Copilot-aktivitet denne måneden er med. ${mtd.snapshots ? `${formatNumber(mtd.snapshots)} brukere har GitHubs egne tall, resten er anslag.` : "Tallene er anslag fra bruksdata."}`}
          </BodyShort>

          <Heading level="3" size="xsmall">
            Hvor stor del av forbruket går til de som bruker mest?
          </Heading>
          <div className="min-w-0 overflow-x-auto">
            <Table size="small">
              <TableHeader>
                <TableRow>
                  <TableHeaderCell scope="col">Måned</TableHeaderCell>
                  <TableHeaderCell scope="col" align="right">
                    Brukere
                  </TableHeaderCell>
                  {tiers.map((p) => (
                    <TableHeaderCell key={p} scope="col" align="right">
                      Topp {p} %
                    </TableHeaderCell>
                  ))}
                </TableRow>
              </TableHeader>
              <TableBody>
                {billed.map((m) => (
                  <TableRow key={m.month}>
                    <TableHeaderCell scope="row" className="whitespace-nowrap">
                      {monthLabel(m.month)}
                      {m.basis === "premium" ? "*" : ""}
                    </TableHeaderCell>
                    <TableDataCell align="right">{m.users ? formatNumber(m.users) : "–"}</TableDataCell>
                    {tiers.map((p) => {
                      const t = m.tiers.find((x) => x.top_pct === p);
                      return (
                        <TableDataCell key={p} align="right">
                          {t?.net_share_pct != null ? formatPercent(t.net_share_pct, 1) : "–"}
                        </TableDataCell>
                      );
                    })}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <BodyShort size="small" textColor="subtle">
            Andelen av forbruket som går til de brukerne som brukte mest. Topp 5 % er de 5 % av brukerne med høyest
            forbruk den måneden. Grupper med færre enn fem personer vises med strek, og det samme gjør en gruppe som
            skiller seg fra den forrige med færre enn fem. * Fakturert som premiumforespørsler, ikke KI-kreditter.
          </BodyShort>
        </VStack>
      </NumbersFold>
    </VStack>
  );
}

export async function HighConsumersSection({ token, events }: { token: string; events?: CostEvent[] }) {
  let months: HighConsumerMonth[];
  try {
    months = await getAdminHighConsumers(token);
  } catch (error) {
    console.error("[kostnad] Failed to fetch high consumers:", error);
    return (
      <InsightSection id="storforbrukere" title="Storforbrukere">
        <ErrorState message="Kunne ikke hente storforbrukere." />
      </InsightSection>
    );
  }
  return (
    <InsightSection id="storforbrukere" title="Storforbrukere">
      <HighConsumersView months={months} events={events} />
    </InsightSection>
  );
}
