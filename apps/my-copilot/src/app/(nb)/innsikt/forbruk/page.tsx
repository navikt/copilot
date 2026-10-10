import { Suspense, cache } from "react";
import type { Metadata } from "next";
import { Alert, BodyShort, List, Skeleton } from "@navikt/ds-react";
import { ListItem } from "@navikt/ds-react/List";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@navikt/ds-react/Table";
import { InsightPage, InsightSection } from "@/components/insight-page";
import ErrorState from "@/components/error-state";
import { SpendBandChart, SpendForecastChart } from "@/components/charts/SpendBandCharts";
import { getSpendBands } from "@/lib/cached-bigquery";
import { getUser, getUserToken } from "@/lib/auth";
import { formatNumber, formatShare, formatUSD } from "@/lib/format";
import { monthLabel } from "@/lib/trends";
import type { SpendBands } from "@/lib/types";

// Internal page: not linked from the overview, and never public, since costs are internal.
export const metadata: Metadata = {
  title: "Forbruk mot grensen",
  description: "Hvor stor del av forbruksgrensen brukerne bruker per måned, med prognose.",
};

const spendBands = cache(getSpendBands);
const fallback = <Skeleton variant="rectangle" height={320} />;

async function load(token: string): Promise<SpendBands | null> {
  try {
    return await spendBands(token);
  } catch (error) {
    console.error("[forbruk] Spend bands failed:", error);
    return null;
  }
}

const uncertain = (
  <Alert variant="warning" size="small">
    Forbruk per person finnes bare fra august 2026. Prognosen bygger derfor på svært få måneder og er usikker.
  </Alert>
);

async function Bands({ token }: { token: string }) {
  const data = await load(token);
  if (!data) return <ErrorState message="Kunne ikke hente forbruk per bånd." />;
  const months = [...data.months, ...(data.current ? [{ ...data.current, projected: true }] : [])];
  if (!months.length) return <BodyShort>Ingen fakturerte måneder ennå.</BodyShort>;
  return (
    <>
      <SpendBandChart months={months} />
      <BodyShort size="small" textColor="subtle">
        Grensen ved månedsslutt:{" "}
        {months
          .map(
            (m) =>
              `${monthLabel(m.month)} ${formatUSD(m.limit_usd)}${m.limit_changed ? " (endret i løpet av måneden)" : ""}`
          )
          .join(", ")}
        . Når grensen endret seg i løpet av en måned, bruker vi grensen som gjaldt ved månedsslutt. Bånd med færre enn
        fem brukere er slått sammen med et nabobånd. Den siste stolpen er en prognose.
      </BodyShort>
    </>
  );
}

