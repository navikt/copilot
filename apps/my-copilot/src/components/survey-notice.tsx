"use client";

import { Suspense, use } from "react";
import NextLink from "next/link";

/**
 * A slim full-width bar that a survey is open, shown only while copilot-survey serves one (openSurveyTitles).
 * Dark like the header, so it sits flush above both dark heroes and light pages.
 */
export function SurveyNotice({ titles }: { titles: Promise<string[]> }) {
  return (
    <Suspense fallback={null}>
      <Notice titles={titles} />
    </Suspense>
  );
}

function Notice({ titles }: { titles: Promise<string[]> }) {
  const open = use(titles);
  if (open.length === 0) return null;
  return (
    <aside aria-label="Brukerundersøkelse" className="bg-gray-800 text-white text-sm border-b border-white/15">
      <p className="max-w-7xl mx-auto px-4 md:px-10 py-2">
        {open.length === 1 ? open[0] : "En brukerundersøkelse"} er åpen.{" "}
        <NextLink
          href="/nav-pilot/undersokelse"
          className="font-semibold underline underline-offset-2 hover:no-underline rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white"
        >
          Svar i nettleseren
        </NextLink>{" "}
        eller i terminalen med <code className="font-mono whitespace-nowrap">nav-pilot survey</code>.
      </p>
    </aside>
  );
}
