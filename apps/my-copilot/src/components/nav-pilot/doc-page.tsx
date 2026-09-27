import { BodyShort, Box, Heading, VStack } from "@navikt/ds-react";
import NextLink from "next/link";
import type { ReactNode } from "react";
import { TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import { BackToTop } from "@/components/back-to-top";
import { PageHero } from "@/components/page-hero";
import { TableOfContents, type TocItem } from "@/components/table-of-contents";
import type { DocLink } from "./doc-pages";

// The frame of a nav-pilot documentation page: a label line with the group
// ("Guider", "Referanse", "Forklaring"), the title, and the table of contents
// on the right from 1280 px. The section menu comes from the (nav-pilot) layout.
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
    <main id="hovedinnhold" tabIndex={-1}>
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
export function PageLinks({ pages, level = "2" }: { pages: DocLink[]; level?: "2" | "3" }) {
  return (
    <VStack as="ul" gap="space-16">
      {pages.map((p) => (
        <li key={p.href}>
          <Box borderWidth="1" borderColor="neutral-subtle" borderRadius="8" padding="space-16">
            <VStack gap="space-4">
              <Heading size="small" level={level}>
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

// A table's header row. With stack, the roles keep the table semantics that
// the .table-stack CSS (display: block on a phone) would otherwise drop; the
// rows and cells of such a table need role="row" and role="cell" too.
export function HeaderRow({ cells, stack }: { cells: string[]; stack?: boolean }) {
  return (
    <TableHeader role={stack ? "rowgroup" : undefined}>
      <TableRow role={stack ? "row" : undefined}>
        {cells.map((c) => (
          <TableHeaderCell key={c} scope="col" role={stack ? "columnheader" : undefined}>
            {c}
          </TableHeaderCell>
        ))}
      </TableRow>
    </TableHeader>
  );
}

export const code = "font-mono text-sm";
export const linkClass = "text-blue-600 hover:underline";
