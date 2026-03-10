"use client";

import { useState } from "react";
import { Box, BodyShort, Heading, Tag, HStack, VStack, CopyButton } from "@navikt/ds-react";
import { DownloadIcon, ChevronDownIcon, ChevronUpIcon, WrenchIcon } from "@navikt/aksel-icons";
import type { AnyCustomization, CustomizationType } from "@/lib/customization-types";
import { DOMAIN_CONFIGS, TYPE_LABELS } from "@/lib/customization-types";

interface CustomizationCardProps {
  item: AnyCustomization;
  onClick?: () => void;
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

const INSTALL_DIRS: Record<Exclude<CustomizationType, "mcp">, string> = {
  agent: ".github/agents",
  instruction: ".github/instructions",
  prompt: ".github/prompts",
  skill: ".github/skills",
};

const EDITOR_SUPPORT: Record<CustomizationType, string> = {
  instruction: "VS Code · JetBrains · CLI · GitHub.com",
  agent: "VS Code · JetBrains (coding agent) · GitHub.com",
  prompt: "VS Code · JetBrains",
  skill: "VS Code",
  mcp: "VS Code · JetBrains · CLI",
};

function getManualInstallCommand(item: AnyCustomization): string {
  if (item.type === "mcp") return "";
  const dir = INSTALL_DIRS[item.type];
  return `mkdir -p ${dir} && curl -sO --output-dir ${dir} ${item.rawGitHubUrl}`;
}

function getMcpCliCommand(item: AnyCustomization): string {
  if (item.type !== "mcp" || item.remotes.length === 0) return "";
  return `gh copilot mcp add --type http ${item.name} ${item.remotes[0].url}`;
}

export function CustomizationCard({ item, onClick }: CustomizationCardProps) {
  const domainConfig = DOMAIN_CONFIGS[item.domain];
  const [showEditors, setShowEditors] = useState(false);
  const [showTools, setShowTools] = useState(false);

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
      className="border-l-4"
      onClick={onClick}
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

        <HStack gap="space-8" align="center">
          {item.installUrl && (
            <a
              href={item.installUrl}
              className="inline-flex items-center gap-1 text-sm font-semibold text-blue-600 hover:underline"
            >
              <DownloadIcon fontSize="1rem" aria-hidden />
              Installer
            </a>
          )}

          <button
            type="button"
            onClick={() => setShowEditors(!showEditors)}
            className="inline-flex items-center gap-1 text-blue-600 hover:underline cursor-pointer bg-transparent border-none p-0"
            style={{ fontSize: "0.875rem", lineHeight: "1.25rem" }}
          >
            {showEditors ? (
              <ChevronUpIcon fontSize="0.875rem" aria-hidden />
            ) : (
              <ChevronDownIcon fontSize="0.875rem" aria-hidden />
            )}
            {item.type === "mcp" ? "Installering" : "Andre editorer"}
          </button>

          {item.type === "agent" && item.tools.length > 0 && (
            <button
              type="button"
              onClick={() => setShowTools(!showTools)}
              className="inline-flex items-center gap-1 text-blue-600 hover:underline cursor-pointer bg-transparent border-none p-0"
              style={{ fontSize: "0.875rem", lineHeight: "1.25rem" }}
            >
              {showTools ? (
                <ChevronUpIcon fontSize="0.875rem" aria-hidden />
              ) : (
                <WrenchIcon fontSize="0.875rem" aria-hidden />
              )}
              {item.tools.length} verktøy
            </button>
          )}
        </HStack>

        {showTools && item.type === "agent" && item.tools.length > 0 && (
          <HStack gap="space-4" wrap>
            {item.tools.map((tool) => (
              <Tag key={tool} size="xsmall" variant="neutral">
                {tool}
              </Tag>
            ))}
          </HStack>
        )}

        {showEditors && (
          <VStack gap="space-8">
            <BodyShort size="small" className="text-gray-500">
              {EDITOR_SUPPORT[item.type]}
            </BodyShort>

            {item.type !== "mcp" && (
              <div className="relative">
                <pre className="text-xs bg-gray-100 rounded p-2 pr-10 overflow-x-auto whitespace-pre-wrap break-all">
                  {getManualInstallCommand(item)}
                </pre>
                <div className="absolute top-1 right-1">
                  <CopyButton size="xsmall" copyText={getManualInstallCommand(item)} />
                </div>
              </div>
            )}

            {item.type === "mcp" && (
              <VStack gap="space-8">
                <BodyShort size="small">
                  Tilgjengelig fra MCP-registeret i VS Code og JetBrains — søk etter serveren under MCP-innstillinger.
                </BodyShort>

                {item.remotes.length > 0 && (
                  <>
                    <BodyShort size="small" weight="semibold">
                      Copilot CLI
                    </BodyShort>
                    <div className="relative">
                      <pre className="text-xs bg-gray-100 rounded p-2 pr-10 overflow-x-auto whitespace-pre-wrap break-all">
                        {getMcpCliCommand(item)}
                      </pre>
                      <div className="absolute top-1 right-1">
                        <CopyButton size="xsmall" copyText={getMcpCliCommand(item)} />
                      </div>
                    </div>
                  </>
                )}
              </VStack>
            )}
          </VStack>
        )}
      </VStack>
    </Box>
  );
}
