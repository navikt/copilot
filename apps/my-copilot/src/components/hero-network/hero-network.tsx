"use client";

import { useEffect, useState } from "react";
import type { HeroCluster, HeroNetworkCanvas } from "./hero-network-canvas";

// three.js only loads on the front page, after hydration, and never on the server.
// Not next/dynamic with ssr:false: its BAILOUT_TO_CLIENT_SIDE_RENDERING digest trips hack/smoke.sh.
export function HeroNetwork({ clusters }: { clusters: HeroCluster[] }) {
  const [Canvas, setCanvas] = useState<typeof HeroNetworkCanvas | null>(null);
  useEffect(() => {
    import("./hero-network-canvas").then((m) => setCanvas(() => m.HeroNetworkCanvas));
  }, []);
  return Canvas ? <Canvas clusters={clusters} /> : null;
}
