"use client";

import { ArrowLeftIcon, ChevronDownIcon, ChevronRightIcon, MenuHamburgerIcon } from "@navikt/aksel-icons";
import { Button, Dialog, Theme } from "@navikt/ds-react";
import NextLink from "next/link";
import { usePathname } from "next/navigation";
import { useId, useState, type MouseEvent, type ReactNode } from "react";
import NavBudgetBar from "@/components/nav-budget-bar";
import { SiteSearch } from "@/components/navigation/site-search";
import { SECTION, TOP_LINKS, activeTop, inSection, sectionGroup } from "@/lib/nav-items";

// The header menu and the nav-pilot section menu, §6.3–§6.6 in
// docs/nav-pilot-dokumentasjon-forslag.md.

export interface HeaderLabels {
  lang: "nb" | "en";
  tagline: string;
  mainMenu: string;
  subscription: string;
  subscriptionHref: string;
  signIn: string;
  skip: string;
  menu: string;
  back: string;
  showSection: string;
  search: string;
}

// As Aksel's header links: "page" on the page itself, "true" anywhere else in the group.
const current = (pathname: string, active: string | undefined, href: string) =>
  pathname === href ? ("page" as const) : active === href ? ("true" as const) : undefined;

export function SiteHeader({ labels, userName }: { labels: HeaderLabels; userName?: string }) {
  const pathname = usePathname();
  const active = activeTop(pathname);
  // The English pages link to the Norwegian ones.
  const hrefLang = labels.lang === "en" ? "nb" : undefined;

  return (
    <>
      <a href="#hovedinnhold" className="skip-link">
        {labels.skip}
      </a>
      <div className="flex items-center gap-8">
        <NextLink href="/" className="wordmark">
          <span className="wordmark-name">nav-pilot</span>
          <span className="wordmark-tagline">{labels.tagline}</span>
        </NextLink>
        <nav aria-label={labels.mainMenu} className="hidden lg:block mx-auto">
          <ul className="flex gap-6 list-none">
            {TOP_LINKS.map((l) => (
              <li key={l.href}>
                <NextLink
                  href={l.href}
                  hrefLang={hrefLang}
                  aria-current={current(pathname, active, l.href)}
                  className="top-link"
                >
                  {l[labels.lang]}
                </NextLink>
              </li>
            ))}
          </ul>
        </nav>
        {/* One search on every width: on phones it sits next to «Meny», one tap away, not inside the menu dialog. */}
        <div className="ml-auto lg:ml-0">
          <SiteSearch label={labels.search} />
        </div>
        <div className="hidden lg:flex items-center gap-4 text-sm">
          {userName ? (
            <>
              <NextLink href={labels.subscriptionHref} hrefLang={hrefLang} className="top-link">
                {labels.subscription}
              </NextLink>
              <NavBudgetBar />
              <span className="text-white/70 whitespace-nowrap">{userName}</span>
            </>
          ) : (
            <a href="/oauth2/login" className="top-link">
              {labels.signIn}
            </a>
          )}
        </div>
        <div className="lg:hidden">
          <MobileMenu labels={labels} userName={userName} />
        </div>
      </div>
    </>
  );
}

