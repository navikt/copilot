"use client";

import { useState } from "react";
import { Button, CopyButton, HStack, Heading, Tag, VStack } from "@navikt/ds-react";

const TOOLS_PREVIEW_COUNT = 8;

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
        <div>
          <Button variant="tertiary" size="small" onClick={() => setExpanded(!expanded)}>
            {expanded ? "Vis færre" : `Vis alle ${tools.length} verktøy`}
          </Button>
        </div>
      )}
    </VStack>
  );
}

// Display only: spread a quoted JSON argument over lines. The copy button keeps the original one-liner.
export function formatCommandForDisplay(command: string): string {
  const match = /^([^\n]*?)'(\{[^\n]*\})'$/.exec(command);
  if (!match) return command;
  try {
    return `${match[1]}'${JSON.stringify(JSON.parse(match[2]), null, 2)}'`;
  } catch {
    return command;
  }
}

export function CommandBlock({ command, copyText }: { command: string; copyText?: string }) {
  return (
    <div className="verktoy-cmd">
      <pre>{formatCommandForDisplay(command)}</pre>
      <CopyButton size="xsmall" copyText={copyText ?? command} />
    </div>
  );
}
