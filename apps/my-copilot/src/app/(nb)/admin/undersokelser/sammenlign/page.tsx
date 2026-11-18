import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { Suspense } from "react";
import { Alert, Box, Link, VStack } from "@navikt/ds-react";
import { PageHero } from "@/components/page-hero";
import { SectionSkeleton } from "@/components/section-skeleton";
import ErrorState from "@/components/error-state";
import { getUserToken, requireAdmin } from "@/lib/auth";
import { getAdminSurveys, getSurveyComparison } from "@/lib/survey";
import { ComparisonPrintHeader, ComparisonView } from "../comparison";

export const metadata: Metadata = {
  title: "Sammenlign undersøkelser",
  description: "Svar fra to eller flere runder av brukerundersøkelsen, side om side.",
  robots: { index: false, follow: false },
};

async function Comparison({ token, ids }: { token: string; ids: string[] }) {
  const res = await getSurveyComparison(token, ids);
  if (res.status === "not-found") notFound();
  if (res.status === "error") return <Alert variant="error">Kunne ikke hente sammenligningen.</Alert>;
  return (
    <>
      <ComparisonPrintHeader waves={res.data.surveys} />
      <ComparisonView
        data={res.data}
        csvHref={`/api/admin/surveys/compare/export?ids=${ids.join(",")}`}
        waveHref={(id) => `/admin/undersokelser/${id}`}
        groups="I påstandene teller bare svar fra dem som bruker KI-verktøy, i alle runder."
      />
    </>
  );
}

export default async function ComparePage({ searchParams }: { searchParams: Promise<{ ids?: string }> }) {
  await requireAdmin();
  const token = await getUserToken();
  if (!token) return <ErrorState title="Sammenlign" message="Mangler innloggingstoken" />;
  let ids = (await searchParams).ids?.split(",").filter(Boolean) ?? [];
  if (ids.length === 0) {
    // Default: the two latest waves that have started in the newest wave's series, oldest first.
    const list = await getAdminSurveys(token);
    if (list.status === "not-found") notFound();
    if (list.status === "error") return <ErrorState title="Sammenlign" message="Kunne ikke hente undersøkelsene." />;
    const started = list.data.surveys
      .filter((s) => s.status !== "kommende")
      .sort((a, b) => a.starts.localeCompare(b.starts));
    const series = started.at(-1)?.series;
    // No series: nothing says which surveys belong together, so pick none.
    ids = started
      .filter((s) => series && s.series === series)
      .slice(-2)
      .map((s) => s.id);
  }

  return (
    <main id="hovedinnhold" tabIndex={-1} className="print-report survey-print">
      <div className="print-hidden">
        <PageHero
          title="Sammenlign undersøkelser"
          description="Svar fra flere runder av brukerundersøkelsen, side om side."
        />
      </div>
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap="space-24">
            <Link href="/admin/undersokelser" className="print-hidden">
              Alle undersøkelser
            </Link>
            {ids.length < 2 ? (
              <Alert variant="info">Det trengs minst to runder for å sammenligne.</Alert>
            ) : (
              <Suspense fallback={<SectionSkeleton rows={6} />}>
                <Comparison token={token} ids={ids} />
              </Suspense>
            )}
          </VStack>
        </Box>
      </div>
    </main>
  );
}
