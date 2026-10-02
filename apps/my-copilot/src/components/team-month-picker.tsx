"use client";

import { Button, HStack } from "@navikt/ds-react";
import Link from "next/link";
import { currentMonthUTC, previousMonth } from "@/lib/month-utils";

export default function TeamMonthPicker({ month }: { month: string }) {
  return (
    <HStack gap="space-8" align="center" wrap paddingBlock="space-16">
      <form action="/innsikt/team" method="get" aria-label="Velg måned">
        <label htmlFor="team-spend-month">Måned</label>{" "}
        <input
          key={month}
          id="team-spend-month"
          name="month"
          type="month"
          defaultValue={month}
          max={currentMonthUTC()}
        />{" "}
        <Button type="submit" size="small" variant="secondary-neutral">
          Vis måned
        </Button>
      </form>
      {month > "2026-05" && <Link href={`/innsikt/team?month=${previousMonth(month)}`}>Forrige måned</Link>}
    </HStack>
  );
}
