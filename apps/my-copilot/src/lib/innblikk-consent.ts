import { useSyncExternalStore } from "react";

// Consent for Innblikk, kept in localStorage (strictly necessary, no cookie).
// Same shape as Aksel's consent cookie: undecided until the visitor chooses.
export const CONSENT_KEY = "ki-utvikling-consent";
const EVENT = "ki-utvikling-consent";

// Fallback when storage throws (private mode, blocked storage): the choice holds for this page load.
let memory: boolean | null = null;
let storageBroken = false;

/** true = ja, false = nei, null = no choice stored. */
export function readConsent(): boolean | null {
  let raw: string | null;
  try {
    raw = localStorage.getItem(CONSENT_KEY);
  } catch {
    return memory;
  }
  if (!raw) return storageBroken ? memory : null;
  try {
    const value = JSON.parse(raw) as { analytics?: unknown; version?: unknown };
    return value.version === 1 && typeof value.analytics === "boolean" ? value.analytics : null;
  } catch {
    return null;
  }
}

export function setConsent(analytics: boolean) {
  memory = analytics;
  try {
    localStorage.setItem(CONSENT_KEY, JSON.stringify({ analytics, version: 1, at: new Date().toISOString() }));
    // sporing.js checks both flags before every event; nav-dekoratoren's stopUmami sets sporing.disabled.
    for (const flag of ["umami.disabled", "sporing.disabled"]) {
      if (analytics) localStorage.removeItem(flag);
      else localStorage.setItem(flag, "1");
    }
    storageBroken = false;
  } catch {
    storageBroken = true;
  }
  window.dispatchEvent(new Event(EVENT));
}

function subscribe(onChange: () => void) {
  window.addEventListener(EVENT, onChange);
  window.addEventListener("storage", onChange);
  return () => {
    window.removeEventListener(EVENT, onChange);
    window.removeEventListener("storage", onChange);
  };
}

/** undefined until hydrated, so server HTML never shows the banner or the script. */
export function useConsent(): boolean | null | undefined {
  return useSyncExternalStore(subscribe, readConsent, () => undefined);
}
