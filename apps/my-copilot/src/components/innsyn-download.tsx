import { Button, HStack, Select } from "@navikt/ds-react";

/** «Last ned innsynssvar» for /admin/kostnad: opens the print page for the chosen year, which also links to the JSON. */
export function InnsynDownload() {
  const thisYear = new Date().getUTCFullYear();
  const years = Array.from({ length: thisYear - 2025 }, (_, i) => thisYear - i);
  return (
    <form action="/admin/kostnad/innsyn" method="get">
      <HStack gap="space-8" align="end">
        <Select label="År for innsynssvar" name="year" size="small" defaultValue={thisYear}>
          {years.map((y) => (
            <option key={y} value={y}>
              {y === thisYear ? `${y} hittil` : y}
            </option>
          ))}
        </Select>
        <Button type="submit" size="small" variant="secondary">
          Last ned innsynssvar
        </Button>
      </HStack>
    </form>
  );
}
