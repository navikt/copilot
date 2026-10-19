import { BodyShort, Link, ReadMore } from "@navikt/ds-react";
import { formatDate } from "@/lib/format";
import { CHART_ANNOTATIONS } from "@/app/(nb)/reisen/milestones";

/** One numbered list of events shared by the charts on /innsikt/bruk and /innsikt/trender; the numbers match the markers in the charts. */
export function Events() {
  return (
    <ReadMore header={`Hendelser (${CHART_ANNOTATIONS.length})`}>
      <BodyShort size="small" spacing>
        Tallene i diagrammene viser til listen. Trykk på en måned for å se hendelsen. Hendelsene viser når noe skjedde,
        ikke hva som var årsaken. Modellvalg for agentene våre gjelder bare agentene. Modellhendelser, retningslinjer og
        fakturering gjelder hele Nav. Før 26. august 2026 måtte Nav slå på nye modeller selv, så en modell kan ha kommet
        i bruk hos oss senere enn datoen i listen. Brudd i dataene er alltid markert med et grått felt.
      </BodyShort>
      <ol className="list-decimal" style={{ paddingInlineStart: "var(--ax-space-20)" }} aria-label="Hendelser">
        {CHART_ANNOTATIONS.map((a) => (
          <li key={`${a.date}-${a.label}`}>
            {formatDate(a.date)}: {a.url ? <Link href={a.url}>{a.label}</Link> : a.label}
            {a.dataBreak && " (brudd i dataene)"}
            {a.model && " (modellhendelse)"}
            {a.note && `. ${a.note}`}
          </li>
        ))}
      </ol>
    </ReadMore>
  );
}
