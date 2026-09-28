"use client";

import { MagnifyingGlassIcon } from "@navikt/aksel-icons";
import { openSiteSearch } from "@/components/navigation/site-search";

/**
 * The search field on the front page. It opens the same dialog as «Søk» in
 * the header, so the site has one search (V11). The tool catalogue at
 * /verktoy keeps its own search, and the last hit hands the term over to it.
 *
 * A button that looks like a field, not a field that opens a dialog on
 * focus: moving focus must not change the context (WCAG 3.2.1).
 */
export function HomeSearch() {
  return (
    <button
      type="button"
      aria-haspopup="dialog"
      onClick={openSiteSearch}
      className="home-search max-w-md hero-animate-d2"
    >
      <MagnifyingGlassIcon aria-hidden fontSize="1.5rem" />
      Søk i sidene om nav-pilot og i nyhetene
    </button>
  );
}
