"use client";

import { BodyShort } from "@navikt/ds-react";
import { useEffect, useRef, useState } from "react";

const messages = [
  "Bygget med GitHub Copilot",
  "Skrevet av mennesker, assistert av AI",
  "Koden bak denne siden er åpen kildekode",
  "Laget med ☕ og Copilot",
  "Kontinuerlig forbedret, én PR om gangen",
];

export function FooterMessage() {
  const [message, setMessage] = useState(messages[0]);
  const initialized = useRef(false);

  useEffect(() => {
    if (!initialized.current) {
      initialized.current = true;
      // eslint-disable-next-line react-hooks/set-state-in-effect -- intentional: randomize on client mount to avoid SSR mismatch
      setMessage(messages[Math.floor(Math.random() * messages.length)]);
    }
  }, []);

  return (
    <BodyShort size="small" className="text-gray-400" suppressHydrationWarning>
      {message}
    </BodyShort>
  );
}
