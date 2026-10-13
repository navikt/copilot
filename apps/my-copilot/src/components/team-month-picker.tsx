"use client";

import { Button, HStack, Select } from "@navikt/ds-react";
import { currentMonthUTC, previousMonth } from "@/lib/month-utils";

export default function TeamMonthPicker({ month }: { month: string }) {
  const current = currentMonthUTC();
  const months: string[] = [];
  for (let value = current; value >= "2026-05"; value = previousMonth(value)) months.push(value);
  return (
    <HStack gap="space-8" align="center" wrap>
      <form
        action="/innsikt/team"
        method="get"
        aria-label="Velg måned"
        onSubmit={(event) => {
          // Keep «Vis bare mine team» (?mine=1) when switching month.
          const mine = event.currentTarget.elements.namedItem("mine") as HTMLInputElement;
          mine.disabled = new URLSearchParams(window.location.search).get("mine") !== "1";
        }}
      >
        <input type="hidden" name="mine" value="1" disabled />
        <HStack gap="space-8" align="end">
          <Select key={month} label="Måned" name="month" defaultValue={month}>
            {months.map((value) => (
              <option key={value} value={value}>
                {new Date(`${value}-01T00:00:00Z`).toLocaleDateString("nb-NO", {
                  month: "long",
                  year: "numeric",
                  timeZone: "UTC",
                })}
              </option>
            ))}
          </Select>
          <Button type="submit" variant="secondary-neutral">
            Vis
          </Button>
        </HStack>
      </form>
    </HStack>
  );
}
