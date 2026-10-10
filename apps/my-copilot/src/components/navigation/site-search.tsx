"use client";

import { MagnifyingGlassIcon } from "@navikt/aksel-icons";
import { BodyShort, Button, Dialog, Search, Theme, VStack } from "@navikt/ds-react";
import NextLink from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useEffectEvent, useId, useRef, useState, useSyncExternalStore } from "react";
import { NAV_PILOT_BREW_INSTALL } from "@/lib/install-commands";
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

// Quick actions: shown before any search, and on top of the hits when the
// query matches them. An action with `copy` copies the text instead of following
// the link; the href is where the text is explained.
type Hit = SearchEntry & { copy?: string };
const ACTIONS: Hit[] = [
  { href: "/abonnement", title: "Gå til mitt abonnement", context: "Snarvei", login: true },
  { href: "/innsikt", title: "Se innsikt om Copilot i Nav", context: "Snarvei" },
  {
    href: "/kom-i-gang#installer",
    title: "Kopier kommandoen som installerer nav-pilot",
    context: NAV_PILOT_BREW_INSTALL,
    copy: NAV_PILOT_BREW_INSTALL,
  },
  { href: "/ordbok", title: "Slå opp i ordboka", context: "Snarvei" },
  { href: "/verktoy", title: "Finn agenter, skills og instruksjoner", context: "Snarvei" },
  { href: "https://github.com/navikt/copilot/issues/new/choose", title: "Meld fra om feil", context: "GitHub" },
];

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
  const [copied, setCopied] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const opener = useRef<Element | null>(null);
  const hint = useShortcutHint();

  const term = query.trim();
  const found = Array.isArray(index) ? searchEntries(index, term) : [];
  // The catalogue has its own search over agents, skills and instructions.
  // The last hit hands the term over to it.
  const actions = term ? searchEntries(ACTIONS, term) : ACTIONS;
  const hits: Hit[] = !term
    ? actions
    : Array.isArray(index)
      ? [
          ...actions,
          ...found,
          {
            href: `/verktoy?q=${encodeURIComponent(term)}`,
            title: `Søk etter «${term}» i verktøykatalogen`,
            context: "Tilpasning",
          },
        ]
      : actions;
  const optionId = (i: number) => `${id}-hit-${i}`;

  const onOpenChange = (next: boolean) => {
    if (next && !open) {
      opener.current = document.activeElement;
      setQuery("");
      setActive(0);
      setCopied(false);
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

  const choose = (hit: Hit) => {
    if (hit.copy) {
      navigator.clipboard.writeText(hit.copy).then(
        () => setCopied(true),
        () => router.push(hit.href) // no clipboard: show the page with the command instead
      );
      return;
    }
    setOpen(false);
    router.push(hit.href);
  };

  // Back to what had focus when the dialog opened: this button, or where the shortcut was pressed.
  const returnFocus = () => {
    const el = opener.current;
    return el instanceof HTMLElement && el !== document.body && el.isConnected
      ? el
      : document.getElementById(`${id}-button`);
  };

  const status = copied
    ? "Kommandoen er kopiert. Lim den inn i terminalen."
    : index === "error"
      ? "Søket er ikke tilgjengelig nå. Prøv igjen senere."
      : !index
        ? "Laster …"
        : term
          ? found.length + actions.length
            ? `${found.length + actions.length} treff`
            : "Ingen treff. Trykk Enter for å søke i verktøykatalogen."
          : "Snarveier";

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
              label="Søk i sider, overskrifter og nyheter"
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
                  choose(hits[active]);
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
                {hits.map((h, i) => {
                  const content = (
                    <>
                      <span className="search-hit-title">{h.title}</span>
                      <span className="search-hit-context">
                        {h.context}
                        {h.login && " · Krever innlogging"}
                      </span>
                    </>
                  );
                  const props = {
                    id: optionId(i),
                    role: "option",
                    "aria-selected": i === active,
                    tabIndex: -1,
                    onMouseMove: () => setActive(i),
                    className: "search-hit",
                  } as const;
                  return (
                    <li key={h.title + h.href} role="none">
                      {h.copy ? (
                        <button type="button" {...props} onClick={() => choose(h)}>
                          {content}
                        </button>
                      ) : (
                        <NextLink {...props} href={h.href} onClick={() => setOpen(false)}>
                          {content}
                        </NextLink>
                      )}
                    </li>
                  );
                })}
              </ul>
            </VStack>
          </Dialog.Body>
        </Dialog.Popup>
      </Theme>
    </Dialog>
  );
}
