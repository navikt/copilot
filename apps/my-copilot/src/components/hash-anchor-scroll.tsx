"use client";

import { useEffect, useLayoutEffect } from "react";
import { usePathname } from "next/navigation";

export function HashAnchorScroll() {
  const pathname = usePathname();
  const useIsomorphicLayoutEffect = typeof window !== "undefined" ? useLayoutEffect : useEffect;

  useIsomorphicLayoutEffect(() => {
    let cancelled = false;
    let observer: MutationObserver | null = null;
    let retryTimer: number | undefined;
    let settleTimer: number | undefined;
    let attempts = 0;
    const maxAttempts = 120;

    const clearTimers = () => {
      if (retryTimer) window.clearTimeout(retryTimer);
      if (settleTimer) window.clearTimeout(settleTimer);
      retryTimer = undefined;
      settleTimer = undefined;
    };

    const tryScroll = () => {
      if (cancelled) return;
      const hash = window.location.hash.slice(1);
      if (!hash) return;

      if (attempts >= maxAttempts) {
        observer?.disconnect();
        clearTimers();
        return;
      }
      attempts += 1;

      let id = hash;
      try {
        id = decodeURIComponent(hash);
      } catch {
        // Keep raw hash if fragment is malformed.
      }

      const target = document.getElementById(id);
      if (!target) {
        if (!observer) {
          observer = new MutationObserver(() => {
            if (settleTimer) {
              window.clearTimeout(settleTimer);
              settleTimer = undefined;
            }
            tryScroll();
          });
          observer.observe(document.documentElement, { childList: true, subtree: true });
        }

        if (!retryTimer) {
          retryTimer = window.setTimeout(() => {
            retryTimer = undefined;
            tryScroll();
          }, 50);
        }
        return;
      }

      if (settleTimer) window.clearTimeout(settleTimer);
      settleTimer = window.setTimeout(() => {
        if (cancelled) return;
        const currentTarget = document.getElementById(id);
        if (!currentTarget) {
          settleTimer = undefined;
          tryScroll();
          return;
        }
        window.requestAnimationFrame(() => {
          if (cancelled) return;
          currentTarget.scrollIntoView({ block: "start" });
        });
        observer?.disconnect();
        clearTimers();
      }, 100);
    };

    const onHashChange = () => {
      clearTimers();
      tryScroll();
    };

    tryScroll();

    window.addEventListener("hashchange", onHashChange);
    return () => {
      cancelled = true;
      observer?.disconnect();
      clearTimers();
      window.removeEventListener("hashchange", onHashChange);
    };
  }, [pathname]);

  return null;
}
