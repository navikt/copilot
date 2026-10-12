import { Alert, BodyShort, VStack } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@navikt/ds-react/Table";
import { InsightSection } from "@/components/insight-page";
import ErrorState from "@/components/error-state";
import { getCostForecast } from "@/lib/cached-bigquery";
import { formatPercent, formatUSD } from "@/lib/format";
import { monthLabel } from "@/lib/trends";
import type { AdminMonth, CostForecast, CostForecastModel } from "@/lib/types";
import { CostForecastCharts } from "@/components/charts/CostForecastCharts";

const label: Record<CostForecastModel["model"], string> = { trend: "Trend", per_user: "Per person" };

/** Owner decision: the per-person model stays hidden until its backtest error is within this, over two closed months. */
export const PER_USER_MAX_ERROR_PCT = 15;

/** The models to show. The API always sends both; per_user shows only once it has proved itself. */
export const shownModels = (models: CostForecastModel[]) =>
  models.filter(
    (m) =>
      m.model !== "per_user" ||
      ((m.backtest_months?.length ?? 0) >= 2 && Math.abs(m.backtest_error_pct) <= PER_USER_MAX_ERROR_PCT)
  );

const dateLabel = (day: string) =>
  new Date(`${day}T00:00:00Z`).toLocaleDateString("nb-NO", { day: "numeric", month: "long", timeZone: "UTC" });

function accuracy(m: CostForecastModel) {
  const months = m.backtest_months ?? [];
  if (!months.length) return "Ingen avsluttet måned ennå";
  return `${formatPercent(m.backtest_error_pct, 1)} i snitt (${months.map(monthLabel).join(", ")})`;
}

/** F4 and F5: month-end forecast for the admin cost page, two models side by side, compared with the budget in GitHub. */
export function CostForecastView({ data: raw }: { data: CostForecast }) {
  const data = { ...raw, models: shownModels(raw.models) };
  if (!data.models.length) return <BodyShort>Vi har for lite data til en prognose for denne måneden ennå.</BodyShort>;
  const runsOut = data.models.find((m) => m.model === "trend")?.allowance_runs_out ?? data.models[0].allowance_runs_out;
  return (
    <VStack gap="space-8">
      {data.models.some((m) => m.over_budget) && (
        <Alert variant="warning" size="small">
          Prognosen for {monthLabel(data.month)} er over budsjettet i GitHub.
        </Alert>
      )}
      <BodyShort>
        {data.budget_status === "set" && data.budget_usd
          ? `Budsjett i GitHub: ${formatUSD(data.budget_usd)}.`
          : data.budget_status === "missing"
            ? "Ikke satt budsjett i GitHub."
            : "Kunne ikke hente budsjettet fra GitHub."}
        {runsOut &&
          (data.allowance_used_up
            ? ` Inkluderte kreditter brukt opp ${dateLabel(runsOut)}.`
            : ` Inkluderte kreditter brukt opp ca. ${dateLabel(runsOut)}.`)}
      </BodyShort>
      <Table size="small">
        <TableHeader>
          <TableRow>
            <TableHeaderCell scope="col">Modell</TableHeaderCell>
            <TableHeaderCell scope="col" align="right">
              Fakturert ved månedsslutt
            </TableHeaderCell>
            <TableHeaderCell scope="col" align="right">
              Spenn
            </TableHeaderCell>
            {data.budget_status === "set" && (
              <TableHeaderCell scope="col" align="right">
                Av budsjettet
              </TableHeaderCell>
            )}
            <TableHeaderCell scope="col">Treffsikkerhet hittil</TableHeaderCell>
          </TableRow>
        </TableHeader>
        <TableBody>
          {data.models.map((m) => (
            <TableRow key={m.model}>
              <TableHeaderCell scope="row">{label[m.model]}</TableHeaderCell>
              <TableDataCell align="right">{formatUSD(m.net_mid_usd)}</TableDataCell>
              <TableDataCell align="right">
                {formatUSD(m.net_low_usd)}–{formatUSD(m.net_high_usd)}
              </TableDataCell>
              {data.budget_status === "set" && (
                <TableDataCell align="right">{formatPercent(m.budget_pct)}</TableDataCell>
              )}
              <TableDataCell>{accuracy(m)}</TableDataCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      <BodyShort size="small" textColor="subtle">
        {data.uncertainty_label && <strong>Prognosen er {data.uncertainty_label}. </strong>}
        Usikkert anslag, kalibreres videre. Prognosen bygger på {data.days_elapsed} av {data.days_in_month} dager.
        Treffsikkerhet hittil er snittavviket når modellen kjøres på dag 5, 10, 15, 20 og 25 i avsluttede måneder.
      </BodyShort>
      <BodyShort size="small" textColor="subtle">
        Trend: bruttoforbruket for hele Nav følger trenden de siste 20 hverdagene. Det trenden venter resten av måneden,
        fordeles på dagene etter vanlig ukedagsmønster, så kurven flater ut i helgene. Per person: hver person følger
        sitt eget mønster fra forrige måned, justert etter bruken så langt, og stopper ved forbruksgrensen. Juli er
        ferie, så i august følger hver person sitt eget snitt så langt. Begge regner så ut fakturert forbruk: vi betaler
        for det som går ut over kredittene som er inkludert i lisensene, og kredittene deles av alle i Nav.
      </BodyShort>
    </VStack>
  );
}

export async function CostForecastSection({ token, months = [] }: { token: string; months?: AdminMonth[] }) {
  let data: CostForecast | null = null;
  try {
    data = await getCostForecast(token);
  } catch (error) {
    console.error("[kostnad] Cost forecast failed:", error);
  }
  return (
    <InsightSection id="prognose" title="Prognose for måneden">
      {data && <CostForecastCharts data={data} months={months} />}
      {data ? <CostForecastView data={data} /> : <ErrorState message="Kunne ikke hente prognosen." />}
    </InsightSection>
  );
}
