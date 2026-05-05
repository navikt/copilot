"use client";

import { useEffect } from "react";
import { faro, getWebInstrumentations, initializeFaro } from "@grafana/faro-web-sdk";
import { TracingInstrumentation } from "@grafana/faro-web-tracing";

const PII_PATTERN = /\b\d{11}\b/g;

function sanitizeUrl(url: string): string {
  return url.replace(PII_PATTERN, "[REDACTED]");
}

export default function Faro({ collectorUrl }: { collectorUrl?: string }) {
  useEffect(() => {
    if (faro.api) return;

    try {
      initializeFaro({
        url: collectorUrl || "https://telemetry.nav.no/collect",
        paused: window.location.hostname === "localhost",
        app: {
          name: "my-copilot",
          namespace: "copilot",
          version: process.env.NEXT_PUBLIC_APP_VERSION || "unknown",
        },
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
              propagateTraceHeaderCorsUrls: [/https:\/\/.*\.nav\.no/, /https:\/\/.*\.nav\.cloud\.nais\.io/],
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
