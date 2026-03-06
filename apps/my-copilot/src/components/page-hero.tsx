"use client";

import { Box, VStack, Heading, BodyShort } from "@navikt/ds-react";
import NextLink from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { NAV_ITEMS } from "@/lib/nav-items";

interface PageHeroProps {
  title: string;
  description: string;
  actions?: ReactNode;
}

export function PageHero({ title, description, actions }: PageHeroProps) {
  const pathname = usePathname();

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
              <Heading size="large" level="1">
                {title}
              </Heading>
              <BodyShort className="max-w-2xl opacity-80">{description}</BodyShort>
            </VStack>
            {actions && <div className="shrink-0">{actions}</div>}
          </div>
          <nav aria-label="Hovednavigasjon" className="flex flex-wrap gap-2">
            {NAV_ITEMS.map(({ href, icon: Icon, label }) => {
              const isActive = pathname === href || pathname.startsWith(href + "/");
              return (
                <NextLink
                  key={href}
                  href={href}
                  aria-current={isActive ? "page" : undefined}
                  className={`inline-flex items-center gap-1.5 px-4 py-2 rounded-full text-sm no-underline transition-colors ${
                    isActive ? "bg-white/25 text-white" : "bg-white/10 text-white/80 hover:bg-white/20 hover:text-white"
                  }`}
                >
                  <Icon aria-hidden fontSize="1rem" />
                  {label}
                </NextLink>
              );
            })}
          </nav>
        </VStack>
      </Box>
    </section>
  );
}
