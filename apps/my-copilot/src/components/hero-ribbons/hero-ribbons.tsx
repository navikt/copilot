"use client";

import { useEffect, useState } from "react";
import type { HeroRibbonsCanvas } from "./hero-ribbons-canvas";

// three.js loads after hydration, never on the server. Not next/dynamic with ssr:false:
// its BAILOUT_TO_CLIENT_SIDE_RENDERING digest trips hack/smoke.sh.
export function HeroRibbons() {
  const [Canvas, setCanvas] = useState<typeof HeroRibbonsCanvas | null>(null);
  useEffect(() => {
    import("./hero-ribbons-canvas").then((m) => setCanvas(() => m.HeroRibbonsCanvas));
  }, []);
  return Canvas ? <Canvas /> : null;
}
