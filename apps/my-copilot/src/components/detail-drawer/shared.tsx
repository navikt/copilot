"use client";

import React, { useState } from "react";
import { Accordion, Button, HStack, Heading, Tag, VStack } from "@navikt/ds-react";

const TOOLS_PREVIEW_COUNT = 5;

export function ToolList({ tools }: { tools: string[] }) {
  const [expanded, setExpanded] = useState(false);
  const showToggle = tools.length > TOOLS_PREVIEW_COUNT;
  const visible = expanded ? tools : tools.slice(0, TOOLS_PREVIEW_COUNT);

  return (
    <VStack gap="space-8">
      <Heading size="xsmall" level="4">
        Verktøy ({tools.length})
      </Heading>
      <HStack gap="space-4" wrap>
        {visible.map((tool) => (
          <Tag key={tool} size="xsmall" variant="neutral">
            {tool}
          </Tag>
        ))}
      </HStack>
      {showToggle && (
        <Button variant="tertiary" size="xsmall" onClick={() => setExpanded(!expanded)}>
          {expanded ? "Vis færre" : `Vis alle ${tools.length} verktøy`}
        </Button>
      )}
    </VStack>
  );
}

export function ExclusiveAccordion({ children }: { children: React.ReactNode }) {
  const [openItem, setOpenItem] = useState<string | null>(null);

  const items = React.Children.toArray(children).filter(Boolean);

  return (
    <Accordion size="small" headingSize="xsmall">
      {items.map((child, i) => {
        if (!React.isValidElement(child)) return child;
        const key = (child.key as string) ?? String(i);
        return React.cloneElement(
          child as React.ReactElement<{ open: boolean; onOpenChange: (open: boolean) => void }>,
          {
            open: openItem === key,
            onOpenChange: (isOpen: boolean) => setOpenItem(isOpen ? key : null),
          }
        );
      })}
    </Accordion>
  );
}
