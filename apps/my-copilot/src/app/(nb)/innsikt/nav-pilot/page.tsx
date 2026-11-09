import { Suspense } from "react";
import type { Metadata } from "next";
import { getUser, getUserToken } from "@/lib/auth";
import ErrorState from "@/components/error-state";
import { ChartSkeleton } from "@/components/section-skeleton";
import { InsightPage, InsightSection } from "@/components/insight-page";
import { RuntimeTelemetry, telemetryFor } from "@/components/nav-pilot-runtime";

export const metadata: Metadata = {
  title: "nav-pilot i bruk",
  description: "Hvordan nav-pilot brukes: enheter, klienter, økter og hvilke skills agenten velger.",
};

export default async function NavPilotPage() {
  await getUser();
  const token = await getUserToken();
  if (!token) return <ErrorState message="Mangler innloggingstoken" />;

  return (
    <InsightPage
      title="nav-pilot i bruk"
      description="Bruksdata fra nav-pilot de siste syv dagene."
      intro="Her ser du hvor mange enheter som kjører nav-pilot, hvilke klienter de har satt opp, hvor mange økter de har, og hvilke skills, subagenter og verktøy som brukes mest."
      updated={async () => (await telemetryFor(token)).telemetry?.latest_day}
      source={
        <>
          Tallene kommer fra copilot-api (<code>/mimir/telemetry</code>). nav-pilot sender telemetri til Mimir,
          metrikklageret på Nais, og hver natt samles dagen i BigQuery-tabellen <code>mimir_daily</code>.
          Enhetstellingene gjelder rullende vinduer på 1, 7 eller 28 dager evaluert i Mimir, ikke kalendermåneder. En
          enhet er en maskin-ID, ikke en person eller lisens. Daglige enhetstall kan ikke summeres til unike enheter.
          Øktmetrikker for Copilot og OpenCode ble innført i kildekoden 12. oktober 2026. Det sier ikke når utviklerne
          installerte versjonen. Fravær av rader er ikke nødvendigvis null aktivitet.
        </>
      }
    >
      <InsightSection id="faktisk-bruk" title="Siste syv dager">
        <Suspense fallback={<ChartSkeleton />}>
          <RuntimeTelemetry token={token} />
        </Suspense>
      </InsightSection>
    </InsightPage>
  );
}
