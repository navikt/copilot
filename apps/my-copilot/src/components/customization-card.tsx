"use client";

import { Box, BodyShort, Heading, Tag, HStack, VStack } from "@navikt/ds-react";
import { DownloadIcon, ChevronRightIcon, WrenchIcon, ComponentIcon } from "@navikt/aksel-icons";
import { SiGnometerminal, SiIntellijidea, SiGithub } from "@icons-pack/react-simple-icons";
import { DOMAIN_CONFIGS, TYPE_LABELS } from "@/lib/customization-types";
import type { EnrichedCustomization } from "@/lib/enrich-customizations";
import { transportLabel, getToolCount, CLIENT_SUPPORT, CLIENT_LABELS } from "@/lib/install-commands";

interface CustomizationCardProps {
  item: EnrichedCustomization;
  onClick?: () => void;
}

function VSCodeIcon({ size = 16 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" aria-hidden>
      <path d="M23.15 2.587 18.21.21a1.516 1.516 0 0 0-1.732.352L7.041 9.36 2.93 6.258a1.01 1.01 0 0 0-1.291.034l-1.36 1.238a1.012 1.012 0 0 0-.001 1.499L4.613 12 .278 14.97a1.01 1.01 0 0 0 .001 1.499l1.36 1.238a1.01 1.01 0 0 0 1.291.035l4.112-3.102 9.437 8.799c.49.488 1.12.657 1.732.352l4.94-2.377c.536-.258.88-.81.88-1.425V4.012a1.525 1.525 0 0 0-.88-1.425zM17.5 17.584 10.061 12 17.5 6.416z" />
    </svg>
  );
}

function ClientIcon({ client, size = 16 }: { client: string; size?: number }) {
  switch (client) {
    case "vscode":
      return <VSCodeIcon size={size} />;
    case "intellij":
      return <SiIntellijidea size={size} aria-hidden />;
    case "cli":
      return <SiGnometerminal size={size} aria-hidden />;
    case "github":
      return <SiGithub size={size} aria-hidden />;
    default:
      return null;
  }
}

export function CustomizationCard({ item, onClick }: CustomizationCardProps) {
  const domainConfig = DOMAIN_CONFIGS[item.domain];

  return (
    <Box
      background="default"
      borderRadius="12"
      borderColor="neutral"
      borderWidth="1"
      padding={{ xs: "space-12", md: "space-16" }}
      style={{
        borderLeftColor: `var(--ax-${domainConfig.color}-400, currentColor)`,
        cursor: onClick ? "pointer" : undefined,
      }}
      className="border-l-4 transition-shadow hover:shadow-md h-full"
      onClick={onClick}
    >
      <VStack gap="space-8" className="h-full">
        <div className="flex items-start justify-between gap-2">
          <Heading size="xsmall" level="3">
            {item.type === "agent" ? `@${item.name}` : item.name}
          </Heading>
          <HStack gap="space-4" className="shrink-0">
            <Tag size="small" variant={item.type === "mcp" ? "success" : "neutral"}>
              {TYPE_LABELS[item.type]}
            </Tag>
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

        <div className="flex items-center justify-between gap-2 mt-auto">
          <HStack gap="space-8" align="center">
            {item.installUrl && (
              <a
                href={item.installUrl}
                className="inline-flex items-center gap-1 text-sm font-semibold text-blue-600 hover:underline"
                onClick={(e) => e.stopPropagation()}
              >
                <DownloadIcon fontSize="1rem" aria-hidden />
                Installer
              </a>
            )}
            {onClick && (
              <span className="inline-flex items-center gap-0.5 text-sm text-gray-500">
                Mer info
                <ChevronRightIcon fontSize="1rem" aria-hidden />
              </span>
            )}
          </HStack>

          <HStack gap="space-8" align="center">
            {item.usageCount > 0 && (
              <span className="inline-flex items-center gap-1 text-gray-400" title={`Brukt i ${item.usageCount} ${item.usageCount === 1 ? "repo" : "repoer"}`}>
                <ComponentIcon fontSize="0.875rem" aria-hidden />
                <span className="text-xs">{item.usageCount}</span>
              </span>
            )}
            {getToolCount(item) > 0 && (
              <span className="inline-flex items-center gap-1 text-gray-400" title={`${getToolCount(item)} verktøy`}>
                <WrenchIcon fontSize="0.875rem" aria-hidden />
                <span className="text-xs">{getToolCount(item)}</span>
              </span>
            )}
            {CLIENT_SUPPORT[item.type].map((client) => (
              <span key={client} title={CLIENT_LABELS[client]} className="text-gray-400">
                <ClientIcon client={client} size={14} />
              </span>
            ))}
          </HStack>
        </div>
      </VStack>
    </Box>
  );
}
