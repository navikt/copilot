import { Alert, BodyShort, VStack } from "@navikt/ds-react";
import { InsightSection } from "@/components/insight-page";
import ErrorState from "@/components/error-state";
import { getCostForecast } from "@/lib/cached-bigquery";
import { formatPercent } from "@/lib/format";
import { monthLabel } from "@/lib/trends";
import type { AdminMonth, CostForecast, CostForecastModel } from "@/lib/types";
import { CostForecastCharts } from "@/components/charts/CostForecastCharts";

/** Display names for the two forecast methods; never «modell» towards readers. */
export const forecastLabel: Record<CostForecastModel["model"], string> = {
  trend: "Nav samlet",
  per_user: "Hver person",
};

/** Owner decision: the per-person model stays hidden until its backtest error is within this, over two closed months. */
export const PER_USER_MAX_ERROR_PCT = 15;

/** The models to show. The API always sends both; per_user shows only once it has proved itself. */
export const shownModels = (models: CostForecastModel[]) =>
  models.filter(
    (m) =>
      m.model !== "per_user" ||
      ((m.backtest_months?.length ?? 0) >= 2 && Math.abs(m.backtest_error_pct) <= PER_USER_MAX_ERROR_PCT)
  );

function accuracy(m: CostForecastModel) {
  const months = m.backtest_months ?? [];
  if (!months.length) return "ingen avsluttet måned ennå";
  return `${formatPercent(m.backtest_error_pct, 1)} i snitt (${months.map(monthLabel).join(", ")})`;
}

/** F4 and F5: the over-budget warning and one line on how sure the forecast is. Cards and charts show the rest. */
export function CostForecastView({ data: raw }: { data: CostForecast }) {
  const data = { ...raw, models: shownModels(raw.models) };
  if (!data.models.length) return <BodyShort>Vi har for lite data til en prognose for denne måneden ennå.</BodyShort>;
  return (
    <VStack gap="space-8">
      {data.models.some((m) => m.over_budget) && (
        <Alert variant="warning" size="small">
          Forventet faktura for {monthLabel(data.month)} er over budsjettet i GitHub.
        </Alert>
      )}
      <BodyShort size="small" textColor="subtle">
        {data.uncertainty_label && <strong>Prognosen er {data.uncertainty_label}. </strong>}
        Anslag (bygger på {data.days_elapsed} av {data.days_in_month} dager). Avvik tidligere måneder:{" "}
        {data.models.map((m) => `${forecastLabel[m.model]} ${accuracy(m)}`).join(", ")}.
      </BodyShort>
    </VStack>
  );
}

/** How the two forecasts are made; under «Kilde og metode» on the page. */
export function ForecastMethod() {
  return (
    <BodyShort size="small">
      Nav samlet: forbruket for hele Nav følger trenden de siste 20 hverdagene, og det som ventes resten av måneden,
      fordeles etter vanlig ukedagsmønster. Hver person: hver enkelt følger sitt eget mønster fra forrige måned, justert
      etter bruken så langt, og stopper ved forbruksgrensen. Juli er ferie, så i august følger hver person sitt eget
      snitt så langt. Begge regner så ut hva som blir fakturert: Nav betaler for det som går ut over kredittene i
      lisensene, og kredittene deles av alle i Nav. Avvik tidligere måneder er snittavviket når prognosen kjøres på dag
      5, 10, 15, 20 og 25 i avsluttede måneder. For resten av året følger vi utviklingen i antall brukere og forbruk per
      bruker de siste månedene. Vi tar hensyn til ferie og viser et spenn fra flatt nivå til fortsatt vekst.
    </BodyShort>
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
    <InsightSection id="prognose" title="Prognose">
      {data ? <CostForecastView data={data} /> : <ErrorState message="Kunne ikke hente prognosen." />}
      {data && <CostForecastCharts data={data} months={months} />}
    </InsightSection>
  );
}
