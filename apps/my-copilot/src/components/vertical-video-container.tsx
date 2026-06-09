"use client";

import { Box } from "@navikt/ds-react";
import type { ReactNode } from "react";

interface VerticalVideoContainerProps {
  children: ReactNode;
}

/**
 * Responsive vertical video container
 *
 * Mobile-first layout:
 * - xs-md (0-1024px): Vertical stack with video taking ~70% of viewport height
 * - lg+ (1024px+): Side-by-side with video on left (~55%), metadata on right (~45%)
 *
 * Handles both 9:16 (vertical) and 16:9 (horizontal) aspect ratios
 */
export function VerticalVideoContainer({ children }: VerticalVideoContainerProps) {
  return (
    <Box
      as="section"
      className="min-h-screen bg-white flex flex-col lg:flex-row lg:gap-0 w-full"
      paddingBlock={{ xs: "space-16", md: "space-24" }}
      paddingInline={{ xs: "space-16", md: "space-40" }}
    >
      {/* This Box wraps the children which includes video + metadata */}
      <Box className="flex flex-col lg:flex-row lg:gap-12 w-full">{children}</Box>
    </Box>
  );
}
