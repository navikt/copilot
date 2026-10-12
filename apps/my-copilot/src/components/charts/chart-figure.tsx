import { useId, type ReactNode } from "react";
import { Heading, VStack } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@navikt/ds-react/Table";
import { chartBoxClass } from "@/lib/chart-utils";

/**
 * Chart first, then a text summary, then the numbers in a «Vis tallene» fold. The title is a question
 * the chart answers («Er vi på vei over budsjettet?»); the section heading above stays a plain name.
 * The summary labels the chart for screen readers only; the cards and the chart already say it visually.
 * Leave out `table` when charts sit side by side, and put one NumbersFold under the row instead.
 */
export function ChartFigure({
  title,
  summary,
  chart,
  table,
}: {
  title: string;
  summary: ReactNode;
  chart: ReactNode;
  /** A ready table, or the cells for a plain one: the first row is the header. */
  table?: ReactNode | string[][];
}) {
  const id = useId();
  return (
    <figure className="min-w-0 break-inside-avoid" style={{ margin: 0 }}>
      <VStack gap="space-8">
        <Heading level="3" size="xsmall">
          {title}
        </Heading>
        <div className={chartBoxClass} role="img" aria-labelledby={id}>
          {chart}
        </div>
        <figcaption id={id} className="sr-only text-sm print:not-sr-only">
          {summary}
        </figcaption>
        {table !== undefined && (
          <NumbersFold>{Array.isArray(table) ? <PlainTable rows={table as string[][]} /> : table}</NumbersFold>
        )}
      </VStack>
    </figure>
  );
}

/**
 * A plain disclosure link, «Vis tallene» by default, that opens full width with no filled background.
 * Native details/summary: works without JavaScript and is hidden in print, where the chart is enough.
 */
export function NumbersFold({ summary = "Vis tallene", children }: { summary?: string; children: ReactNode }) {
  return (
    <details className="group min-w-0 print:hidden">
      <summary className="cursor-pointer list-none text-(--ax-text-accent) underline underline-offset-4 hover:no-underline [&::-webkit-details-marker]:hidden">
        <span
          aria-hidden
          className="inline-block transition-transform group-open:rotate-90"
          style={{ marginRight: "var(--ax-space-4)" }}
        >
          ›
        </span>
        {summary}
      </summary>
      <div className="min-w-0 overflow-x-auto" style={{ marginTop: "var(--ax-space-8)" }}>
        {children}
      </div>
    </details>
  );
}

/** First column left, the rest right-aligned numbers; the first row is the header. */
export function PlainTable({ rows: [head, ...rows] }: { rows: string[][] }) {
  return (
    <Table size="small">
      <TableHeader>
        <TableRow>
          {head.map((h, i) => (
            <TableHeaderCell key={h} scope="col" align={i ? "right" : "left"}>
              {h}
            </TableHeaderCell>
          ))}
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((cells) => (
          <TableRow key={cells[0]}>
            {cells.map((c, i) =>
              i ? (
                <TableDataCell key={i} align="right">
                  {c}
                </TableDataCell>
              ) : (
                <TableHeaderCell key={i} scope="row" className="whitespace-nowrap">
                  {c}
                </TableHeaderCell>
              )
            )}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
