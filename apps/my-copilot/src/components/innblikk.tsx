"use client";

import Script from "next/script";
import { usePathname } from "next/navigation";
import { isPrivatePath } from "@/lib/private-paths";

// Innblikk is Team ResearchOps' page analytics, based on Umami. sporing.js posts
// to reops-event-proxy.nav.no, sets no cookies, and data-exclude-search strips
// query strings. It keeps the hash, tracks client-side navigation once loaded,
// and sends the previous URL as referrer, so beforeSend drops the hash, cancels
// events from logged-in pages and drops a logged-in referrer.

// Same decoding as proxy.ts, so /%61dmin counts as /admin.
const decodePath = (p: string) => {
  try {
    return decodeURIComponent(p);
  } catch {
    return p;
  }
};

export function innblikkBeforeSend(_type: string, payload: { url?: string; referrer?: string }) {
  const path = (u: string) => decodePath(new URL(u, location.href).pathname);
  if (!payload.url || isPrivatePath(path(payload.url))) return false;
  const noHash = (u?: string) => u?.split("#")[0];
  const referrer = payload.referrer && !isPrivatePath(path(payload.referrer)) ? noHash(payload.referrer) : undefined;
  return { ...payload, url: noHash(payload.url), referrer };
}

if (typeof window !== "undefined") {
  (window as unknown as Record<string, unknown>).innblikkBeforeSend = innblikkBeforeSend;
}

export function Innblikk({ websiteId }: { websiteId?: string }) {
  const pathname = usePathname();
  if (!websiteId || isPrivatePath(decodePath(pathname))) return null;
  return (
    <Script
      src="https://cdn.nav.no/team-researchops/sporing/sporing.js"
      strategy="afterInteractive"
      data-website-id={websiteId}
      data-domains="ki-utvikling.nav.no"
      data-exclude-search="true"
      data-do-not-track="true"
      data-before-send="innblikkBeforeSend"
    />
  );
}
