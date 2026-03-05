"use client";

import { useState } from "react";

function getGreeting(): string {
  const hour = new Date().getHours();
  if (hour < 6) return "God natt!";
  if (hour < 10) return "God morgen!";
  if (hour < 17) return "Hei!";
  return "God kveld!";
}

export function Greeting() {
  const [greeting] = useState(getGreeting);

  return (
    <span className="greeting-fade" suppressHydrationWarning>
      {greeting}{" "}
    </span>
  );
}
