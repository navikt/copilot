"use client";

import { Box, VStack, Heading, BodyShort } from "@navikt/ds-react";
import NextLink from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { inSection, sectionGroup } from "@/lib/nav-items";

// Praksis og regler has no section menu. GOV.UK's "Pages in this section"
// gives Retningslinjer a way in, and a way back to /praksis.
const PRAKSIS_PAGES = [
  { href: "/praksis", label: "God praksis og guider" },
  { href: "/retningslinjer", label: "Retningslinjer" },
];

interface PageHeroProps {
  /** Group name shown above the title, as on aksel.nav.no. Defaults to the section-menu group. */
  label?: string;
  title: string;
  description: string;
  actions?: ReactNode;
  badge?: ReactNode;
  pathname?: string;
}

interface PageHeroBaseProps extends Omit<PageHeroProps, "pathname"> {
  pathname: string;
}

export function PageHeroBase({ label, title, description, actions, badge, pathname }: PageHeroBaseProps) {
  // Under the nav-pilot umbrella the pages are read, not sold: light background and a label line (§6.7).
  const section = inSection(pathname);
  const kicker = label ?? (section ? sectionGroup(pathname)?.label : undefined);
  return (
    <section className={section ? undefined : "hero-gradient-subtle text-white"}>
      <Box
        paddingBlock={{ xs: "space-16", md: "space-20" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-7xl mx-auto"
      >
        <VStack gap="space-12">
          <div className="flex flex-col md:flex-row md:items-start md:justify-between gap-4">
            <VStack gap="space-4">
              {kicker && (
                <BodyShort size="small" weight="semibold" className={section ? "page-kicker" : "opacity-80"}>
                  {kicker}
                </BodyShort>
              )}
              <div className="flex items-center gap-3">
                <Heading size={section ? "xlarge" : "large"} level="1">
                  {title}
                </Heading>
                {/* Wrapped: without a loading.tsx above the page, React dev warns about a missing key on a badge passed from a server component. */}
                {badge && <span className="contents">{badge}</span>}
              </div>
              <BodyShort className="max-w-2xl opacity-80">{description}</BodyShort>
            </VStack>
            {actions && <div className="shrink-0">{actions}</div>}
          </div>
          {PRAKSIS_PAGES.some((l) => l.href === pathname) && (
            <nav aria-label="Sider i denne delen" className="flex flex-wrap gap-x-6 gap-y-2 text-sm">
              <span className="opacity-80">Sider i denne delen:</span>
              {PRAKSIS_PAGES.map((l) => (
                <NextLink
                  key={l.href}
                  href={l.href}
                  aria-current={l.href === pathname ? "page" : undefined}
                  className="text-white underline underline-offset-4 aria-[current=page]:font-semibold aria-[current=page]:no-underline"
                >
                  {l.label}
                </NextLink>
              ))}
            </nav>
          )}
        </VStack>
      </Box>
    </section>
  );
}

export function PageHero(props: PageHeroProps) {
  const currentPathname = usePathname();

  return <PageHeroBase {...props} pathname={props.pathname ?? currentPathname} />;
}
