"use client";

import { Accordion, Alert, BodyShort, Box, CopyButton, HStack, Heading, Tag, VStack } from "@navikt/ds-react";
import { DownloadIcon, ExternalLinkIcon } from "@navikt/aksel-icons";
import type { EnrichedCustomization } from "@/lib/enrich-customizations";
import type { McpToolRisk } from "@/lib/customization-types";
import { normalizeExample } from "@/lib/manifest-types";
import {
  transportLabel,
  getMcpServerConfig,
  getVsCodeAddMcpCommand,
  getMcpAddFields,
  NAV_PILOT_MCP_TOOLS_MIN_VERSION,
} from "@/lib/install-commands";
import { ExclusiveAccordion } from "./shared";

const RISK_GROUPS: {
  risk: McpToolRisk;
  heading: string;
  text: string;
  tag: "neutral" | "info" | "warning" | "error";
}[] = [
  { risk: "read", heading: "Leser", text: "Henter informasjon. På som standard.", tag: "neutral" },
  {
    risk: "write",
    heading: "Endrer prosjektet",
    text: "Endrer filer eller data i prosjektet. På som standard.",
    tag: "info",
  },
  {
    risk: "external",
    heading: "Gjør noe i et annet system",
    text: "Gir et synlig resultat utenfor maskinen din, for eksempel en ny issue eller en fil i Figma. Av som standard.",
    tag: "warning",
  },
  {
    risk: "host-exec",
    heading: "Kjører utenfor sandkassen",
    text: "Kjører på maskinen din med dine rettigheter, utenfor cplt-sandkassen. Agenten kan da gjøre alt du kan. Av som standard.",
    tag: "error",
  },
];

function riskOf(toolRisk: Record<string, McpToolRisk> | undefined, tool: string): McpToolRisk {
  return toolRisk?.[tool] ?? "read";
}

function McpToolList({ tools, toolRisk }: { tools: string[]; toolRisk?: Record<string, McpToolRisk> }) {
  return (
    <VStack gap="space-8">
      <Heading size="xsmall" level="4">
        Verktøy ({tools.length})
      </Heading>
      {RISK_GROUPS.map((group) => {
        const inGroup = tools.filter((tool) => riskOf(toolRisk, tool) === group.risk);
        if (inGroup.length === 0) return null;
        return (
          <VStack key={group.risk} gap="space-4">
            <BodyShort size="small" weight="semibold">
              {group.heading} ({inGroup.length})
            </BodyShort>
            <BodyShort size="small" className="text-gray-600">
              {group.text}
            </BodyShort>
            <HStack gap="space-4" wrap>
              {inGroup.map((tool) => (
                <Tag key={tool} size="xsmall" variant={group.tag}>
                  {tool}
                </Tag>
              ))}
            </HStack>
          </VStack>
        );
      })}
    </VStack>
  );
}

