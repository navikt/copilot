"use client";

import { useEffect, useState } from "react";
import { ChevronUpIcon } from "@navikt/aksel-icons";

export function BackToTop() {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const onScroll = () => setVisible(window.scrollY > 500);
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <button
      onClick={() => window.scrollTo({ top: 0, behavior: "smooth" })}
      aria-label="Tilbake til toppen"
      className={`fixed bottom-6 right-6 z-50 rounded-full bg-gray-800 text-white p-3 shadow-lg transition-all hover:bg-gray-700 ${
        visible ? "opacity-100 translate-y-0" : "opacity-0 translate-y-4 pointer-events-none"
      }`}
    >
      <ChevronUpIcon aria-hidden fontSize="1.25rem" />
    </button>
  );
}
