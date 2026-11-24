"use client";

import { Accordion } from "@navikt/ds-react";

// Same reason as aksel-table.tsx: a server component only gets a client reference to Accordion,
// so Accordion.Item and friends are undefined there. Pick them out here.
export { Accordion };
export const AccordionItem = Accordion.Item;
export const AccordionHeader = Accordion.Header;
export const AccordionContent = Accordion.Content;
