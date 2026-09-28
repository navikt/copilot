import type { Metadata } from "next";
import NextLink from "next/link";
import { Alert, Box, BodyLong, Heading, Link, VStack } from "@navikt/ds-react";
import { Bullets } from "@/components/nav-pilot/doc-page";
import { PageHero } from "@/components/page-hero";
import { getUser } from "@/lib/auth";
import { getActiveSurveys } from "@/lib/survey";
import { SurveyForm } from "./survey-form";

export const metadata: Metadata = {
  title: "Brukerundersøkelse",
  description: "Svar på en åpen brukerundersøkelse om Copilot og nav-pilot i Nav.",
  robots: { index: false },
};

export default async function SurveyPage({ searchParams }: { searchParams: Promise<{ id?: string }> }) {
  await getUser();
  const active = await getActiveSurveys();
  const surveys = active.status === "ok" ? active.surveys : [];
  const { id } = await searchParams;
  const survey = surveys.length === 1 ? surveys[0] : surveys.find((s) => s.id === id);

  return (
    <main>
      <PageHero title="Brukerundersøkelse" description="Svar på en åpen undersøkelse om Copilot og nav-pilot i Nav." />
      <div className="max-w-3xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32" }}
        >
          {active.status === "error" && (
            <Alert variant="error">Undersøkelsene kan ikke hentes akkurat nå. Last inn siden på nytt om litt.</Alert>
          )}
          {active.status === "ok" && surveys.length === 0 && (
            <VStack gap="space-8">
              <Heading size="medium" level="2">
                Ingen åpen undersøkelse akkurat nå
              </Heading>
              <BodyLong>
                Når vi kjører en undersøkelse, kan du svare her eller i terminalen med{" "}
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
        </Box>
      </div>
    </main>
  );
}
