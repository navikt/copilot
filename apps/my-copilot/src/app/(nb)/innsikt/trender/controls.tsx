import { BodyShort, Button, Checkbox, HStack, Link, ReadMore, Select } from "@navikt/ds-react";
import { formatDate } from "@/lib/format";
import { PERIODS } from "@/lib/trends";
import { CHART_ANNOTATIONS } from "../../reisen/milestones";

/** One numbered list of events for the whole page; the numbers match the markers in the charts. */
export function Events() {
  return (
    <ReadMore header={`Hendelser (${CHART_ANNOTATIONS.length})`} size="small">
      <BodyShort size="small" spacing>
        Tallene i diagrammene viser til listen. Trykk på en måned for å se hendelsen. Hendelsene viser når noe skjedde,
        ikke hva som var årsaken. Modellvalgene gjelder bare våre egne agenter, mens faktureringen gjelder hele Nav. Et
        brudd i dataene er alltid markert med et grått felt.
      </BodyShort>
      <ol className="list-decimal space-y-1 pl-6" aria-label="Hendelser">
        {CHART_ANNOTATIONS.map((a) => (
          <li key={`${a.date}-${a.label}`}>
            {formatDate(a.date)}: {a.url ? <Link href={a.url}>{a.label}</Link> : a.label}
            {a.dataBreak && " (brudd i dataene)"}
            {a.note && `. ${a.note}`}
          </li>
        ))}
      </ol>
    </ReadMore>
  );
}

export function PeriodSelect({ value, all }: { value: string; all: boolean }) {
  return (
    <form action="/innsikt/trender" method="get" aria-label="Velg periode">
      <HStack gap="space-8" align="end" wrap>
        <Select key={value} label="Periode" name="periode" defaultValue={value} size="small">
          {PERIODS.map((p) => (
            <option key={p.value} value={p.value}>
              {p.label}
            </option>
          ))}
        </Select>
        <Checkbox key={String(all)} name="hendelser" value="alle" defaultChecked={all} size="small">
          Vis alle hendelser i diagrammene
        </Checkbox>
        <Button type="submit" size="small" variant="secondary-neutral">
          Vis
        </Button>
      </HStack>
    </form>
  );
}
