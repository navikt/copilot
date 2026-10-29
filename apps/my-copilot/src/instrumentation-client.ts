import { initNaisAPMClient } from "@nais/apm/react";
import { apmOptions } from "@/lib/apm";

// The collector URL comes from the nais-* meta tags rendered in SiteShell. On
// localhost there is none, so @nais/apm sends nothing.
initNaisAPMClient(apmOptions);