async function Current({ token }: { token: string }) {
  const data = await load(token);
  if (!data) return <ErrorState message="Kunne ikke hente prognosen." />;
  const c = data.current;
  if (!c?.bands) return <BodyShort>Ingen bruk i denne måneden ennå.</BodyShort>;
  return (
    <>
      {uncertain}
      <BodyShort>
        Ut fra {data.days} av {data.days_in_month} dager i {monthLabel(c.month)} venter vi at {formatNumber(c.users)}{" "}
        brukere fordeler seg slik ved månedsslutt, med en grense på {formatUSD(c.limit_usd)}:
      </BodyShort>
      <div className="min-w-0 overflow-x-auto">
        <Table size="small">
          <TableHeader>
            <TableRow>
              <TableHeaderCell>Andel av grensen</TableHeaderCell>
              <TableHeaderCell align="right">Brukere</TableHeaderCell>
            </TableRow>
          </TableHeader>
          <TableBody>
            {c.bands.map((b) => (
              <TableRow key={b.first}>
                <TableDataCell>{b.label}</TableDataCell>
                <TableDataCell align="right">{formatNumber(b.users)}</TableDataCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <BodyShort size="small" textColor="subtle">
        Forutsetninger: Hver bruker fortsetter i samme tempo som hittil i måneden. Vi ganger brutto bruk så langt med{" "}
        {data.days_in_month}/{data.days} og deretter med {formatShare(data.net_ratio)}, som var andelen av brutto bruk
        som ble fakturert forrige hele måned. Tidlig i måneden kan et par travle dager gi for høye tall.
      </BodyShort>
    </>
  );
}

async function Forecast({ token }: { token: string }) {
  const data = await load(token);
  if (!data) return <ErrorState message="Kunne ikke hente prognosen." />;
  if (!data.forecast.length) return <BodyShort>For lite data til en prognose ennå.</BodyShort>;
  return (
    <>
      {uncertain}
      <SpendForecastChart totals={data.totals} forecast={data.forecast} />
      <div className="min-w-0 overflow-x-auto">
        <Table size="small">
          <TableHeader>
            <TableRow>
              <TableHeaderCell>Måned</TableHeaderCell>
              <TableHeaderCell align="right">Lav</TableHeaderCell>
              <TableHeaderCell align="right">Trend</TableHeaderCell>
              <TableHeaderCell align="right">Høy</TableHeaderCell>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.forecast.map((f) => (
              <TableRow key={f.month}>
                <TableDataCell className="whitespace-nowrap">{monthLabel(f.month)}</TableDataCell>
                <TableDataCell align="right">{formatUSD(f.low_usd)}</TableDataCell>
                <TableDataCell align="right">{formatUSD(f.mid_usd)}</TableDataCell>
                <TableDataCell align="right">{formatUSD(f.high_usd)}</TableDataCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <BodyShort size="small" textColor="subtle">
        Forutsetninger:
      </BodyShort>
      <List size="small">
        <ListItem>
          Trend: en rett linje gjennom totalene per måned fra august 2026, der denne måneden er fremskrevet.
        </ListItem>
        <ListItem>Lav: forbruket holder seg på nivået denne måneden ventes å ende på.</ListItem>
        <ListItem>Høy: like langt over trenden som lav ligger under.</ListItem>
        <ListItem>Tallene er avrundet til nærmeste hundre USD.</ListItem>
      </List>
    </>
  );
}

export default async function ForbrukPage() {
  await getUser();
  const token = await getUserToken();
  if (!token) return <ErrorState message="Mangler innloggingstoken" />;

  return (
    <InsightPage
      title="Forbruk mot grensen"
      description="Hvor stor del av forbruksgrensen brukerne bruker per måned, med prognose."
      intro="Siden viser hvordan brukerne fordeler seg etter hvor mye av forbruksgrensen de har brukt ved månedsslutt, og hva forbruket ventes å bli fremover. Siden er intern."
      updated={async () => (await load(token))?.last_day}
      source={
        <>
          Tallene kommer fra <code>/usage/spend-bands</code>. Fakturert forbruk per bruker og måned (netto) finnes fra
          august 2026 og kommer med når fakturaen for måneden er lest inn. Vi teller alle med lisens, også de uten
          forbruk. For denne måneden teller vi bare brukere med Copilot-aktivitet, og forbruket er brutto bruk per dag
          omregnet til netto. Vi bruker standardgrensen per måned, fordi grensen per person ikke er tilgjengelig ennå.
          Unntak for enkeltpersoner er derfor ikke med. Siden viser bare antall per bånd, aldri enkeltpersoner, og et
          bånd har minst fem brukere.
        </>
      }
    >
      <InsightSection id="brukere-per-band" title="Brukere per andel av grensen">
        <Suspense fallback={fallback}>
          <Bands token={token} />
        </Suspense>
      </InsightSection>
      <InsightSection id="denne-maneden" title="Prognose for denne måneden">
        <Suspense fallback={fallback}>
          <Current token={token} />
        </Suspense>
      </InsightSection>
      <InsightSection id="totalforbruk" title="Totalforbruk de neste tre månedene">
        <Suspense fallback={fallback}>
          <Forecast token={token} />
        </Suspense>
      </InsightSection>
    </InsightPage>
  );
}
