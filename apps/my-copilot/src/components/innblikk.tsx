"use client";

import Script from "next/script";
import { usePathname } from "next/navigation";
import { BodyShort, Box, Button, HStack, Link } from "@navikt/ds-react";
import { readConsent, setConsent, useConsent } from "@/lib/innblikk-consent";
import { isPrivatePath } from "@/lib/private-paths";

// Innblikk is Team ResearchOps' page analytics, based on Umami. sporing.js posts
// to reops-event-proxy.nav.no, sets no cookies, and data-exclude-search strips
// query strings. It keeps the hash, tracks client-side navigation once loaded,
// and sends the previous URL as referrer, so beforeSend drops the hash, cancels
// events from logged-in pages and drops a logged-in referrer. Nothing loads
// before the visitor says ja, as on aksel.nav.no; withdrawn consent cancels
// every further event without a reload.

// Same decoding as proxy.ts, so /%61dmin counts as /admin.
const decodePath = (p: string) => {
  try {
    return decodeURIComponent(p);
  } catch {
    return p;
  }
};

export function innblikkBeforeSend(_type: string, payload: { url?: string; referrer?: string }) {
  if (readConsent() !== true) return false;
  const path = (u: string) => decodePath(new URL(u, location.href).pathname);
  if (!payload.url || isPrivatePath(path(payload.url))) return false;
  const noHash = (u?: string) => u?.split("#")[0];
  const referrer = payload.referrer && !isPrivatePath(path(payload.referrer)) ? noHash(payload.referrer) : undefined;
  return { ...payload, url: noHash(payload.url), referrer };
}

if (typeof window !== "undefined") {
  (window as unknown as Record<string, unknown>).innblikkBeforeSend = innblikkBeforeSend;
}

const TEXT = {
  nb: {
    label: "Nettstatistikk",
    question:
      "Får vi telle hvilke sider du besøker? Vi bruker Innblikk, Navs verktøy for nettstatistikk, og kobler det ikke til deg.",
    privacy: "Les om personvern",
    yes: "Ja",
    no: "Nei",
  },
  en: {
    label: "Web statistics",
    question:
      "May we count which pages you visit? We use Innblikk, Nav's web statistics tool, and do not link it to you.",
    privacy: "Read about privacy (Norwegian)",
    yes: "Yes",
    no: "No",
  },
};

function choose(analytics: boolean) {
  setConsent(analytics);
  // The banner unmounts; hand focus to the page instead of dropping it on <body>.
  document.getElementById("hovedinnhold")?.focus();
}

export function Innblikk({ websiteId, lang = "nb" }: { websiteId?: string; lang?: "nb" | "en" }) {
  const pathname = usePathname();
  const consent = useConsent();
  if (!websiteId || isPrivatePath(decodePath(pathname)) || consent === undefined || consent === false) return null;
  if (consent === null) {
    const t = TEXT[lang];
    return (
      <Box
        as="section"
        aria-label={t.label}
        background="default"
        padding="space-16"
        borderWidth="1 0 0 0"
        className="fixed bottom-0 inset-x-0 z-50"
      >
        <HStack gap="space-16" align="center" justify="center" wrap>
          <BodyShort>
            {t.question}{" "}
            <Link href="/personvern" hrefLang={lang === "en" ? "nb" : undefined}>
              {t.privacy}
            </Link>
          </BodyShort>
          <HStack gap="space-8">
            <Button size="small" variant="secondary" onClick={() => choose(true)}>
              {t.yes}
            </Button>
            <Button size="small" variant="secondary" onClick={() => choose(false)}>
              {t.no}
            </Button>
          </HStack>
        </HStack>
      </Box>
    );
  }
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

/** For /personvern: shows the stored choice and lets the visitor change it. */
export function InnblikkChoice() {
  const consent = useConsent();
  if (consent === undefined) return null;
  const state =
    consent === true ? "Du har sagt ja." : consent === false ? "Du har sagt nei." : "Du har ikke valgt ennå.";
  return (
    <HStack gap="space-8" align="center" wrap>
      <BodyShort aria-live="polite">Nettstatistikk: {state}</BodyShort>
      <Button size="small" variant="secondary" onClick={() => setConsent(true)} aria-pressed={consent === true}>
        Ja, tell sidene jeg besøker
      </Button>
      <Button size="small" variant="secondary" onClick={() => setConsent(false)} aria-pressed={consent === false}>
        Nei
      </Button>
    </HStack>
  );
}
