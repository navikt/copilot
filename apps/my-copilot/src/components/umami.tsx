"use client";

import Script from "next/script";
import { usePathname } from "next/navigation";
import { isPrivatePath } from "@/lib/private-paths";

// Team ResearchOps' Umami script. It posts to reops-event-proxy.nav.no, sets
// no cookies, and data-exclude-search strips query strings. It keeps the hash,
// and once loaded on a public page it also tracks client-side navigation, so
// beforeSend drops the hash and cancels events from logged-in pages.
export function umamiBeforeSend(_type: string, payload: { url?: string; referrer?: string }) {
  if (!payload.url || isPrivatePath(new URL(payload.url, location.href).pathname)) return false;
  const noHash = (u?: string) => u?.split("#")[0];
  return { ...payload, url: noHash(payload.url), referrer: noHash(payload.referrer) };
}

if (typeof window !== "undefined") {
  (window as unknown as Record<string, unknown>).umamiBeforeSend = umamiBeforeSend;
}

export function Umami({ websiteId }: { websiteId?: string }) {
  const pathname = usePathname();
  if (!websiteId || isPrivatePath(pathname)) return null;
  return (
    <Script
      src="https://cdn.nav.no/team-researchops/sporing/sporing.js"
      strategy="afterInteractive"
      data-website-id={websiteId}
      data-domains="ki-utvikling.nav.no"
      data-exclude-search="true"
      data-before-send="umamiBeforeSend"
    />
  );
}
