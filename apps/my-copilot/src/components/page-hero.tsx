"use client";

import { Box, VStack, Heading, BodyShort } from "@navikt/ds-react";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { NAV_ITEMS } from "@/lib/nav-items";
import { NavPill } from "./navigation/nav-pill";

interface PageHeroProps {
  title: string;
  description: string;
  actions?: ReactNode;
  badge?: ReactNode;
  pathname?: string;
}

interface PageHeroBaseProps extends Omit<PageHeroProps, "pathname"> {
  pathname: string;
}

export function PageHeroBase({ title, description, actions, badge, pathname }: PageHeroBaseProps) {
  return (
    <section className="hero-gradient-subtle text-white">
      <Box
        paddingBlock={{ xs: "space-16", md: "space-20" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-7xl mx-auto"
      >
        <VStack gap="space-12">
          <div className="flex flex-col md:flex-row md:items-start md:justify-between gap-4">
            <VStack gap="space-4">
              <div className="flex items-center gap-3">
                <Heading size="large" level="1">
                  {title}
                </Heading>
                {badge}
              </div>
              <BodyShort className="max-w-2xl opacity-80">{description}</BodyShort>
            </VStack>
            {actions && <div className="shrink-0">{actions}</div>}
          </div>
          <nav aria-label="Hovednavigasjon" className="flex flex-wrap gap-2">
            {NAV_ITEMS.map(({ href, icon: Icon, label, requiresAuth }) => {
              const isActive = pathname === href || pathname.startsWith(href + "/");
              return (
                <NavPill
                  key={href}
                  href={href}
                  icon={<Icon aria-hidden fontSize="1rem" />}
                  label={label}
                  active={isActive}
                  locked={requiresAuth}
                  muted
                  prefetch={requiresAuth ? false : undefined}
                />
              );
            })}
          </nav>
        </VStack>
      </Box>
    </section>
  );
}

export function PageHero(props: PageHeroProps) {
  const currentPathname = usePathname();

  return <PageHeroBase {...props} pathname={props.pathname ?? currentPathname} />;
}
