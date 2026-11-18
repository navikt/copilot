import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { HStack, Link } from "@navikt/ds-react";
import { InsightPage } from "@/components/insight-page";
import ErrorState from "@/components/error-state";
import { getUser, getUserToken } from "@/lib/auth";
import { getPublishedSeriesComparison } from "@/lib/survey";
import { ComparisonPrintHeader, ComparisonView } from "../../../../admin/undersokelser/comparison";

export const metadata: Metadata = {
  title: "Utvikling over tid – Undersøkelser",
  description: "Aggregerte svar fra flere runder av brukerundersøkelsen, side om side.",
  robots: { index: false, follow: false },
};

export default async function PublishedSeriesPage({ params }: { params: Promise<{ series: string }> }) {
  const { series } = await params;
  if (!/^[a-z0-9-]{1,64}$/.test(series)) notFound();
  await getUser();
  const token = await getUserToken();
  if (!token) return <ErrorState title="Utvikling over tid" message="Mangler innloggingstoken" />;
  const res = await getPublishedSeriesComparison(token, series);
  if (res.status === "not-found") notFound();
  if (res.status === "error") return <ErrorState title="Utvikling over tid" message="Kunne ikke hente tallene." />;

  return (
    <InsightPage
      title="Utvikling over tid"
      description="Aggregerte svar fra flere runder av brukerundersøkelsen, side om side."
      source={
        <>
          Tallene kommer fra <code>copilot-survey</code> og gjelder de siste fire avsluttede og publiserte rundene med
          minst fem svar. Hvert svaralternativ vises med antall og andel, og n er antallet som svarte på spørsmålet i
          runden. En versjon av et spørsmål som færre enn fem svarte på, vises uten tall. Fritekstsvar og hvilken kanal
          folk svarte i, publiseres ikke. Spørsmål eller innspill kan tas i #ki-produktutvikling på Slack.
        </>
      }
      className="print-report survey-print"
      printHeader={<ComparisonPrintHeader waves={res.data.surveys} />}
    >
      <HStack gap="space-16" className="print-hidden">
        <Link href="/innsikt/undersokelser">Alle undersøkelser</Link>
      </HStack>
      <ComparisonView
        data={res.data}
        csvHref={`/api/survey/compare/export?series=${series}`}
        waveHref={(id) => `/innsikt/undersokelser/${id}`}
        groups="Hver runde vises med sine egne tall, de samme som på resultatsiden for runden. Gruppene er derfor ikke helt like: våren 2026 viste påstandene til alle, også dem som ikke bruker KI-verktøy, mens høsten 2026 bare viste dem til dem som bruker slike verktøy. Det gjelder også testen."
      />
    </InsightPage>
  );
}
