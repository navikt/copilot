"use client";

import { useSearchParams, usePathname, useRouter } from "next/navigation";

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

  const currentDays = parseInt(searchParams.get("days") || "28", 10);

  const handleSelect = (days: number) => {
    const params = new URLSearchParams(searchParams.toString());
    params.set("days", String(days));
    router.push(`${pathname}?${params.toString()}`);
  };

  return (
    <div className="flex gap-1 flex-wrap">
      {TIMEFRAME_OPTIONS.map((option) => {
        const isActive = currentDays === option.value;
        return (
          <button
            key={option.value}
            onClick={() => handleSelect(option.value)}
            className={[
              "px-3 py-1 rounded text-sm border transition-colors",
              isActive
                ? "bg-blue-600 border-blue-600 text-white"
                : "bg-white border-gray-300 text-gray-700 hover:border-gray-400",
            ].join(" ")}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
