"use client";

import { usePathname } from "next/navigation";
import { useApmRouteTracking } from "@nais/apm/react";

// App Router navigations never reload the page. One route_change event per
// pathname; query strings and hashes stay out of it.
export default function ApmRouteTracker() {
  useApmRouteTracking(usePathname());
  return null;
}
