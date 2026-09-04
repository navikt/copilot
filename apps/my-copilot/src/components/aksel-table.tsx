"use client";

import { Table } from "@navikt/ds-react";

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
export const TableHeaderCell = Table.HeaderCell;
export const TableDataCell = Table.DataCell;
