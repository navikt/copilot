"use client";

import { Suspense, use } from "react";
import NextLink from "next/link";
import { Box } from "@navikt/ds-react";

/** The header's background (site-shell.tsx), so the bar reads as part of the menu. */
const HEADER_BACKGROUND = "#0f1825";

/**
 * A slim full-width bar that a survey is open, shown only while copilot-survey serves one (openSurveyTitles).
 * Same colour and horizontal padding as the header, so it sits flush above both dark heroes and light pages.
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
    <aside
      aria-label="Brukerundersøkelse"
      className="text-white text-sm border-b border-white/15"
      style={{ background: HEADER_BACKGROUND }}
    >
      <Box
        as="p"
        paddingBlock="space-8"
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-7xl mx-auto"
      >
        {open.length === 1 ? open[0] : "En brukerundersøkelse"} er åpen.{" "}
        <NextLink
          href="/nav-pilot/undersokelse"
          className="font-semibold underline underline-offset-2 hover:no-underline rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white"
        >
          Svar i nettleseren
        </NextLink>{" "}
        eller i terminalen med <code className="font-mono whitespace-nowrap">nav-pilot survey</code>.
      </Box>
    </aside>
  );
}
