import { Button, Checkbox, HStack, Select } from "@navikt/ds-react";
import { PERIODS } from "@/lib/trends";

export function PeriodSelect({ value, all }: { value: string; all: boolean }) {
  return (
    <form action="/innsikt/trender" method="get" aria-label="Velg periode">
      <HStack gap="space-8" align="end" wrap>
        <Select key={value} label="Periode" name="periode" defaultValue={value}>
          {PERIODS.map((p) => (
            <option key={p.value} value={p.value}>
              {p.label}
            </option>
          ))}
        </Select>
        <Checkbox key={String(all)} name="hendelser" value="alle" defaultChecked={all}>
          Vis alle hendelser i diagrammene
        </Checkbox>
        <Button type="submit" variant="secondary-neutral">
          Vis
        </Button>
      </HStack>
    </form>
  );
}
