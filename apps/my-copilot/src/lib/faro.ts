import { Faro, getWebInstrumentations, initializeFaro } from "@grafana/faro-web-sdk";

let faro: Faro | null = null;

export async function initInstrumentation(): Promise<void> {
  if (typeof window === "undefined" || faro !== null || process.env.NODE_ENV !== "production") return;

  await getFaro();
}

async function getFaro(): Promise<Faro> {
  if (faro != null) return faro;

  const instrumentations = [
    ...getWebInstrumentations({
      captureConsole: true,
    }),
  ];

  // Only add tracing instrumentation on client-side
  if (typeof window !== "undefined") {
    try {
      const { TracingInstrumentation } = await import("@grafana/faro-web-tracing");
      instrumentations.push(new TracingInstrumentation());
    } catch (error) {
      console.warn("Failed to load tracing instrumentation:", error);
    }
  }

  faro = initializeFaro({
    url: process.env.NEXT_PUBLIC_FARO_URL || "https://telemetry.ekstern.dev.nav.no/collect",
    app: {
      name: process.env.NEXT_PUBLIC_FARO_APP_NAME || "min-copilot",
      namespace: process.env.NEXT_PUBLIC_FARO_NAMESPACE || "nais",
    },
    instrumentations,
  });
  return faro;
}
