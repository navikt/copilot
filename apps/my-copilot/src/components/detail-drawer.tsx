"use client";

import React, { useState } from "react";
import { Dialog, DialogBackdrop, DialogPanel, DialogTitle } from "@headlessui/react";
import { XMarkIcon, ExternalLinkIcon, DownloadIcon } from "@navikt/aksel-icons";
import { Alert, Box, BodyShort, Button, Heading, Tag, HStack, VStack, CopyButton, Accordion } from "@navikt/ds-react";
import type { EnrichedCustomization } from "@/lib/enrich-customizations";
import { DOMAIN_CONFIGS, TYPE_LABELS } from "@/lib/customization-types";
import { normalizeExample } from "@/lib/manifest-types";

const TOOLS_PREVIEW_COUNT = 5;

function ToolList({ tools }: { tools: string[] }) {
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
import {
  transportLabel,
  getManualInstallCommand,
  getMcpServerConfig,
  getVsCodeAddMcpCommand,
  getMcpAddFields,
  CLIENT_SUPPORT,
} from "@/lib/install-commands";

interface DetailDrawerProps {
  item: EnrichedCustomization | null;
  open: boolean;
  onClose: () => void;
}

function ExclusiveAccordion({ children }: { children: React.ReactNode }) {
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

function McpDetails({ item }: { item: EnrichedCustomization }) {
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
                {item.remotes.length > 0 && (
                  <Alert variant="warning" size="small">
                    IntelliJ støtter foreløpig ikke OAuth-autentisering for MCP-servere. Noen servere som krever
                    innlogging vil derfor ikke fungere. Copilot-teamet i Nav følger med på dette.
                  </Alert>
                )}
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

function StaticCustomizationDetails({ item }: { item: EnrichedCustomization }) {
  if (item.type === "mcp") return null;

  return (
    <VStack gap="space-16">
      {item.type === "agent" && item.tools.length > 0 && <ToolList tools={item.tools} />}

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
                    Installer med ett klikk
                  </a>
                )}
                <BodyShort size="small">Eller kopier filen manuelt:</BodyShort>
                <div className="relative">
                  <pre className="text-xs bg-gray-100 rounded p-2 pr-10 overflow-x-auto whitespace-pre-wrap break-all">
                    {getManualInstallCommand(item)}
                  </pre>
                  <div className="absolute top-1 right-1">
                    <CopyButton size="xsmall" copyText={getManualInstallCommand(item)} />
                  </div>
                </div>
                <BodyShort size="small" className="text-gray-500">
                  {item.type === "agent" && "Aktiver med @-mention i Copilot Chat."}
                  {item.type === "instruction" && `Lastes automatisk for filer som matcher ${item.applyTo}.`}
                  {item.type === "prompt" && `Kjør med ${item.invocation} i Copilot Chat.`}
                  {item.type === "skill" && "Plukkes opp automatisk av Copilot Chat og agenter."}
                </BodyShort>
              </VStack>
            </Accordion.Content>
          </Accordion.Item>
          {CLIENT_SUPPORT[item.type].includes("intellij") && (
            <Accordion.Item>
              <Accordion.Header>IntelliJ</Accordion.Header>
              <Accordion.Content>
                <VStack gap="space-8">
                  <BodyShort size="small">Kopier filen til prosjektet — samme plassering som for VS Code:</BodyShort>
                  <div className="relative">
                    <pre className="text-xs bg-gray-100 rounded p-2 pr-10 overflow-x-auto whitespace-pre-wrap break-all">
                      {getManualInstallCommand(item)}
                    </pre>
                    <div className="absolute top-1 right-1">
                      <CopyButton size="xsmall" copyText={getManualInstallCommand(item)} />
                    </div>
                  </div>
                  <BodyShort size="small" className="text-gray-500">
                    {item.type === "agent"
                      ? "Bruk @-mention i Copilot Chat eller Coding Agent-modus."
                      : item.type === "instruction"
                        ? `Lastes automatisk for filer som matcher ${item.applyTo}.`
                        : item.type === "prompt"
                          ? `Kjør med ${item.invocation} i Copilot Chat.`
                          : item.type === "skill"
                            ? "Krever Agent Mode (forhåndsvisning). Aktiver via Settings > GitHub Copilot > Chat > Agent."
                            : null}
                  </BodyShort>
                </VStack>
              </Accordion.Content>
            </Accordion.Item>
          )}
          {CLIENT_SUPPORT[item.type].includes("cli") && (
            <Accordion.Item>
              <Accordion.Header>Copilot CLI</Accordion.Header>
              <Accordion.Content>
                <VStack gap="space-8">
                  <BodyShort size="small">Kopier filen til prosjektet:</BodyShort>
                  <div className="relative">
                    <pre className="text-xs bg-gray-100 rounded p-2 pr-10 overflow-x-auto whitespace-pre-wrap break-all">
                      {getManualInstallCommand(item)}
                    </pre>
                    <div className="absolute top-1 right-1">
                      <CopyButton size="xsmall" copyText={getManualInstallCommand(item)} />
                    </div>
                  </div>
                  <BodyShort size="small" className="text-gray-500">
                    {item.type === "agent" && "Velg agent med /agent-kommandoen i en CLI-sesjon."}
                    {item.type === "instruction" && "Lastes automatisk når du kjører copilot fra prosjektmappen."}
                    {item.type === "skill" && "Administrer med /skills list, /skills info og /skills add."}
                  </BodyShort>
                </VStack>
              </Accordion.Content>
            </Accordion.Item>
          )}
        </ExclusiveAccordion>
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

                    {item.usageCount > 0 && (
                      <VStack gap="space-8">
                        <Heading size="xsmall" level="4">
                          Brukt i {item.usageCount} {item.usageCount === 1 ? "repo" : "repoer"}
                        </Heading>
                        <HStack gap="space-4" wrap>
                          {item.usedBy.map((repo, i) => (
                            <span key={repo} className="inline-flex items-center">
                              <a
                                href={`https://github.com/navikt/${repo}`}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="text-sm text-blue-600 hover:underline"
                              >
                                {repo}
                              </a>
                              {i < item.usedBy.length - 1 || item.usageCount > item.usedBy.length ? (
                                <span className="text-gray-400">,</span>
                              ) : null}
                            </span>
                          ))}
                          {item.usageCount > item.usedBy.length && (
                            <span className="text-sm text-gray-500">+{item.usageCount - item.usedBy.length} andre</span>
                          )}
                        </HStack>
                      </VStack>
                    )}

                    {item.type === "mcp" ? <McpDetails item={item} /> : <StaticCustomizationDetails item={item} />}
                  </VStack>
                </Box>

                <Box
                  paddingBlock="space-12"
                  paddingInline="space-20"
                  style={{ borderTop: "1px solid var(--ax-border-neutral)" }}
                >
                  <a
                    href={
                      item.type === "mcp" && item.repository?.url
                        ? item.repository.url
                        : `https://github.com/navikt/copilot/blob/main/${item.filePath}`
                    }
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-700 transition-colors"
                  >
                    <ExternalLinkIcon className="w-4 h-4" />
                    Vis kildekode på GitHub
                  </a>
                </Box>
              </div>
            </DialogPanel>
          </div>
        </div>
      </div>
    </Dialog>
  );
}
