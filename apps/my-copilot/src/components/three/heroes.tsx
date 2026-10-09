"use client";

import { LazyCanvas } from "./lazy-canvas";

// One literal import() per visual, so each gets its own chunk with three in it.
export const SandboxHero = () => <LazyCanvas load={() => import("./sandbox-canvas").then((m) => m.SandboxCanvas)} />;

export const CoordinatorHero = () => (
  <LazyCanvas load={() => import("./coordinator-canvas").then((m) => m.CoordinatorCanvas)} />
);

/** Takes the phase count, not PHASES, so the timeline text stays out of the client bundle. */
export const JourneyHero = ({ phases }: { phases: number }) => (
  <LazyCanvas
    load={() =>
      import("./journey-canvas").then((m) => {
        const Journey = () => <m.JourneyCanvas phases={phases} />;
        return Journey;
      })
    }
  />
);
