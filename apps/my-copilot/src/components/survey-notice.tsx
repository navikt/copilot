"use client";

import { Suspense, use } from "react";
import { Alert, Link } from "@navikt/ds-react";

/** A short notice that a survey is open, shown only while copilot-survey serves one (openSurveyTitles). */
export function SurveyNotice({ titles, className }: { titles: Promise<string[]>; className?: string }) {
  return (
    <Suspense fallback={null}>
      <Notice titles={titles} className={className} />
    </Suspense>
  );
}

function Notice({ titles, className }: { titles: Promise<string[]>; className?: string }) {
  const open = use(titles);
  if (open.length === 0) return null;
  return (
    <div className={className}>
      <Alert variant="info" size="small">
        {open.length === 1 ? open[0] : "En brukerundersøkelse"} er åpen.{" "}
        <Link href="/nav-pilot/undersokelse">Svar her</Link> eller med{" "}
        <code className="font-mono text-sm">nav-pilot survey</code>.
      </Alert>
    </div>
  );
}
