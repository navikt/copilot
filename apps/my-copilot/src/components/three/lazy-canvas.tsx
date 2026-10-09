"use client";

import { useEffect, useState, type ComponentType } from "react";
import { hasWebGL2 } from "@/lib/webgl";

// three.js loads after hydration, and never on the server. Not next/dynamic with ssr:false:
// its BAILOUT_TO_CLIENT_SIDE_RENDERING digest trips hack/smoke.sh. `load` must call import()
// with a literal path so the bundler splits the canvas and three into their own chunk.
export function LazyCanvas({ load }: { load: () => Promise<ComponentType> }) {
  const [Canvas, setCanvas] = useState<ComponentType | null>(null);
  useEffect(() => {
    let live = true;
    // Probe first: without WebGL 2, three is never downloaded and the hero stays as it was.
    if (!hasWebGL2()) return;
    load().then((C) => live && setCanvas(() => C));
    return () => {
      live = false;
    };
    // load is a fresh arrow each render; it only needs to run once.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  return Canvas ? <Canvas /> : null;
}
