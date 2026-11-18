import type { Metadata } from "next";
import { Suspense } from "react";
import { notFound } from "next/navigation";
import { Alert, Box, Link, VStack } from "@navikt/ds-react";
import { PageHero } from "@/components/page-hero";
import ErrorState from "@/components/error-state";
import { SectionSkeleton } from "@/components/section-skeleton";
import { getUserToken, requireAdmin } from "@/lib/auth";
import { getSurveyCoding } from "@/lib/survey";
import { CodingPanel } from "./coding-panel";

export const metadata: Metadata = {
  title: "Koding av fritekst",
  description: "Koding av fritekstsvar etter kodeboka.",
  robots: { index: false, follow: false },
};

async function Coding({ id }: { id: string }) {
  const token = await getUserToken();
  if (!token) return <ErrorState title="Koding av fritekst" message="Mangler innloggingstoken" />;
  const res = await getSurveyCoding(token, id);
  if (res.status === "not-found") notFound();
  if (res.status === "error") return <ErrorState title="Koding av fritekst" message="Kunne ikke hente kodingen." />;
  return <CodingPanel coding={res.data} />;
}

export default async function SurveyCodingPage({ params }: { params: Promise<{ id: string }> }) {
  await requireAdmin();
  const { id } = await params;
  if (!/^[a-z0-9-]{1,64}$/.test(id)) notFound();

  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero
        title="Koding av fritekst"
        description="Én maskert tekst om gangen. Du ser modellens koder først når du har lagret dine egne."
      />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap="space-24">
            <Link href={`/admin/undersokelser/${id}/fritekst`}>Tilbake til fritekstsvarene</Link>
            <Alert variant="info" size="small">
              Kodingen din kan ikke endres etter at du har lagret den. Der du og modellen er uenige, avgjør du, og
              avgjørelsen logges med hvem og når, uten teksten.
            </Alert>
            <Suspense fallback={<SectionSkeleton chart={false} rows={4} />}>
              <Coding id={id} />
            </Suspense>
          </VStack>
        </Box>
      </div>
    </main>
  );
}
