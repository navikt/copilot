"use client";

import { MagnifyingGlassIcon } from "@navikt/aksel-icons";
import { BodyShort, Button, Dialog, Search, Theme } from "@navikt/ds-react";
import NextLink from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useEffectEvent, useId, useRef, useState, useSyncExternalStore } from "react";
import { SEARCH_INDEX_URL, searchEntries, type SearchEntry } from "@/lib/site-search";

// The site search, V11 in docs/nav-pilot-dokumentasjon-forslag.md: a «Søk»
// button in the header that opens an Aksel Dialog with a combobox, as on
// aksel.nav.no. Cmd/Ctrl+K and «/» open it too.
//
// The field is Aksel Search with the combobox roles added. Aksel's Combobox
// is a select widget: the choice stays in the field, and an option is one
// line of text.

const OPEN_EVENT = "site-search:open";

/** Opens the search dialog in the header. For the search field on the front page. */
export const openSiteSearch = () => window.dispatchEvent(new Event(OPEN_EVENT));

let indexRequest: Promise<SearchEntry[]> | undefined;
const loadIndex = () =>
  (indexRequest ??= fetch(SEARCH_INDEX_URL)
    .then((r) => (r.ok ? (r.json() as Promise<SearchEntry[]>) : Promise.reject(new Error(`${r.status}`))))
    .catch((e: unknown) => {
      indexRequest = undefined; // try again next time the dialog opens
      throw e;
    }));

const noSubscribe = () => () => {};
const useShortcutHint = () =>
  useSyncExternalStore(
    noSubscribe,
    () => (/Mac|iPhone|iPad/.test(navigator.platform) ? "⌘ K" : "Ctrl K"),
    () => "Ctrl K"
  );

const isTyping = (target: EventTarget | null) =>
  target instanceof HTMLElement && (target.isContentEditable || !!target.closest("input, textarea, select"));

export function SiteSearch({ label }: { label: string }) {
  const router = useRouter();
  const id = useId();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const [index, setIndex] = useState<SearchEntry[] | "error">();
  const inputRef = useRef<HTMLInputElement>(null);
  const opener = useRef<Element | null>(null);
  const hint = useShortcutHint();

  const hits = Array.isArray(index) ? searchEntries(index, query) : [];
  const optionId = (i: number) => `${id}-hit-${i}`;

  const onOpenChange = (next: boolean) => {
    if (next && !open) {
      opener.current = document.activeElement;
      setQuery("");
      setActive(0);
      loadIndex().then(setIndex, () => setIndex("error"));
    }
    setOpen(next);
  };

  const onKey = useEffectEvent((e: KeyboardEvent) => {
    const cmdK = e.key.toLowerCase() === "k" && (e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey;
    const slash = e.key === "/" && !e.metaKey && !e.ctrlKey && !e.altKey && !isTyping(e.target);
    if (!cmdK && !slash) return;
    e.preventDefault();
    onOpenChange(true);
  });
  const onOpenEvent = useEffectEvent(() => onOpenChange(true));
  useEffect(() => {
    const key = (e: KeyboardEvent) => onKey(e);
    const openEvent = () => onOpenEvent();
    window.addEventListener("keydown", key);
    window.addEventListener(OPEN_EVENT, openEvent);
    return () => {
      window.removeEventListener("keydown", key);
      window.removeEventListener(OPEN_EVENT, openEvent);
    };
  }, []);

  useEffect(() => {
    document.getElementById(optionId(active))?.scrollIntoView({ block: "nearest" });
  });

  // Back to what opened the dialog: this button, or the field on the front page.
  const returnFocus = () => {
    const el = opener.current;
    return el instanceof HTMLElement && el !== document.body && el.isConnected
      ? el
      : document.getElementById(`${id}-button`);
  };

  const status =
    index === "error"
      ? "Søket er ikke tilgjengelig nå. Prøv igjen senere."
      : !index
        ? "Henter søket …"
        : query.trim()
          ? `${hits.length} treff`
          : "Søk i sidene om nav-pilot og i nyhetene.";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <Dialog.Trigger>
        <Button
          id={`${id}-button`}
          variant="tertiary-neutral"
          icon={<MagnifyingGlassIcon aria-hidden />}
          aria-keyshortcuts="Meta+K Control+K /"
        >
          {label}
          <kbd aria-hidden className="search-kbd hidden lg:inline-block">
            {hint}
          </kbd>
        </Button>
      </Dialog.Trigger>
      <Theme theme="light" asChild>
        <Dialog.Popup
          width="medium"
          lang="nb"
          closeOnOutsideClick
          initialFocusTo={inputRef}
          returnFocusTo={returnFocus}
          className="site-search"
        >
          <Dialog.Header>
            <Dialog.Title>Søk</Dialog.Title>
          </Dialog.Header>
          <Dialog.Body>
            <Search
              ref={inputRef}
              label="Søk i sidene om nav-pilot og i nyhetene"
              variant="simple"
              autoComplete="off"
              role="combobox"
              aria-expanded={hits.length > 0}
              aria-controls={`${id}-list`}
              aria-autocomplete="list"
              aria-activedescendant={hits.length > 0 ? optionId(active) : undefined}
              value={query}
              onChange={(v) => {
                setQuery(v);
                setActive(0);
              }}
              onKeyDown={(e) => {
                if ((e.key === "ArrowDown" || e.key === "ArrowUp") && hits.length > 0) {
                  e.preventDefault();
                  setActive((a) => (a + (e.key === "ArrowDown" ? 1 : hits.length - 1)) % hits.length);
                } else if (e.key === "Enter" && hits[active]) {
                  e.preventDefault();
                  setOpen(false);
                  router.push(hits[active].href);
                }
              }}
            />
            <BodyShort
              role="status"
              aria-live="polite"
              size="small"
              className="mt-3 mb-1 text-[var(--ax-text-neutral-subtle)]"
            >
              {status}
            </BodyShort>
            <ul role="listbox" id={`${id}-list`} aria-label="Treff" className="list-none">
              {hits.map((h, i) => (
                <li key={h.href} role="none">
                  <NextLink
                    id={optionId(i)}
                    role="option"
                    aria-selected={i === active}
                    tabIndex={-1}
                    href={h.href}
                    onClick={() => setOpen(false)}
                    onMouseMove={() => setActive(i)}
                    className="search-hit"
                  >
                    <span className="search-hit-title">{h.title}</span>
                    <span className="search-hit-context">{h.context}</span>
                  </NextLink>
                </li>
              ))}
            </ul>
          </Dialog.Body>
        </Dialog.Popup>
      </Theme>
    </Dialog>
  );
}