// Below 1024 px: an Aksel Dialog from the right with two levels, the five
// groups and the section menu, as on aksel.nav.no.
function MobileMenu({ labels, userName }: { labels: HeaderLabels; userName?: string }) {
  const pathname = usePathname();
  const active = activeTop(pathname);
  const hrefLang = labels.lang === "en" ? "nb" : undefined;
  const [open, setOpen] = useState(false);
  const [level2, setLevel2] = useState(false);
  const [swapped, setSwapped] = useState(false);
  const [shownFor, setShownFor] = useState(pathname);
  if (shownFor !== pathname) {
    // The route changed: close the panel.
    setShownFor(pathname);
    setOpen(false);
  }

  const onOpenChange = (next: boolean) => {
    if (next) {
      setLevel2(inSection(pathname));
      setSwapped(false);
    }
    setOpen(next);
  };
  const swap = (to: boolean) => {
    setLevel2(to);
    setSwapped(true);
  };
  // A click on a link closes the panel, also when the link is to the page you are on.
  const closeOnLink = (e: MouseEvent) => {
    if ((e.target as HTMLElement).closest("a")) setOpen(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <Dialog.Trigger>
        <Button variant="secondary-neutral" icon={<MenuHamburgerIcon aria-hidden />}>
          {labels.menu}
        </Button>
      </Dialog.Trigger>
      {/* The header is dark. The panel is light, like the pages, as on aksel.nav.no. */}
      <Theme theme="light" asChild>
        <Dialog.Popup position="right" width="400px" closeOnOutsideClick lang={labels.lang}>
          <Dialog.Header>
            <Dialog.Title>{labels.menu}</Dialog.Title>
          </Dialog.Header>
          <Dialog.Body onClick={closeOnLink}>
            {level2 ? (
              <nav aria-label="nav-pilot" lang="nb">
                <button type="button" className="panel-item" onClick={() => swap(false)} autoFocus={swapped}>
                  <ArrowLeftIcon aria-hidden fontSize="1.25rem" />
                  <span lang={labels.lang}>{labels.back}</span>
                </button>
                <SectionTitle />
                <SectionMenu />
              </nav>
            ) : (
              <nav aria-label={labels.mainMenu}>
                <ul className="list-none">
                  {TOP_LINKS.map((l) => (
                    <li key={l.href} className="flex border-b border-[var(--ax-border-neutral-subtle)]">
                      <NextLink
                        href={l.href}
                        hrefLang={hrefLang}
                        aria-current={current(pathname, active, l.href)}
                        className="panel-item flex-1"
                      >
                        {l[labels.lang]}
                      </NextLink>
                      {l.href === "/nav-pilot" && (
                        <button
                          type="button"
                          className="panel-item"
                          aria-label={labels.showSection}
                          onClick={() => swap(true)}
                          autoFocus={swapped}
                        >
                          <ChevronRightIcon aria-hidden fontSize="1.5rem" />
                        </button>
                      )}
                    </li>
                  ))}
                  {userName && (
                    <li className="panel-gap">
                      <NextLink href={labels.subscriptionHref} hrefLang={hrefLang} className="panel-item">
                        {labels.subscription}
                      </NextLink>
                    </li>
                  )}
                  <li className={userName ? undefined : "panel-gap"}>
                    {userName ? (
                      <span className="panel-item" style={{ fontWeight: 400 }}>
                        {userName}
                      </span>
                    ) : (
                      <a href="/oauth2/login" className="panel-item">
                        {labels.signIn}
                      </a>
                    )}
                  </li>
                </ul>
              </nav>
            )}
          </Dialog.Body>
        </Dialog.Popup>
      </Theme>
    </Dialog>
  );
}

// Disclosure groups (WAI-ARIA disclosure navigation), never role="menu".
function SectionMenu() {
  const pathname = usePathname();
  const currentGroup = sectionGroup(pathname)?.label;
  const [toggled, setToggled] = useState<Record<string, boolean>>({});
  const [shownFor, setShownFor] = useState(pathname);
  if (shownFor !== pathname) {
    // The route changed: forget what was opened or closed, so the current group opens.
    setShownFor(pathname);
    setToggled({});
  }
  const isOpen = (label: string) => toggled[label] ?? label === currentGroup;
  const idBase = useId();

  const link = (href: string, label: string) => (
    <NextLink href={href} aria-current={pathname === href ? "page" : undefined} className="section-link">
      {label}
    </NextLink>
  );

  return (
    <ul className="list-none text-sm">
      {SECTION.map((grp, n) => (
        <li key={grp.label}>
          {grp.items ? (
            <>
              <button
                type="button"
                className="section-link w-full justify-between"
                aria-expanded={isOpen(grp.label)}
                aria-controls={`${idBase}-${n}`}
                onClick={() => setToggled({ ...toggled, [grp.label]: !isOpen(grp.label) })}
              >
                {grp.label}
                <ChevronDownIcon
                  aria-hidden
                  fontSize="1.25rem"
                  className={isOpen(grp.label) ? "rotate-180 transition-transform" : "transition-transform"}
                />
              </button>
              <ul id={`${idBase}-${n}`} hidden={!isOpen(grp.label)} className="list-none section-sub">
                {grp.items.map((i) => (
                  <li key={i.href}>{link(i.href, i.label)}</li>
                ))}
              </ul>
            </>
          ) : (
            link(grp.href!, grp.label)
          )}
        </li>
      ))}
    </ul>
  );
}

const SectionTitle = () => (
  <NextLink href="/nav-pilot" className="section-title">
    nav-pilot
  </NextLink>
);

/** The section menu on the left, for the (nav-pilot) route group. */
export function SectionLayout({ children }: { children: ReactNode }) {
  return (
    <div className="max-w-7xl mx-auto lg:flex">
      <nav aria-label="nav-pilot" className="hidden lg:block w-60 shrink-0 section-nav">
        <div className="sticky top-4">
          <SectionTitle />
          <SectionMenu />
        </div>
      </nav>
      <div className="flex-1 min-w-0">{children}</div>
    </div>
  );
}
