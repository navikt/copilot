"use client";

import NextLink from "next/link";
import { Alert, Link } from "@navikt/ds-react";
import type { ActiveSurveys } from "@/lib/survey";

/** A short notice that a survey is open, shown only while copilot-survey serves one. */
export function SurveyNotice({ active }: { active: ActiveSurveys }) {
  if (active.status !== "ok" || active.surveys.length === 0) return null;
  const title = active.surveys.length === 1 ? active.surveys[0].title : "En brukerundersøkelse";
  return (
    <Alert variant="info" size="small">
      {title} er åpen.{" "}
      <Link as={NextLink} href="/nav-pilot/undersokelse">
        Svar her
      </Link>{" "}
      eller med <code className="font-mono text-sm">nav-pilot survey</code>.
    </Alert>
  );
}
