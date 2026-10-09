"use client";

import dynamic from "next/dynamic";

// three.js only loads on the front page, after hydration, and never on the server.
export const HeroNetwork = dynamic(() => import("./hero-network-canvas").then((m) => m.HeroNetworkCanvas), {
  ssr: false,
});
