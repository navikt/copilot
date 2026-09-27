import { BodyShort, Box, Heading, VStack } from "@navikt/ds-react";
import NextLink from "next/link";
import type { ReactNode } from "react";
import { BackToTop } from "@/components/back-to-top";
import { PageHero } from "@/components/page-hero";
import { TableOfContents, type TocItem } from "@/components/table-of-contents";

// The frame of a nav-pilot documentation page: a label line with the group
// ("Guider", "Referanse", "Forklaring"), the title, and the table of contents
// on the right from 1280 px. The left column is kept free for the section menu.
export function DocPage({
  label,
  title,
  description,
  toc,
  wide,
  children,
}: {
  label?: string;
  title: string;
  description: string;
  toc?: TocItem[];
  /** Let the content use the full width, for pages with wide tables. */
  wide?: boolean;
  children: ReactNode;
}) {
  return (
    <main>
      <PageHero label={label} title={title} description={description} />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <div className="flex gap-12">
            <div className={`min-w-0 flex-1 ${wide ? "" : "max-w-3xl"}`}>
              <VStack gap={{ xs: "space-32", md: "space-40" }}>{children}</VStack>
            </div>
            {toc && (
              <aside className="hidden xl:block w-56 shrink-0">
                <div className="sticky top-6">
                  <TableOfContents items={toc} title="Innhold på siden" />
                </div>
              </aside>
            )}
          </div>
        </Box>
      </div>
      <BackToTop />
    </main>
  );
}

// A bullet list. Aksel's List needs List.Item, which is undefined in a server
// component (see components/aksel-table.tsx).
export function Bullets({ children }: { children: ReactNode }) {
  return (
    <VStack as="ul" gap="space-4" className="list-disc" style={{ paddingInlineStart: "var(--ax-space-20)" }}>
      {children}
    </VStack>
  );
}

// The pages in a group, for the overview pages /nav-pilot/guider and
// /nav-pilot/forklaring.
export function PageLinks({ pages }: { pages: { href: string; title: string; desc: string }[] }) {
  return (
    <VStack as="ul" gap="space-16">
      {pages.map((p) => (
        <li key={p.href}>
          <Box borderWidth="1" borderColor="neutral-subtle" borderRadius="8" padding="space-16">
            <VStack gap="space-4">
              <Heading size="small" level="2">
                <NextLink href={p.href} className={linkClass}>
                  {p.title}
                </NextLink>
              </Heading>
              <BodyShort>{p.desc}</BodyShort>
            </VStack>
          </Box>
        </li>
      ))}
    </VStack>
  );
}

export const code = "font-mono text-sm";
export const linkClass = "text-blue-600 hover:underline";
