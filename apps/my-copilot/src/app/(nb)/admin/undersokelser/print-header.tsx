"use client";

import { useEffect, useState } from "react";
import { flushSync } from "react-dom";
import { BodyShort, Heading } from "@navikt/ds-react";

const stamp = new Intl.DateTimeFormat("nb-NO", { dateStyle: "long", timeStyle: "short", timeZone: "Europe/Oslo" });

/**
 * The first lines of the PDF: title, one line per wave (period and n), and when it was saved.
 * Print only; the screen has the page hero. The time is set at render, and again just before printing.
 */
export function PrintHeader({ title, lines }: { title: string; lines: string[] }) {
  // The server's time until beforeprint; it is the same instant in Oslo time on both sides, give or take seconds.
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    // The page is laid out for print right after beforeprint returns, so the new time must be in the DOM by then.
    const update = () => flushSync(() => setNow(new Date()));
    window.addEventListener("beforeprint", update);
    return () => window.removeEventListener("beforeprint", update);
  }, []);
  return (
    <div className="print-only" data-testid="print-header">
      <Heading level="1" size="large">
        {title}
      </Heading>
      {lines.map((l) => (
        <BodyShort key={l}>{l}</BodyShort>
      ))}
      <BodyShort size="small" suppressHydrationWarning>
        Eksportert {stamp.format(now)} · Bare aggregerte tall
      </BodyShort>
    </div>
  );
}
