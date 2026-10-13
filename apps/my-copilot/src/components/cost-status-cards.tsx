import { BodyShort, Box, Detail, HGrid, Label, Link } from "@navikt/ds-react";
import { useId, type ReactNode } from "react";
import { forecastLabel, shownModels } from "@/components/cost-forecast";
import { loadMonthInsights } from "@/components/cost-anomalies";
import { getCostForecast } from "@/lib/cached-bigquery";
import { formatDate, formatNumber, formatPercent, formatUSD } from "@/lib/format";
import { monthName } from "@/lib/team-year";
import type { CostForecast, CostForecastModel } from "@/lib/types";

const range = (r: CostForecastModel) =>
  `${formatUSD(r.net_mid_usd)} (${formatUSD(r.net_low_usd)}–${formatUSD(r.net_high_usd)})`;

function Card({ title, children }: { title: string; children: ReactNode }) {
  const id = useId();
  return (
    <Box
      as="section"
      aria-labelledby={id}
      padding="space-16"
      borderWidth="1"
      borderRadius="8"
      borderColor="neutral-subtle"
    >
      <Label as="h3" size="small" id={id}>
        {title}
      </Label>
      {children}
    </Box>
  );
}

/** F9: status cards at the top of the admin cost page. Amounts are internal: admin only, never logged. */
export function CostStatusCardsView({
  data: raw,
  insights,
  grossToDate,
}: {
  data: CostForecast;
  insights?: number;
  /** Preliminary list-price usage this month, from the monthly overview. */
  grossToDate?: number | null;
}) {
  const data = { ...raw, models: shownModels(raw.models) };
  const used = data.net_to_date_usd;
  return (
    <>
      <h2 className="sr-only">Status for måneden</h2>
      <HGrid columns={{ xs: 1, md: 2, lg: insights === undefined ? 4 : 5 }} gap="space-16">
        <Card title={`Forbruk hittil i ${monthName(data.month)}`}>
          <BodyShort size="large" weight="semibold">
            {formatUSD(used)}
          </BodyShort>
          <Detail textColor="subtle">
            Anslått fakturert beløp.
            {grossToDate != null && ` Foreløpig forbruk til listepris: ${formatUSD(grossToDate)}.`}
          </Detail>
        </Card>
        <Card title="Forventet faktura">
          {data.models.length ? (
            <>
              {data.models.map((m) => (
                <BodyShort key={m.model}>
                  {forecastLabel[m.model]}: {range(m)}
                </BodyShort>
              ))}
              <Detail textColor="subtle">Ved månedsslutt. Usikkerhetsområdet dekker omtrent ni av ti utfall.</Detail>
            </>
          ) : (
            <BodyShort>For lite data ennå.</BodyShort>
          )}
        </Card>
        <Card title="Budsjett i GitHub">
          {data.budget_status === "set" && data.budget_usd ? (
            <>
              <BodyShort size="large" weight="semibold">
                {formatUSD(data.budget_usd)} per måned
              </BodyShort>
              <BodyShort>
                Brukt: {formatUSD(used)} ({formatPercent((used / data.budget_usd) * 100)})
              </BodyShort>
              {data.models.some((m) => m.over_budget) && (
                <BodyShort weight="semibold">Forventet faktura er over budsjettet.</BodyShort>
              )}
            </>
          ) : (
            <BodyShort>
              {data.budget_status === "missing" ? "Ikke satt budsjett i GitHub." : "Kunne ikke hente budsjettet."}
            </BodyShort>
          )}
        </Card>
        <Card title="Kreditter i lisensene">
          <BodyShort>
            {data.allowance_used_up
              ? `Brukt opp${data.allowance_run_out_date ? ` ${formatDate(data.allowance_run_out_date)}` : " denne måneden"}.`
              : data.allowance_run_out_date
                ? `Blir brukt opp rundt ${formatDate(data.allowance_run_out_date)}.`
                : "Ikke brukt opp ennå."}
          </BodyShort>
          <Detail textColor="subtle">Kredittene deles av alle i Nav.</Detail>
        </Card>
        {insights !== undefined && (
          <Card title="Månedsinnsikt">
            <BodyShort size="large" weight="semibold">
              {formatNumber(insights)}
            </BodyShort>
            <Link href="#maanedsinnsikt">Se månedsinnsikten</Link>
          </Card>
        )}
      </HGrid>
    </>
  );
}

export async function CostStatusCards({ token, grossToDate }: { token: string; grossToDate?: number | null }) {
  // The forecast section below shows the error; do not log amounts here.
  const [data, insights] = await Promise.all([
    getCostForecast(token).catch(() => null),
    loadMonthInsights(token).catch(() => undefined),
  ]);
  return data && <CostStatusCardsView data={data} grossToDate={grossToDate} insights={insights?.length} />;
}
