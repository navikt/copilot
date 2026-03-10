"use client";

import { Dialog, DialogBackdrop, DialogPanel, DialogTitle } from "@headlessui/react";
import { XMarkIcon, ExternalLinkIcon } from "@navikt/aksel-icons";
import { Box, BodyShort, Heading, Tag, HStack, VStack, CopyButton } from "@navikt/ds-react";
import type { AnyCustomization, CustomizationType } from "@/lib/customization-types";
import { DOMAIN_CONFIGS, TYPE_LABELS } from "@/lib/customization-types";

interface DetailDrawerProps {
  item: AnyCustomization | null;
  open: boolean;
  onClose: () => void;
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

function getManualInstallCommand(item: AnyCustomization): string {
  if (item.type === "mcp") return "";
  const dir = INSTALL_DIRS[item.type];
  return `mkdir -p ${dir} && curl -sO --output-dir ${dir} ${item.rawGitHubUrl}`;
}

function getMcpCliCommand(item: AnyCustomization): string {
  if (item.type !== "mcp" || item.remotes.length === 0) return "";
  return `gh copilot mcp add --type http ${item.name} ${item.remotes[0].url}`;
}

function getVsCodeAddMcpCommand(item: AnyCustomization): string {
  if (item.type !== "mcp") return "";

  if (item.packages && item.packages.length > 0) {
    const pkg = item.packages[0];
    const runtime = pkg.registryType === "npm" ? "npx" : pkg.registryType === "pypi" ? "uvx" : null;
    if (!runtime) return "";

    const args: string[] = pkg.registryType === "npm" ? ["-y", pkg.identifier] : [pkg.identifier];
    if (pkg.packageArguments) {
      for (const arg of pkg.packageArguments) {
        if (arg.name) args.push(arg.name);
        if (arg.value) args.push(arg.value);
      }
    }

    const serverName = item.name.split("/").pop() ?? item.name;
    const config: Record<string, unknown> = { name: serverName, command: runtime, args };

    if (pkg.environmentVariables) {
      const env: Record<string, string> = {};
      for (const v of pkg.environmentVariables) {
        env[v.name] = v.isSecret ? `\${input:${v.name}}` : (v.description ?? "");
      }
      if (Object.keys(env).length > 0) config.env = env;
    }

    return `code --add-mcp '${JSON.stringify(config)}'`;
  }

  if (item.remotes.length > 0) {
    const serverName = item.name.split("/").pop() ?? item.name;
    const config = { name: serverName, type: "http", url: item.remotes[0].url };
    return `code --add-mcp '${JSON.stringify(config)}'`;
  }

  return "";
}

const EDITOR_SUPPORT: Record<CustomizationType, string> = {
  instruction: "VS Code · JetBrains · CLI · GitHub.com",
  agent: "VS Code · JetBrains (coding agent) · GitHub.com",
  prompt: "VS Code · JetBrains",
  skill: "VS Code",
  mcp: "VS Code · JetBrains · CLI",
};

function McpDetails({ item }: { item: AnyCustomization }) {
  if (item.type !== "mcp") return null;

  return (
    <VStack gap="space-16">
      {(item.websiteUrl || item.repository) && (
        <VStack gap="space-8">
          <Heading size="xsmall" level="4">
            Lenker
          </Heading>
          <VStack gap="space-4">
            {item.websiteUrl && (
              <a
                href={item.websiteUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1 text-sm text-blue-600 hover:underline"
              >
                <ExternalLinkIcon fontSize="1rem" aria-hidden />
                Dokumentasjon
              </a>
            )}
            {item.repository && (
              <a
                href={item.repository.url}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1 text-sm text-blue-600 hover:underline"
              >
                <ExternalLinkIcon fontSize="1rem" aria-hidden />
                Kildekode ({item.repository.source})
              </a>
            )}
          </VStack>
        </VStack>
      )}

      {item.tools && item.tools.length > 0 && (
        <VStack gap="space-8">
          <Heading size="xsmall" level="4">
            Verktøy ({item.tools.length})
          </Heading>
          <HStack gap="space-4" wrap>
            {item.tools.map((tool) => (
              <Tag key={tool} size="xsmall" variant="neutral">
                {tool}
              </Tag>
            ))}
          </HStack>
        </VStack>
      )}

      {item.tags && item.tags.length > 0 && (
        <VStack gap="space-8">
          <Heading size="xsmall" level="4">
            Kategorier
          </Heading>
          <HStack gap="space-4" wrap>
            {item.tags.map((tag) => (
              <Tag key={tag} size="xsmall" variant="info">
                {tag}
              </Tag>
            ))}
          </HStack>
        </VStack>
      )}

      {item.remotes.length > 0 && (
        <VStack gap="space-8">
          <Heading size="xsmall" level="4">
            Transport
          </Heading>
          <HStack gap="space-4" wrap>
            {item.remotes.map((remote) => (
              <Tag key={remote.type} size="xsmall" variant="neutral">
                {transportLabel(remote.type)}
              </Tag>
            ))}
          </HStack>
        </VStack>
      )}

      {item.packages && item.packages.length > 0 && (
        <VStack gap="space-8">
          <Heading size="xsmall" level="4">
            Pakker
          </Heading>
          {item.packages.map((pkg) => (
            <Box key={pkg.identifier} background="neutral-soft" borderRadius="8" padding="space-12">
              <VStack gap="space-4">
                <HStack gap="space-4" align="center">
                  <Tag size="xsmall" variant="neutral">
                    {pkg.registryType}
                  </Tag>
                  <BodyShort size="small" weight="semibold">
                    {pkg.identifier}
                  </BodyShort>
                </HStack>
                {pkg.runtimeHint && (
                  <BodyShort size="small" className="text-gray-500">
                    Runtime: {pkg.runtimeHint}
                  </BodyShort>
                )}
                <BodyShort size="small" className="text-gray-500">
                  Transport: {transportLabel(pkg.transport.type)}
                </BodyShort>
                {pkg.packageArguments && pkg.packageArguments.length > 0 && (
                  <VStack gap="space-4">
                    <BodyShort size="small" weight="semibold">
                      Sikkerhetsargumenter:
                    </BodyShort>
                    {pkg.packageArguments.map((arg) => (
                      <BodyShort key={arg.name ?? arg.value} size="small" className="text-gray-600">
                        <code className="text-xs bg-gray-100 rounded px-1">{arg.name ?? arg.value}</code>
                        {arg.description && ` — ${arg.description}`}
                      </BodyShort>
                    ))}
                  </VStack>
                )}
              </VStack>
            </Box>
          ))}
        </VStack>
      )}

      <VStack gap="space-8">
        <Heading size="xsmall" level="4">
          Installering
        </Heading>
        <BodyShort size="small" className="text-gray-500">
          {EDITOR_SUPPORT.mcp}
        </BodyShort>
        <BodyShort size="small">
          Tilgjengelig fra MCP-registeret i VS Code og JetBrains — søk etter serveren under MCP-innstillinger.
        </BodyShort>
        {getVsCodeAddMcpCommand(item) && (
          <VStack gap="space-4">
            <BodyShort size="small" weight="semibold">
              VS Code CLI
            </BodyShort>
            <div className="relative">
              <pre className="text-xs bg-gray-100 rounded p-2 pr-10 overflow-x-auto whitespace-pre-wrap break-all">
                {getVsCodeAddMcpCommand(item)}
              </pre>
              <div className="absolute top-1 right-1">
                <CopyButton size="xsmall" copyText={getVsCodeAddMcpCommand(item)} />
              </div>
            </div>
          </VStack>
        )}
        {getMcpCliCommand(item) && (
          <VStack gap="space-4">
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
          </VStack>
        )}
      </VStack>
    </VStack>
  );
}

function StaticCustomizationDetails({ item }: { item: AnyCustomization }) {
  if (item.type === "mcp") return null;

  return (
    <VStack gap="space-16">
      {item.type === "agent" && item.tools.length > 0 && (
        <VStack gap="space-8">
          <Heading size="xsmall" level="4">
            Verktøy ({item.tools.length})
          </Heading>
          <HStack gap="space-4" wrap>
            {item.tools.map((tool) => (
              <Tag key={tool} size="xsmall" variant="neutral">
                {tool}
              </Tag>
            ))}
          </HStack>
        </VStack>
      )}

      {item.type === "instruction" && (
        <VStack gap="space-8">
          <Heading size="xsmall" level="4">
            Gjelder for
          </Heading>
          <code className="text-xs bg-gray-100 rounded px-2 py-1 inline-block">{item.applyTo}</code>
        </VStack>
      )}

      {item.type === "prompt" && (
        <VStack gap="space-8">
          <Heading size="xsmall" level="4">
            Aktivering
          </Heading>
          <code className="text-xs bg-gray-100 rounded px-2 py-1 inline-block">{item.invocation}</code>
        </VStack>
      )}

      <VStack gap="space-8">
        <Heading size="xsmall" level="4">
          Installering
        </Heading>
        <BodyShort size="small" className="text-gray-500">
          {EDITOR_SUPPORT[item.type]}
        </BodyShort>

        {item.installUrl && (
          <a
            href={item.installUrl}
            className="inline-flex items-center gap-1 text-sm font-semibold text-blue-600 hover:underline"
          >
            Installer med ett klikk
          </a>
        )}

        <VStack gap="space-4">
          <BodyShort size="small" weight="semibold">
            Manuell installering
          </BodyShort>
          <div className="relative">
            <pre className="text-xs bg-gray-100 rounded p-2 pr-10 overflow-x-auto whitespace-pre-wrap break-all">
              {getManualInstallCommand(item)}
            </pre>
            <div className="absolute top-1 right-1">
              <CopyButton size="xsmall" copyText={getManualInstallCommand(item)} />
            </div>
          </div>
        </VStack>
      </VStack>
    </VStack>
  );
}

export function DetailDrawer({ item, open, onClose }: DetailDrawerProps) {
  if (!item) return null;

  const domainConfig = DOMAIN_CONFIGS[item.domain];

  return (
    <Dialog open={open} onClose={onClose} className="relative z-50">
      <DialogBackdrop
        transition
        className="fixed inset-0 bg-black/30 transition-opacity data-closed:opacity-0 data-enter:duration-300 data-enter:ease-out data-leave:duration-200 data-leave:ease-in"
      />

      <div className="fixed inset-0 overflow-hidden">
        <div className="absolute inset-0 overflow-hidden">
          <div className="pointer-events-none fixed inset-y-0 right-0 flex max-w-full pl-10">
            <DialogPanel
              transition
              className="pointer-events-auto w-screen max-w-md transform transition data-closed:translate-x-full data-enter:duration-300 data-enter:ease-out data-leave:duration-200 data-leave:ease-in"
            >
              <div className="flex h-full flex-col overflow-y-auto bg-white shadow-xl">
                <Box
                  paddingBlock="space-16"
                  paddingInline="space-20"
                  style={{ borderBottom: "1px solid var(--ax-border-neutral)" }}
                >
                  <div className="flex items-start justify-between">
                    <VStack gap="space-8">
                      <DialogTitle as="div">
                        <Heading size="small" level="3">
                          {item.type === "agent" ? `@${item.name}` : item.name}
                        </Heading>
                      </DialogTitle>
                      <HStack gap="space-4">
                        <Tag size="small" variant={item.type === "mcp" ? "success" : "neutral"}>
                          {TYPE_LABELS[item.type]}
                        </Tag>
                        <Tag size="small" variant="info">
                          {domainConfig.label}
                        </Tag>
                      </HStack>
                    </VStack>
                    <button
                      type="button"
                      onClick={onClose}
                      className="text-gray-400 hover:text-gray-600 transition-colors bg-transparent border-none cursor-pointer"
                      aria-label="Lukk panel"
                    >
                      <XMarkIcon className="w-6 h-6" />
                    </button>
                  </div>
                </Box>

                <Box paddingBlock="space-16" paddingInline="space-20" className="flex-1">
                  <VStack gap="space-16">
                    <BodyShort>{item.description}</BodyShort>

                    {item.type === "mcp" ? <McpDetails item={item} /> : <StaticCustomizationDetails item={item} />}
                  </VStack>
                </Box>
              </div>
            </DialogPanel>
          </div>
        </div>
      </div>
    </Dialog>
  );
}
