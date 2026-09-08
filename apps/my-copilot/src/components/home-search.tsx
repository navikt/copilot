"use client";

import { Search } from "@navikt/ds-react";
import { useRouter } from "next/navigation";

/**
 * Search entry point on the front page.
 *
 * The catalogue at /verktoy has had search since it was built, and reads the
 * term from `?q=` (customization-catalog.tsx). What was missing was a way in:
 * a user had to know the tool catalogue existed, navigate to it, and only then
 * could they look for anything (#300). This submits into that same page, so
 * there is one search implementation, not two.
 */
export function HomeSearch() {
  const router = useRouter();

  return (
    <form
      role="search"
      className="max-w-md hero-animate-d2"
      onSubmit={(e) => {
        e.preventDefault();
        const q = new FormData(e.currentTarget).get("q");
        const term = typeof q === "string" ? q.trim() : "";
        router.push(term ? `/verktoy?q=${encodeURIComponent(term)}` : "/verktoy");
      }}
    >
      <Search
        name="q"
        label="Søk etter agenter, skills og instruksjoner"
        hideLabel
        placeholder="Søk etter agenter, skills og instruksjoner"
        variant="secondary"
        size="medium"
      />
    </form>
  );
}
