"use client";

import { useEffect, useState } from "react";
import { BodyShort, Heading } from "@navikt/ds-react";

const stamp = new Intl.DateTimeFormat("nb-NO", { dateStyle: "long", timeStyle: "short", timeZone: "Europe/Oslo" });

/**
 * The first lines of the PDF: title, one line per wave (period and n), and when it was saved.
 * Print only; the screen has the page hero. The time is set on the client, and again just before printing.
 */
export function PrintHeader({ title, lines }: { title: string; lines: string[] }) {
  const [now, setNow] = useState<Date>();
  useEffect(() => {
    const update = () => setNow(new Date());
    update();
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
      {now ? <BodyShort size="small">Eksportert {stamp.format(now)} · Bare aggregerte tall</BodyShort> : null}
    </div>
  );
}
