"use client";

import { useSearchParams, usePathname, useRouter } from "next/navigation";
import { ToggleGroup } from "@navikt/ds-react";

const TIMEFRAME_OPTIONS = [
  { label: "7 dager", value: 7 },
  { label: "14 dager", value: 14 },
  { label: "28 dager", value: 28 },
  { label: "90 dager", value: 90 },
  { label: "365 dager", value: 365 },
];

export default function TimeframeSelector() {
  const searchParams = useSearchParams();
  const pathname = usePathname();
  const router = useRouter();

  const requestedDays = parseInt(searchParams.get("days") || "28", 10);
  const currentDays = TIMEFRAME_OPTIONS.some((option) => option.value === requestedDays) ? requestedDays : 28;

  const handleSelect = (days: number) => {
    const params = new URLSearchParams(searchParams.toString());
    params.set("days", String(days));
    router.push(`${pathname}?${params.toString()}`);
  };

  return (
    <ToggleGroup size="small" value={String(currentDays)} onChange={(val) => handleSelect(Number(val))}>
      {TIMEFRAME_OPTIONS.map((option) => {
        return (
          <ToggleGroup.Item key={option.value} value={String(option.value)}>
            {option.label}
          </ToggleGroup.Item>
        );
      })}
    </ToggleGroup>
  );
}
