// Next.js client-side instrumentation entry (Next 15+). This module runs once
// in the browser before the app hydrates and initializes @nais/apm browser
// telemetry (Grafana Faro under the hood).
//
// App name, team (namespace), version and collector URL are resolved from the
// `<meta name="nais-*">` tags rendered server-side in src/app/layout.tsx; app
// and namespace are also passed explicitly here as belt-and-braces so telemetry
// is always attributed to the copilot team. When no collector URL resolves
// (local dev), the SDK runs in console-echo mode and sends nothing.
import { initNaisAPMClient } from "@nais/apm/react";

import { propagateExtraOrigins } from "@/lib/apm";

initNaisAPMClient({
  app: "my-copilot",
  namespace: "copilot",
  // Opt into browser tracing so browser spans join backend traces. The SDK's
  // mandatory propagation floor (own origin + https://*.nav.no) is extended
  // with nav.cloud.nais.io ingresses.
  tracing: { propagateExtraOrigins },
});
