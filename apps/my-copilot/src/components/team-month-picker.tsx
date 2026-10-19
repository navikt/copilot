"use client";

import { QuerySelectForm } from "@/components/query-select-form";
import { currentMonthUTC, previousMonth } from "@/lib/month-utils";

export default function TeamMonthPicker({ month }: { month: string }) {
  const current = currentMonthUTC();
  const months: { value: string; label: string }[] = [];
  for (let value = current; value >= "2026-05"; value = previousMonth(value))
    months.push({
      value,
      label: new Date(`${value}-01T00:00:00Z`).toLocaleDateString("nb-NO", {
        month: "long",
        year: "numeric",
        timeZone: "UTC",
      }),
    });
  return (
    <QuerySelectForm
      action="/innsikt/team"
      formLabel="Velg måned"
      label="Måned"
      name="month"
      value={month}
      options={months}
      onSubmit={(event) => {
        // Keep «Vis bare mine team» (?mine=1) when switching month.
        const mine = event.currentTarget.elements.namedItem("mine") as HTMLInputElement;
        mine.disabled = new URLSearchParams(window.location.search).get("mine") !== "1";
      }}
      before={<input type="hidden" name="mine" value="1" disabled />}
    />
  );
}
