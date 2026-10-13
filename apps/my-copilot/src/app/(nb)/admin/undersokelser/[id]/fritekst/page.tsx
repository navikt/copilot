import type { Metadata } from "next";
import { Suspense } from "react";
import { notFound } from "next/navigation";
import { Alert, BodyShort, Box, Link, VStack } from "@navikt/ds-react";
import { PageHero } from "@/components/page-hero";
import ErrorState from "@/components/error-state";
import { SectionSkeleton } from "@/components/section-skeleton";
import { getUserToken, requireAdmin } from "@/lib/auth";
import { getSurveyFreetext } from "@/lib/survey";
import { FreetextList } from "./freetext-list";

export const metadata: Metadata = {
  title: "Fritekstsvar",
  description: "Fritekstsvar fra en brukerundersøkelse.",
  robots: { index: false, follow: false },
};

async function Freetext({ id }: { id: string }) {
  const token = await getUserToken();
  if (!token) return <ErrorState title="Fritekstsvar" message="Mangler innloggingstoken" />;
  const res = await getSurveyFreetext(token, id);
  if (res.status === "not-found") notFound();
  if (res.status === "error") return <ErrorState title="Fritekstsvar" message="Kunne ikke hente fritekstsvarene." />;
  return <FreetextList surveyId={id} fields={res.data.fields} />;
}

export default async function SurveyFreetextPage({ params }: { params: Promise<{ id: string }> }) {
  await requireAdmin();
  const { id } = await params;
  if (!/^[a-z0-9-]{1,64}$/.test(id)) notFound();

  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero title="Fritekstsvar" description="Svarene vises hver for seg, i tilfeldig rekkefølge." />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap="space-24">
            <Link href={`/admin/undersokelser/${id}`}>Tilbake til resultatene</Link>
            <Alert variant="warning" size="small">
              E-postadresser, telefonnumre, fødselsnumre, IP-adresser, brukernavn og lenker er maskert. Navn er ikke
              maskert. Skjul svar som kan peke ut en person.
            </Alert>
            <BodyShort size="small">
              Skjulte svar vises ikke her eller i eksporten, men ligger i databasen til de slettes.
            </BodyShort>
            <Suspense fallback={<SectionSkeleton chart={false} rows={4} />}>
              <Freetext id={id} />
            </Suspense>
          </VStack>
        </Box>
      </div>
    </main>
  );
}
