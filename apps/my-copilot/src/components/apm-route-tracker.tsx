"use client";

import { usePathname, useSearchParams } from "next/navigation";
import { useApmRouteTracking } from "@nais/apm/react";

// Reports a Faro route-change event on every App Router navigation. Mounted in
// the root layout under <Suspense> because useSearchParams() requires it.
export default function ApmRouteTracker() {
  useApmRouteTracking(usePathname(), useSearchParams());
  return null;
}
