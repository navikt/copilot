"use client";

import { Select } from "@navikt/ds-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useTransition } from "react";

const TIMEFRAME_OPTIONS = [
  { value: "7", label: "Siste 7 dager" },
  { value: "28", label: "Siste 28 dager" },
  { value: "90", label: "Siste 90 dager" },
  { value: "0", label: "All tid" },
];

export default function TimeframeSelector() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [isPending, startTransition] = useTransition();

  const currentDays = searchParams.get("days") || "28";

  const handleChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const days = e.target.value;
    startTransition(() => {
      const params = new URLSearchParams(searchParams.toString());
      params.set("days", days);
      router.push(`?${params.toString()}`);
    });
  };

  return (
    <Select
      label="Tidsperiode"
      size="small"
      value={currentDays}
      onChange={handleChange}
      disabled={isPending}
      className="w-48"
      hideLabel
    >
      {TIMEFRAME_OPTIONS.map((option) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </Select>
  );
}