export function McpDetails({ item }: { item: EnrichedCustomization }) {
  if (item.type !== "mcp") return null;

  const enableCommand = `nav-pilot mcp enable ${item.serverId}`;
  const tools = item.tools ?? [];
  const offByDefault = tools.filter((tool) => ["external", "host-exec"].includes(riskOf(item.toolRisk, tool)));
  const hostExec = tools.filter((tool) => riskOf(item.toolRisk, tool) === "host-exec");
  const isGitHub = item.remotes.some(
    (remote) => remote.url.replace(/\/+$/, "") === "https://api.githubcopilot.com/mcp"
  );

  return (
    <VStack gap="space-16">
      <VStack gap="space-8">
        <Heading size="xsmall" level="4">
          Installering
        </Heading>
        <Box background="info-soft" borderRadius="8" padding="space-12">
          <VStack gap="space-8">
            <BodyShort size="small">Med nav-pilot ({NAV_PILOT_MCP_TOOLS_MIN_VERSION} eller nyere) kjører du:</BodyShort>
            <div className="relative">
              <pre className="text-xs bg-gray-100 rounded p-2 pr-10 overflow-x-auto whitespace-pre-wrap break-all">
                {enableCommand}
              </pre>
              <div className="absolute top-1 right-1">
                <CopyButton size="xsmall" copyText={enableCommand} />
              </div>
            </div>
            <BodyShort size="small">
              Kommandoen legger serveren inn i oppsettet for Copilot CLI og OpenCode (de du har installert), spør om
              adressene serveren trenger i sandkassen, og sier fra om noe mangler.
            </BodyShort>
            {offByDefault.length > 0 && (
              <BodyShort size="small">
                Den slår bare på verktøy som leser eller endrer prosjektet. Vil du også ha{" "}
                {offByDefault.length === 1 ? "verktøyet" : "verktøyene"} som er av, kjører du kommandoen i en terminal
                og velger dem, eller legger til <code className="text-xs bg-gray-100 rounded px-1">--tools</code> med
                navnene.
                {hostExec.length > 0 && (
                  <>
                    {" "}
                    Verktøy som kjører utenfor sandkassen, krever i tillegg at du svarer ja, eller{" "}
                    <code className="text-xs bg-gray-100 rounded px-1">--allow-host-exec</code>.
                  </>
                )}
              </BodyShort>
            )}
            {isGitHub && (
              <BodyShort size="small">
                GitHub-serveren får et endepunkt som bare leser. Velger du et verktøy som skriver til GitHub, kan
                agenten skrive via MCP, forbi cplts kontroll av{" "}
                <code className="text-xs bg-gray-100 rounded px-1">gh</code>.
              </BodyShort>
            )}
            <BodyShort size="small">
              Virker ikke serveren? Kjør <code className="text-xs bg-gray-100 rounded px-1">nav-pilot mcp list</code>{" "}
              for å se hva som er galt og hvordan du retter det.
            </BodyShort>
          </VStack>
        </Box>
      </VStack>

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

      {tools.length > 0 && <McpToolList tools={tools} toolRisk={item.toolRisk} />}

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

      {item.setupInstructions && item.setupInstructions.length > 0 && (
        <VStack gap="space-8">
          <Heading size="xsmall" level="4">
            Oppsett
          </Heading>
          {item.setupInstructions.map((instruction, instructionIndex) => (
            <Box
              key={`${instruction.title}-${instructionIndex}`}
              background="neutral-soft"
              borderRadius="8"
              padding="space-12"
            >
              <VStack gap="space-8">
                <VStack gap="space-4">
                  <BodyShort size="small" weight="semibold">
                    {instruction.title}
                  </BodyShort>
                  <BodyShort size="small">{instruction.description}</BodyShort>
                </VStack>
                {instruction.commands.map((command, commandIndex) => (
                  <Box key={`${command}-${commandIndex}`} background="default" borderRadius="4" padding="space-8">
                    <VStack gap="space-4">
                      <BodyShort size="small">
                        <code className="text-xs break-all">{command}</code>
                      </BodyShort>
                      <CopyButton size="xsmall" copyText={command} />
                    </VStack>
                  </Box>
                ))}
              </VStack>
            </Box>
          ))}
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
                      Argumenter:
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
          Manuelt oppsett
        </Heading>
        <BodyShort size="small" className="text-gray-500">
          For VS Code og IntelliJ, eller hvis du ikke bruker nav-pilot.
        </BodyShort>
        {(offByDefault.length > 0 || isGitHub) && (
          <Alert variant="warning" size="small">
            Manuelt oppsett slår på alle verktøyene til serveren
            {offByDefault.length > 0 && <>, også {offByDefault.join(", ")}</>}.
            {hostExec.length > 0 && (
              <>
                {" "}
                {hostExec.length === offByDefault.length ? "De" : hostExec.join(", ")} kjører på maskinen din, utenfor
                sandkassen.
              </>
            )}{" "}
            Kommandoen for VS Code kan ikke velge verktøy. Slå av de du ikke vil ha i verktøyvalget i klienten.
          </Alert>
        )}
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
