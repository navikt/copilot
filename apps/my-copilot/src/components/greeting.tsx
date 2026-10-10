"use client";

import { useSyncExternalStore } from "react";

export function getGreeting(hour: number): string {
  if (hour < 6) return "God natt!";
  if (hour < 10) return "God morgen!";
  if (hour < 17) return "Hei!";
  return "God kveld!";
}

const subscribe = () => () => {};

// The server runs in UTC and cannot know the reader's hour. It renders the
// neutral «Hei!», and the browser swaps in its own greeting after hydration.
// Computing the hour during render on both sides showed the server's greeting
// («God kveld!» at midnight in Oslo) or broke hydration.
export function Greeting() {
  const greeting = useSyncExternalStore(
    subscribe,
    () => getGreeting(new Date().getHours()),
    () => getGreeting(12)
  );

  return <span className="greeting-fade">{greeting} </span>;
}
