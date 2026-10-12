import type { Metadata } from "next";
import NextLink from "next/link";
import { Alert, BodyLong, Heading, Link, VStack } from "@navikt/ds-react";
import { Bullets, DocPage } from "@/components/nav-pilot/doc-page";
import { getUser } from "@/lib/auth";
import { canPreviewSurveys, getActiveSurveys, getSurveyPreview, SURVEY_SCHEDULE, surveyWindow } from "@/lib/survey";
import { SurveyForm } from "./survey-form";

export const metadata: Metadata = {
  title: "Brukerundersøkelse",
  description: "Svar på en åpen undersøkelse om Copilot og nav-pilot i Nav.",
  robots: { index: false },
};

export default async function SurveyPage({
  searchParams,
}: {
  searchParams: Promise<{ id?: string; forhandsvis?: string }>;
}) {
  const user = await getUser();
  const { id, forhandsvis } = await searchParams;
  const preview =
    forhandsvis && user && canPreviewSurveys(user.email) ? await getSurveyPreview(forhandsvis) : undefined;
  const active = await getActiveSurveys();
  const surveys = active.status === "ok" ? active.surveys : [];
  const phase = surveyWindow();
  const opens = new Date(SURVEY_SCHEDULE.starts).toLocaleDateString("nb-NO", {
    day: "numeric",
    month: "long",
    timeZone: "UTC",
  });
  const survey = surveys.length === 1 ? surveys[0] : surveys.find((s) => s.id === id);

  return (
    <DocPage title="Brukerundersøkelse" description="Svar på en åpen undersøkelse om Copilot og nav-pilot i Nav.">
      {preview ? (
        <SurveyForm survey={preview} preview />
      ) : (
        <>
          {active.status === "error" && (
            <Alert variant="error">Vi fikk ikke hentet undersøkelsene. Last inn siden på nytt om litt.</Alert>
          )}
          {active.status === "ok" && surveys.length === 0 && (
            <VStack gap="space-8">
              <Heading size="medium" level="2">
                {phase === "before"
                  ? `Neste undersøkelse åpner ${opens}`
                  : phase === "after"
                    ? "Undersøkelsen er stengt. Takk til alle som svarte."
                    : "Ingen åpen undersøkelse akkurat nå"}
              </Heading>
              <BodyLong>
                Når en undersøkelse er åpen, kan du svare her eller i terminalen med{" "}
                <code className="font-mono text-sm">nav-pilot survey</code>.
              </BodyLong>
            </VStack>
          )}
          {surveys.length > 1 && !survey && (
            <VStack gap="space-8">
              <Heading size="medium" level="2">
                Velg undersøkelse
              </Heading>
              <Bullets>
                {surveys.map((s) => (
                  <li key={s.id}>
                    <Link as={NextLink} href={`/nav-pilot/undersokelse?id=${encodeURIComponent(s.id)}`}>
                      {s.title}
                    </Link>
                  </li>
                ))}
              </Bullets>
            </VStack>
          )}
          {survey && <SurveyForm survey={survey} />}
        </>
      )}
    </DocPage>
  );
}
