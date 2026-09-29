"use client";

import { MagnifyingGlassIcon } from "@navikt/aksel-icons";
import { BodyShort, Button, Dialog, Search, Theme, VStack } from "@navikt/ds-react";
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

  const term = query.trim();
  const found = Array.isArray(index) ? searchEntries(index, term) : [];
  // The catalogue has its own search over agents, skills and instructions.
  // The last hit hands the term over to it.
  const hits: SearchEntry[] =
    Array.isArray(index) && term
      ? [
          ...found,
          {
            href: `/verktoy?q=${encodeURIComponent(term)}`,
            title: `Søk etter «${term}» i verktøykatalogen`,
            context: "Tilpasning",
          },
        ]
      : [];
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
    // Not on top of another dialog, such as the «Meny» panel.
    if (!open && document.querySelector('[role="dialog"][aria-modal="true"]')) return;
    e.preventDefault();
    onOpenChange(true);
  });
  useEffect(() => {
    const key = (e: KeyboardEvent) => onKey(e);
    window.addEventListener("keydown", key);
    return () => window.removeEventListener("keydown", key);
  }, []);

  useEffect(() => {
    document.getElementById(optionId(active))?.scrollIntoView({ block: "nearest" });
  });

  // Back to what had focus when the dialog opened: this button, or where the shortcut was pressed.
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
        ? "Laster …"
        : term
          ? found.length
            ? `${found.length} treff`
            : "Ingen treff. Trykk Enter for å søke i verktøykatalogen."
          : "Søk i sidene om nav-pilot og i nyhetene.";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <Dialog.Trigger>
        <Button
          id={`${id}-button`}
          variant="tertiary-neutral"
          icon={<MagnifyingGlassIcon aria-hidden />}
          aria-keyshortcuts="Meta+K Control+K /"
          aria-label={label}
          className="whitespace-nowrap"
        >
          {/* Icon-only below 640 px: the label text is what pushes «Meny»/«Menu» past 360 px (#1293).
              `hidden` (not Tailwind's sr-only/not-sr-only pair, which Aksel's own .sr-only class
              overrides) plus aria-label above keeps the accessible name stable either way. */}
          <span aria-hidden className="hidden sm:inline">
            {label}
          </span>
          <kbd aria-hidden className="search-kbd hidden xl:inline-block">
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
                } else if (e.key === "Escape") {
                  // Aksel Search would only clear the field. Esc closes the dialog, text or not.
                  setOpen(false);
                }
              }}
            />
          </Dialog.Header>
          <Dialog.Body>
            <VStack gap="space-4">
              <BodyShort role="status" size="small" textColor="subtle">
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
            </VStack>
          </Dialog.Body>
        </Dialog.Popup>
      </Theme>
    </Dialog>
  );
}
