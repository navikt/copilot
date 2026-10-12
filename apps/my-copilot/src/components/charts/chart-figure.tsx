import { useId, type ReactNode } from "react";
import { BodyShort, Heading, ReadMore, VStack } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@navikt/ds-react/Table";
import { chartBoxClass } from "@/lib/chart-utils";

/**
 * Chart first, then a text summary, then the numbers in a «Vis tallene» fold. The title is a question
 * the chart answers («Er vi på vei over budsjettet?»); the section heading above stays a plain name.
 * The summary labels the chart for screen readers; the table is hidden in print, where the chart is enough.
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
  table: ReactNode | string[][];
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
        <figcaption id={id}>
          <BodyShort size="small" textColor="subtle">
            {summary}
          </BodyShort>
        </figcaption>
        <div className="print:hidden">
          <ReadMore header="Vis tallene" size="small">
            <div className="min-w-0 overflow-x-auto">
              {Array.isArray(table) ? <PlainTable rows={table as string[][]} /> : table}
            </div>
          </ReadMore>
        </div>
      </VStack>
    </figure>
  );
}

/** First column left, the rest right-aligned numbers; the first row is the header. */
function PlainTable({ rows: [head, ...rows] }: { rows: string[][] }) {
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
