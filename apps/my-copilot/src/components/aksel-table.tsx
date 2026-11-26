"use client";

import { Table } from "@navikt/ds-react";
import type { ComponentProps } from "react";

// Table er et klientkomponent i @navikt/ds-react. Et serverkomponent som gjør
// `import { Table }` får bare en klientreferanse, og statiske felter på den —
// `Table.Header`, `Table.Body`, … — er `undefined`. React kaster da #130
// («Element type is invalid … got: undefined») midt i render, og siden havner i
// error.tsx. Her, innenfor "use client", er Table den ekte funksjonen, så
// delkomponentene kan plukkes ut og eksporteres som egne klientreferanser.
export { Table };
export const TableHeader = Table.Header;
export const TableBody = Table.Body;
export const TableRow = Table.Row;

/** A short header (at most four words, 24 characters) stays on one line from md up; a longer one may wrap. */
const isShortHeader = (c: unknown) => typeof c === "string" && c.length <= 24 && c.trim().split(/\s+/).length <= 4;

export function TableHeaderCell({ className, ...props }: ComponentProps<typeof Table.HeaderCell>) {
  const nowrap = isShortHeader(props.children) ? "md:whitespace-nowrap" : "";
  return <Table.HeaderCell {...props} className={[className, nowrap].filter(Boolean).join(" ") || undefined} />;
}
export const TableDataCell = Table.DataCell;
