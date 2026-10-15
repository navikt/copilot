import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { Alert, BodyShort, Box, Button, HStack, Link, Tag, VStack } from "@navikt/ds-react";
import { PageHero } from "@/components/page-hero";
import ErrorState from "@/components/error-state";
import { getUserToken, requireAdmin } from "@/lib/auth";
import { formatDate, formatNumber } from "@/lib/format";
import { getSurveyPreview, getSurveyResults } from "@/lib/survey";
import { DocumentTitle, PrintButton } from "../../../innsikt/team/rapport/report-controls";
import { PrintHeader } from "../print-header";
import { SurveyQuestions } from "../survey-questions";

export const metadata: Metadata = {
  title: "Resultater",
  description: "Resultater fra en brukerundersøkelse.",
  robots: { index: false, follow: false },
};

const UNDER_FIVE = "under 5";

export default async function SurveyResultsPage({ params }: { params: Promise<{ id: string }> }) {
  await requireAdmin();
  const { id } = await params;
  if (!/^[a-z0-9-]{1,64}$/.test(id)) notFound();
  const token = await getUserToken();
  if (!token) return <ErrorState title="Resultater" message="Mangler innloggingstoken" />;
  const [res, def] = await Promise.all([getSurveyResults(token, id), getSurveyPreview(id)]);
  if (res.status === "not-found") notFound();
  if (res.status === "error") return <ErrorState title="Resultater" message="Kunne ikke hente resultatene." />;
  const s = res.data;

  return (
    <main id="hovedinnhold" tabIndex={-1} className="print-report survey-print">
      <DocumentTitle title={`${s.title} – resultater`} />
      <div className="print-hidden">
        <PageHero
          title={s.title}
          description="Resultater fra brukerundersøkelsen."
          actions={<PrintButton>Lagre som PDF</PrintButton>}
        />
      </div>
      <PrintHeader
        title={s.title}
        lines={[
          `${formatDate(s.starts)}–${formatDate(s.ends)} · ${s.suppressed ? UNDER_FIVE : formatNumber(s.responses)} svar`,
        ]}
      />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap="space-24">
            <HStack gap="space-16" className="print-hidden">
              <Link href="/admin/undersokelser">Alle undersøkelser</Link>
              <Link href="/admin/undersokelser/sammenlign">Sammenlign bølger</Link>
              <Link href={`/admin/undersokelser/${id}/fritekst`}>Se fritekstsvarene</Link>
              <Button
                as="a"
                href={`/api/admin/surveys/${id}/export`}
                size="small"
                variant="secondary"
                className="print:hidden"
              >
                Last ned svar (CSV)
              </Button>
            </HStack>
            <BodyShort className="print-hidden">
              {formatDate(s.starts)}–{formatDate(s.ends)} ·{" "}
              <Tag size="small" variant="neutral">
                {s.status}
              </Tag>{" "}
              · {s.suppressed ? UNDER_FIVE : formatNumber(s.responses)} svar
            </BodyShort>
            {s.suppressed || !s.questions ? (
              <Alert variant="info">For få svar til å vise resultater ({UNDER_FIVE}).</Alert>
            ) : (
              <>
                <BodyShort size="small">
                  Spørsmål som ble endret underveis, vises med én tabell per versjon. Fritekstsvarene har egen side og
                  er ikke med i PDF-en.
                </BodyShort>
                <SurveyQuestions questions={s.questions} def={def} />
              </>
            )}
          </VStack>
        </Box>
      </div>
    </main>
  );
}
