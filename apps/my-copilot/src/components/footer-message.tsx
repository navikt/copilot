"use client";

import { BodyShort } from "@navikt/ds-react";
import { useState } from "react";

const messages = [
  "Bygget med GitHub Copilot",
  "Skrevet av mennesker, assistert av AI",
  "Koden bak denne siden er åpen kildekode",
  "Laget med ☕ og Copilot",
  "Kontinuerlig forbedret, én PR om gangen",
];

function pickMessage() {
  return messages[Math.floor(Math.random() * messages.length)];
}

export function FooterMessage() {
  const [message] = useState(pickMessage);

  return (
    <BodyShort size="small" className="text-gray-400" suppressHydrationWarning>
      {message}
    </BodyShort>
  );
}
