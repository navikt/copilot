"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";

function scrollToHash(): boolean {
  const hash = window.location.hash.slice(1);
  if (!hash) return true;

  const id = decodeURIComponent(hash);
  const target = document.getElementById(id);
  if (!target) return false;

  target.scrollIntoView({ block: "start" });
  return true;
}

export function HashAnchorScroll() {
  const pathname = usePathname();

  useEffect(() => {
    let cancelled = false;
    let attempts = 0;
    const maxAttempts = 30;

    const tryScroll = () => {
      if (cancelled) return;
      if (scrollToHash() || attempts >= maxAttempts) return;
      attempts += 1;
      window.setTimeout(tryScroll, 50);
    };

    tryScroll();

    const onHashChange = () => {
      scrollToHash();
    };

    window.addEventListener("hashchange", onHashChange);
    return () => {
      cancelled = true;
      window.removeEventListener("hashchange", onHashChange);
    };
  }, [pathname]);

  return null;
}
