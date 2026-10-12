import { BodyShort, VStack } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import { InsightSection } from "@/components/insight-page";
import ErrorState from "@/components/error-state";
import { getAdminHighConsumers } from "@/lib/cached-bigquery";
import { formatDate, formatNumber, formatPercent, formatUSD } from "@/lib/format";
import { monthLabel } from "@/lib/trends";
import type { HighConsumerMonth } from "@/lib/types";

/** Base bands from copilot-api (budgetEdges); a merged band spans several columns. */
const BUDGET_BANDS = ["0–10 %", "10–25 %", "25–50 %", "50–75 %", "75–90 %", "90–100 %", "Over 100 %"];

/** F8: high consumers as groups and users per share of the personal limit. Never individuals. */
export function HighConsumersView({ months }: { months: HighConsumerMonth[] }) {
  if (!months.length) return <BodyShort>Ingen fakturerte måneder ennå.</BodyShort>;
  const tiers = [1, 5, 10];
  const withBands = months.filter((m) => m.budget_bands);
  const mtd = months.find((m) => m.basis === "hittil");
  const billed = months.filter((m) => m.basis !== "hittil");
  return (
    <VStack gap="space-16">
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
                  {m.basis === "premium" ? " (premium)" : ""}
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
        Andelen av forbruket som går til de brukerne som brukte mest. Topp 5 % er de 5 % av brukerne med høyest forbruk
        den måneden. Grupper med færre enn fem personer vises med strek, og det samme gjør en gruppe som skiller seg fra
        den forrige med færre enn fem. Januar til mai er premiumforespørsler.
      </BodyShort>

      {withBands.length > 0 && (
        <div className="min-w-0 overflow-x-auto">
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
              </TableRow>
            </TableHeader>
            <TableBody>
              {withBands.map((m) => (
                <TableRow key={m.month}>
                  <TableHeaderCell scope="row" className="whitespace-nowrap">
                    {m.basis === "hittil" && m.last_day
                      ? `Hittil i måneden (${formatDate(m.last_day)})`
                      : monthLabel(m.month)}
                  </TableHeaderCell>
                  <TableDataCell align="right">
                    {m.basis === "hittil" ? "Egen grense" : formatUSD(m.limit_usd)}
                    {m.limit_changed ? "*" : ""}
                  </TableDataCell>
                  {m.budget_bands!.map((b) => (
                    <TableDataCell key={b.first} align="right" colSpan={b.last - b.first + 1}>
                      {b.first !== b.last && <span className="text-text-subtle">{b.label}: </span>}
                      {formatPercent((b.users / m.users) * 100)}
                    </TableDataCell>
                  ))}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      <BodyShort size="small" textColor="subtle">
        Andel brukere etter hvor mye av den personlige grensen de brukte i måneden, regnet fra brutto forbruk mot
        standardgrensen ved månedsslutt for hele måneder. Unntak for enkeltpersoner er ikke med. Grupper med færre enn
        fem brukere er slått sammen med nabogruppen. * Grensen endret seg i løpet av måneden.
        {mtd &&
          ` Hittil i måneden: brutto forbruk fra den 1. mot hver brukers egen grense, uten prognose. Brukere med grense 0 er ikke med. Bare brukere med Copilot-aktivitet denne måneden er med. ${mtd.snapshots ? `${formatNumber(mtd.snapshots)} brukere har GitHubs egne tall, resten er anslag.` : "Tallene er anslag fra bruksdata."}`}
      </BodyShort>
    </VStack>
  );
}

export async function HighConsumersSection({ token }: { token: string }) {
  let months: HighConsumerMonth[];
  try {
    months = await getAdminHighConsumers(token);
  } catch (error) {
    console.error("[kostnad] Failed to fetch high consumers:", error);
    return (
      <InsightSection id="storforbrukere" title="Storforbrukere og personlig grense">
        <ErrorState message="Kunne ikke hente storforbrukere." />
      </InsightSection>
    );
  }
  return (
    <InsightSection id="storforbrukere" title="Storforbrukere og personlig grense">
      <HighConsumersView months={months} />
    </InsightSection>
  );
}
