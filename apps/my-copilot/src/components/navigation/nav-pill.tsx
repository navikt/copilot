"use client";

import { PadlockLockedIcon } from "@navikt/aksel-icons";
import NextLink from "next/link";
import type { ReactNode } from "react";

interface NavPillProps {
  href: string;
  icon: ReactNode;
  label: string;
  active?: boolean;
  locked?: boolean;
  muted?: boolean;
  prefetch?: boolean;
}

export function NavPill({ href, icon, label, active = false, locked = false, muted = false, prefetch }: NavPillProps) {
  return (
    <NextLink
      href={href}
      prefetch={prefetch}
      aria-current={active ? "page" : undefined}
      className={`inline-flex items-center gap-1.5 px-4 py-2 rounded-full text-sm no-underline transition-colors ${
        active
          ? "bg-white/25 text-white"
          : `bg-white/10 ${muted ? "text-white/80 hover:text-white" : "text-white"} hover:bg-white/20`
      }`}
    >
      {icon}
      {label}
      {locked && <PadlockLockedIcon aria-label="Krever innlogging" fontSize="0.75rem" className="opacity-60" />}
    </NextLink>
  );
}
