import { useSyncExternalStore } from "react";

// Consent for Innblikk, kept in localStorage (strictly necessary, no cookie).
// Same shape as Aksel's consent cookie: undecided until the visitor chooses.
export const CONSENT_KEY = "ki-utvikling-consent";
const EVENT = "ki-utvikling-consent";

/** true = ja, false = nei, null = no choice stored. */
export function readConsent(): boolean | null {
  try {
    const raw = localStorage.getItem(CONSENT_KEY);
    if (!raw) return null;
    const value = JSON.parse(raw) as { analytics?: unknown; version?: unknown };
    return value.version === 1 && typeof value.analytics === "boolean" ? value.analytics : null;
  } catch {
    return null;
  }
}

export function setConsent(analytics: boolean) {
  localStorage.setItem(CONSENT_KEY, JSON.stringify({ analytics, version: 1, at: new Date().toISOString() }));
  // sporing.js checks both flags before every event; nav-dekoratoren's stopUmami sets sporing.disabled.
  for (const flag of ["umami.disabled", "sporing.disabled"]) {
    if (analytics) localStorage.removeItem(flag);
    else localStorage.setItem(flag, "1");
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
