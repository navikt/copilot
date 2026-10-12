import { BodyShort, Box, Detail, HGrid, Label, Link } from "@navikt/ds-react";
import { useId, type ReactNode } from "react";
import { shownModels } from "@/components/cost-forecast";
import { getCostForecast } from "@/lib/cached-bigquery";
import { formatDate, formatNumber, formatPercent, formatUSD } from "@/lib/format";
import type { CostForecast, CostForecastModel } from "@/lib/types";

const label: Record<CostForecastModel["model"], string> = { trend: "Trend", per_user: "Per person" };

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
export function CostStatusCardsView({ data: raw, anomalies }: { data: CostForecast; anomalies?: number }) {
  const data = { ...raw, models: shownModels(raw.models) };
  const used = data.net_to_date_usd;
  return (
    <>
      <h2 className="sr-only">Status for måneden</h2>
      <HGrid columns={{ xs: 1, md: 2, lg: 4 }} gap="space-16">
        <Card title="Forbruk hittil i måneden">
          <BodyShort size="large" weight="semibold">
            {formatUSD(used)}
          </BodyShort>
          <Detail textColor="subtle">Anslått netto ut fra brutto per dag.</Detail>
        </Card>
        <Card title="Prognose for måneden">
          {data.models.length ? (
            <>
              {data.models.map((m) => (
                <BodyShort key={m.model}>
                  {label[m.model]}: {range(m)}
                </BodyShort>
              ))}
              <Detail textColor="subtle">Netto ved månedsslutt. Intervallet dekker omtrent ni av ti utfall.</Detail>
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
                <BodyShort weight="semibold">Advarsel: prognosen er over budsjettet.</BodyShort>
              )}
            </>
          ) : (
            <BodyShort>
              {data.budget_status === "missing" ? "Ikke satt budsjett i GitHub." : "Kunne ikke hente budsjettet."}
            </BodyShort>
          )}
        </Card>
        <Card title="Inkluderte kreditter">
          <BodyShort>
            {data.allowance_used_up
              ? "Brukt opp denne måneden."
              : data.allowance_run_out_date
                ? `Inkluderte kreditter brukt opp ca. ${formatDate(data.allowance_run_out_date)}.`
                : "Ikke brukt opp ennå."}
          </BodyShort>
          <Detail textColor="subtle">Kredittene i lisensene deles av alle i Nav.</Detail>
        </Card>
        {anomalies !== undefined && (
          <Card title="Aktive avvik">
            <BodyShort size="large" weight="semibold">
              {formatNumber(anomalies)}
            </BodyShort>
            <Link href="#avvik">Se listen over avvik</Link>
          </Card>
        )}
      </HGrid>
    </>
  );
}

export async function CostStatusCards({ token }: { token: string }) {
  // The forecast section below shows the error; do not log amounts here.
  const data = await getCostForecast(token).catch(() => null);
  return data && <CostStatusCardsView data={data} />;
}
