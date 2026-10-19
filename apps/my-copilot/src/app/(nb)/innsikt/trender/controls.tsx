import { Checkbox } from "@navikt/ds-react";
import { QuerySelectForm } from "@/components/query-select-form";
import { PERIODS } from "@/lib/trends";

export function PeriodSelect({ value, all }: { value: string; all: boolean }) {
  return (
    <QuerySelectForm
      action="/innsikt/trender"
      formLabel="Velg periode"
      label="Periode"
      name="periode"
      value={value}
      options={PERIODS}
      after={
        <Checkbox key={String(all)} name="hendelser" value="alle" defaultChecked={all}>
          Vis alle hendelser i diagrammene
        </Checkbox>
      }
    />
  );
}
