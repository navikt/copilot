"use client";

import { BodyShort } from "@navikt/ds-react";
import { useEffect, useRef, useState } from "react";

const MESSAGES = {
  nb: [
    "Bygget med GitHub Copilot",
    "Skrevet av mennesker, assistert av KI",
    "Koden bak denne siden er åpen kildekode",
    "Laget med ☕ og Copilot",
    "Kontinuerlig forbedret, én PR om gangen",
  ],
  en: [
    "Built with GitHub Copilot",
    "Written by people, assisted by AI",
    "The code behind this page is open source",
    "Made with ☕ and Copilot",
    "Improved one PR at a time",
  ],
};

export function FooterMessage({ lang }: { lang: "nb" | "en" }) {
  const messages = MESSAGES[lang];
  const [message, setMessage] = useState(messages[0]);
  const initialized = useRef(false);

  useEffect(() => {
    if (!initialized.current) {
      initialized.current = true;

      setMessage(messages[Math.floor(Math.random() * messages.length)]);
    }
  }, [messages]);

  return (
    <BodyShort size="small" className="text-gray-400" suppressHydrationWarning>
      {message}
    </BodyShort>
  );
}
