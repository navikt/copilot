"use client";

import { Box, BodyShort, Heading, Tag, ReadMore, HStack, VStack } from "@navikt/ds-react";
import { DownloadIcon } from "@navikt/aksel-icons";
import type { AnyCustomization } from "@/lib/customization-types";
import { DOMAIN_CONFIGS, TYPE_LABELS } from "@/lib/customization-types";

interface CustomizationCardProps {
  item: AnyCustomization;
}

function transportLabel(type: string): string {
  switch (type) {
    case "streamable-http":
      return "Streamable HTTP";
    case "sse":
      return "SSE";
    case "stdio":
      return "stdio";
    default:
      return type;
  }
}

export function CustomizationCard({ item }: CustomizationCardProps) {
  const domainConfig = DOMAIN_CONFIGS[item.domain];

  return (
    <Box
      background="default"
      borderRadius="12"
      borderColor="neutral"
      borderWidth="1"
      padding={{ xs: "space-12", md: "space-16" }}
      style={{ borderLeftColor: `var(--ax-${domainConfig.color}-400, currentColor)` }}
      className="border-l-4"
    >
      <VStack gap="space-8">
        <div className="flex items-start justify-between gap-2">
          <Heading size="xsmall" level="3">
            {item.type === "agent" ? `@${item.name}` : item.name}
          </Heading>
          <HStack gap="space-4" className="shrink-0">
            <Tag size="small" variant={item.type === "mcp" ? "success" : "neutral"}>
              {TYPE_LABELS[item.type]}
            </Tag>
            {item.type === "mcp" && (
              <Tag size="small" variant="neutral">
                v{item.version}
              </Tag>
            )}
            <Tag size="small" variant="info">
              {domainConfig.label}
            </Tag>
          </HStack>
        </div>

        {item.type === "instruction" && (
          <code className="text-xs bg-gray-100 rounded px-2 py-1 inline-block w-fit">{item.applyTo}</code>
        )}

        {item.type === "prompt" && (
          <code className="text-xs bg-gray-100 rounded px-2 py-1 inline-block w-fit">{item.invocation}</code>
        )}

        {item.type === "agent" && item.tools.length > 0 && (
          <ReadMore header={`${item.tools.length} verktøy`} size="small">
            <HStack gap="space-4" wrap>
              {item.tools.map((tool) => (
                <Tag key={tool} size="xsmall" variant="neutral">
                  {tool}
                </Tag>
              ))}
            </HStack>
          </ReadMore>
        )}

        {item.type === "mcp" && item.remotes.length > 0 && (
          <HStack gap="space-4" wrap>
            {item.remotes.map((remote) => (
              <Tag key={remote.type} size="xsmall" variant="neutral">
                {transportLabel(remote.type)}
              </Tag>
            ))}
          </HStack>
        )}

        <BodyShort size="small" className="text-gray-600 line-clamp-3">
          {item.description}
        </BodyShort>

        {item.installUrl && (
          <HStack gap="space-8">
            <a
              href={item.installUrl}
              className="inline-flex items-center gap-1 text-sm font-semibold text-blue-600 hover:underline"
            >
              <DownloadIcon fontSize="1rem" aria-hidden />
              Installer
            </a>
            {item.insidersInstallUrl && (
              <a
                href={item.insidersInstallUrl}
                className="inline-flex items-center gap-1 text-sm text-gray-500 hover:underline"
              >
                Insiders
              </a>
            )}
          </HStack>
        )}
      </VStack>
    </Box>
  );
}
