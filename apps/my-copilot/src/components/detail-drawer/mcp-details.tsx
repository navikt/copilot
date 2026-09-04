"use client";

import { Accordion, BodyShort, Box, CopyButton, HStack, Heading, Tag, VStack } from "@navikt/ds-react";
import { DownloadIcon, ExternalLinkIcon } from "@navikt/aksel-icons";
import type { EnrichedCustomization } from "@/lib/enrich-customizations";
import { normalizeExample } from "@/lib/manifest-types";
import { transportLabel, getMcpServerConfig, getVsCodeAddMcpCommand, getMcpAddFields } from "@/lib/install-commands";
import { ToolList, ExclusiveAccordion } from "./shared";

export function McpDetails({ item }: { item: EnrichedCustomization }) {
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

      {item.tools && item.tools.length > 0 && <ToolList tools={item.tools} />}

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

      {item.examples && item.examples.length > 0 && (
        <VStack gap="space-8">
          <Heading size="xsmall" level="4">
            Eksempler
          </Heading>
          <VStack gap="space-8">
            {item.examples.map((raw, index) => {
              const example = normalizeExample(raw);
              return (
                <Box key={`${example.prompt}-${index}`} background="neutral-soft" borderRadius="8" padding="space-12">
                  <VStack gap="space-4">
                    {example.scenario && (
                      <BodyShort size="small" weight="semibold">
                        {example.scenario}
                      </BodyShort>
                    )}
                    <div className="relative">
                      <code className="text-xs block pr-8 break-all">{example.prompt}</code>
                      <div className="absolute top-0 right-0">
                        <CopyButton size="xsmall" copyText={example.prompt} />
                      </div>
                    </div>
                  </VStack>
                </Box>
              );
            })}
          </VStack>
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
        <ExclusiveAccordion>
          <Accordion.Item>
            <Accordion.Header>VS Code</Accordion.Header>
            <Accordion.Content>
              <VStack gap="space-8">
                {item.installUrl && (
                  <a
                    href={item.installUrl}
                    className="inline-flex items-center gap-1 text-sm font-semibold text-blue-600 hover:underline"
                  >
                    <DownloadIcon fontSize="1rem" aria-hidden />
                    Installer fra MCP-registeret
                  </a>
                )}
                <BodyShort size="small" className="text-gray-500">
                  Alternativt kan du bruke kommandoen:
                </BodyShort>
                {getVsCodeAddMcpCommand(item) && (
                  <div className="relative">
                    <pre className="text-xs bg-gray-100 rounded p-2 pr-10 overflow-x-auto whitespace-pre-wrap break-all">
                      {getVsCodeAddMcpCommand(item)}
                    </pre>
                    <div className="absolute top-1 right-1">
                      <CopyButton size="xsmall" copyText={getVsCodeAddMcpCommand(item)} />
                    </div>
                  </div>
                )}
                <BodyShort size="small" className="text-gray-500">
                  Eller legg til i .vscode/mcp.json under &quot;servers&quot;:
                </BodyShort>
                <div className="relative">
                  <pre className="text-xs bg-gray-100 rounded p-2 pr-10 overflow-x-auto whitespace-pre-wrap break-all">
                    {getMcpServerConfig(item)}
                  </pre>
                  <div className="absolute top-1 right-1">
                    <CopyButton size="xsmall" copyText={getMcpServerConfig(item)} />
                  </div>
                </div>
              </VStack>
            </Accordion.Content>
          </Accordion.Item>
          <Accordion.Item>
            <Accordion.Header>IntelliJ</Accordion.Header>
            <Accordion.Content>
              <VStack gap="space-8">
                <BodyShort size="small">
                  Åpne Copilot Chat i IntelliJ og klikk på <strong>MCP-register-ikonet</strong> for å søke etter og
                  installere serveren direkte fra registeret.
                </BodyShort>
                <BodyShort size="small" className="text-gray-500">
                  Alternativt kan du legge til manuelt i{" "}
                  <code className="text-xs bg-gray-100 rounded px-1">~/.config/github-copilot/intellij/mcp.json</code>{" "}
                  under <code className="text-xs bg-gray-100 rounded px-1">&quot;servers&quot;</code>:
                </BodyShort>
                <div className="relative">
                  <pre className="text-xs bg-gray-100 rounded p-2 pr-10 overflow-x-auto whitespace-pre-wrap break-all">
                    {getMcpServerConfig(item)}
                  </pre>
                  <div className="absolute top-1 right-1">
                    <CopyButton size="xsmall" copyText={getMcpServerConfig(item)} />
                  </div>
                </div>
              </VStack>
            </Accordion.Content>
          </Accordion.Item>
          {getMcpServerConfig(item) && (
            <Accordion.Item>
              <Accordion.Header>Copilot CLI</Accordion.Header>
              <Accordion.Content>
                <VStack gap="space-8">
                  {(() => {
                    const fields = getMcpAddFields(item);
                    if (!fields) return null;
                    return (
                      <VStack gap="space-4">
                        <BodyShort size="small">
                          Kjør <code className="text-xs bg-gray-100 rounded px-1">/mcp add</code> og fyll inn:
                        </BodyShort>
                        <Box background="neutral-soft" borderRadius="8" padding="space-8">
                          <VStack gap="space-4">
                            <BodyShort size="small">
                              <strong>Server Name:</strong>{" "}
                              <code className="text-xs bg-gray-100 rounded px-1">{fields.name}</code>
                            </BodyShort>
                            <BodyShort size="small">
                              <strong>Server Type:</strong>{" "}
                              <code className="text-xs bg-gray-100 rounded px-1">{fields.type}</code>
                            </BodyShort>
                            {fields.url && (
                              <BodyShort size="small">
                                <strong>URL:</strong>{" "}
                                <code className="text-xs bg-gray-100 rounded px-1 break-all">{fields.url}</code>
                              </BodyShort>
                            )}
                            {fields.command && (
                              <BodyShort size="small">
                                <strong>Command:</strong>{" "}
                                <code className="text-xs bg-gray-100 rounded px-1 break-all">{fields.command}</code>
                              </BodyShort>
                            )}
                            {fields.env && (
                              <BodyShort size="small">
                                <strong>Environment Variables:</strong>{" "}
                                <code className="text-xs bg-gray-100 rounded px-1 break-all">{fields.env}</code>
                              </BodyShort>
                            )}
                          </VStack>
                        </Box>
                      </VStack>
                    );
                  })()}
                  <BodyShort size="small" className="text-gray-500">
                    Eller legg til i ~/.copilot/mcp-config.json under &quot;mcpServers&quot;:
                  </BodyShort>
                  <div className="relative">
                    <pre className="text-xs bg-gray-100 rounded p-2 pr-10 overflow-x-auto whitespace-pre-wrap break-all">
                      {getMcpServerConfig(item)}
                    </pre>
                    <div className="absolute top-1 right-1">
                      <CopyButton size="xsmall" copyText={getMcpServerConfig(item)} />
                    </div>
                  </div>
                </VStack>
              </Accordion.Content>
            </Accordion.Item>
          )}
        </ExclusiveAccordion>
      </VStack>
    </VStack>
  );
}
