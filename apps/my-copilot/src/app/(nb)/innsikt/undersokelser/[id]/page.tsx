import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { Alert, BodyShort, Box, Heading, HStack, Link, VStack } from "@navikt/ds-react";
import { InsightPage } from "@/components/insight-page";
import ErrorState from "@/components/error-state";
import { getUserToken } from "@/lib/auth";
import { formatDate, formatNumber } from "@/lib/format";
import { channelName, getPublishedSurveyResults, getSurveyPreview, UNDER_FIVE } from "@/lib/survey";
import { PrintHeader } from "../../../admin/undersokelser/print-header";
import { SurveyQuestions } from "../../../admin/undersokelser/survey-questions";
import { DocumentTitle, PrintButton } from "@/components/report-controls";

export const metadata: Metadata = {
  title: "Resultater – Undersøkelser",
  description: "Aggregerte resultater fra en avsluttet brukerundersøkelse.",
  robots: { index: false, follow: false },
};

export default async function PublishedSurveyResultsPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  if (!/^[a-z0-9-]{1,64}$/.test(id)) notFound();
  const token = await getUserToken();
  if (!token) return <ErrorState title="Resultater" message="Mangler innloggingstoken" />;
  const [res, def] = await Promise.all([getPublishedSurveyResults(token, id), getSurveyPreview(id)]);
  if (res.status === "not-found") notFound();
  if (res.status === "error") return <ErrorState title="Resultater" message="Kunne ikke hente resultatene." />;
  const s = res.data;
  const channels = Object.entries(s.channels ?? {})
    .sort((a, b) => b[1] - a[1])
    .map(([c, n]) => `${channelName(c)}: ${formatNumber(n)}`)
    .join(", ");

  return (
    <InsightPage
      title={s.title}
      description="Aggregerte resultater fra brukerundersøkelsen."
      actions={<PrintButton>Lagre som PDF</PrintButton>}
      source="Aggregerte svar fra undersøkelsen. Spørsmål om resultatene kan tas i #ki-produktutvikling på Slack."
      className="print-report survey-print"
      printHeader={
        <>
          <DocumentTitle title={`${s.title} – resultater`} />
          <PrintHeader
            title={s.title}
            lines={[
              `${formatDate(s.starts)}–${formatDate(s.ends)} · ${s.suppressed ? UNDER_FIVE : formatNumber(s.responses)} svar`,
            ]}
          />
        </>
      }
    >
      <VStack gap="space-24">
        <HStack gap="space-16" className="print-hidden">
          <Link href="/innsikt/undersokelser">Alle undersøkelser</Link>
        </HStack>
        <section aria-labelledby="heading-metode">
          <Box background="neutral-soft" padding="space-16" borderRadius="8">
            <VStack gap="space-8">
              <Heading id="heading-metode" level="2" size="xsmall">
                Metode og datagrunnlag
              </Heading>
              <BodyShort size="small">
                Periode: {formatDate(s.starts)}–{formatDate(s.ends)} ·{" "}
                {s.suppressed ? UNDER_FIVE : formatNumber(s.responses)} svar
                {channels ? ` · Besvart via: ${channels}` : ""}
              </BodyShort>
              <BodyShort size="small" textColor="subtle">
                Deltakelsen var frivillig blant ansatte som bruker utviklerverktøy i Nav, så svarprosent mot en fast
                populasjon kan ikke beregnes. Spørsmål som ble endret underveis, vises med én tabell per versjon.
                Fritekstsvar publiseres ikke av hensyn til personvern.
              </BodyShort>
            </VStack>
          </Box>
        </section>
        {s.suppressed || !s.questions ? (
          <Alert variant="info">For få svar til å vise resultater ({UNDER_FIVE}).</Alert>
        ) : (
          <SurveyQuestions questions={s.questions} def={def} />
        )}
      </VStack>
    </InsightPage>
  );
}
