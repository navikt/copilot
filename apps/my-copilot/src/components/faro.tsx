"use client";

import { useEffect } from "react";
import { faro, getWebInstrumentations, initializeFaro } from "@grafana/faro-web-sdk";
import { TracingInstrumentation } from "@grafana/faro-web-tracing";

const PII_PATTERN = /\b\d{11}\b/g;

// Hosts we propagate W3C trace headers to. Anchored at the start of the URL
// and terminated at a host boundary (path, port, or end of string) so that
// lookalike hosts such as https://x.nav.no.evil.com do NOT match and trace
// headers never leak to arbitrary origins (CodeQL alert #31).
export const propagateTraceHeaderCorsUrls = [
  /^https:\/\/([a-z0-9-]+\.)*nav\.no(\/|:|$)/,
  /^https:\/\/([a-z0-9-]+\.)*nav\.cloud\.nais\.io(\/|:|$)/,
];

// Crypto-wallet browser extensions (MetaMask and others that inject
// window.ethereum) throw into every page they run on. Not our errors; Faro
// matches these against the message, name and stack of each error.
export const ignoreErrors = [/MetaMask/i, /\bethereum\b/i];

function sanitizeUrl(url: string): string {
  return url.replace(PII_PATTERN, "[REDACTED]");
}

export default function Faro({ collectorUrl }: { collectorUrl?: string }) {
  useEffect(() => {
    if (faro.config) return;

    try {
      initializeFaro({
        url: collectorUrl || "https://telemetry.nav.no/collect",
        paused: window.location.hostname === "localhost",
        app: {
          name: "my-copilot",
          namespace: "copilot",
          version: process.env.NEXT_PUBLIC_APP_VERSION || "unknown",
        },
        ignoreErrors,
        beforeSend: (event) => {
          if (event.meta.page?.url) {
            event.meta.page.url = sanitizeUrl(event.meta.page.url);
          }
          return event;
        },
        instrumentations: [
          ...getWebInstrumentations({
            captureConsole: true,
          }),
          new TracingInstrumentation({
            instrumentationOptions: {
              propagateTraceHeaderCorsUrls,
            },
          }),
        ],
      });
    } catch (e) {
      console.warn("Faro initialization failed", e);
    }
  }, [collectorUrl]);

  return null;
}
