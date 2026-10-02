"use client";

import { Button, HStack, Select } from "@navikt/ds-react";
import Link from "next/link";
import { currentMonthUTC, previousMonth } from "@/lib/month-utils";

export default function TeamMonthPicker({ month }: { month: string }) {
  const next = new Date(`${month}-01T00:00:00Z`);
  next.setUTCMonth(next.getUTCMonth() + 1);
  const nextMonth = next.toISOString().slice(0, 7);
  const current = currentMonthUTC();
  const months: string[] = [];
  for (let value = current; value >= "2026-05"; value = previousMonth(value)) months.push(value);
  return (
    <HStack gap="space-8" align="center" wrap paddingBlock="space-16">
      <form action="/innsikt/team" method="get" aria-label="Velg måned">
        <HStack gap="space-8" align="end">
          <Select key={month} label="Måned" name="month" defaultValue={month} size="small">
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
          <Button type="submit" size="small" variant="secondary-neutral">
            Vis
          </Button>
        </HStack>
      </form>
      {month > "2026-05" && <Link href={`/innsikt/team?month=${previousMonth(month)}`}>Forrige måned</Link>}
      {nextMonth <= current && <Link href={`/innsikt/team?month=${nextMonth}`}>Neste måned</Link>}
    </HStack>
  );
}
