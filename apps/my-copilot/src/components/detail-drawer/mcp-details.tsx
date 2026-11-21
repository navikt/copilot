"use client";

import { useState } from "react";
import { Alert, BodyShort, Box, Chips, CopyButton, HStack, Heading, Tag, VStack } from "@navikt/ds-react";
import { DownloadIcon, ExternalLinkIcon } from "@navikt/aksel-icons";
import type { EnrichedCustomization } from "@/lib/enrich-customizations";
import type { McpToolRisk } from "@/lib/customization-types";
import { normalizeExample } from "@/lib/manifest-types";
import NextLink from "next/link";
import { linkClass } from "@/components/nav-pilot/doc-page";
import {
  transportLabel,
  getMcpServerConfig,
  getVsCodeAddMcpCommand,
  getMcpAddFields,
  NAV_PILOT_MCP_TOOLS_MIN_VERSION,
} from "@/lib/install-commands";
import { CommandBlock } from "./shared";

type ManualClient = "vscode" | "intellij" | "cli";

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

const GATEWAY_HOSTS = ["fleet-gateway.intern.nav.no", "fleet-gateway.intern.dev.nav.no"];

// Registry URLs may hold the {{domain_internal}} template, so accept fleet-gateway.<nav domain> there.
function isGatewayUrl(raw: string): boolean {
  try {
    const host = new URL(raw.replace("{{domain_internal}}", "intern.nav.no")).hostname;
    return GATEWAY_HOSTS.includes(host);
  } catch {
    return false;
  }
}

export function isFleetGatewayMcp(item: EnrichedCustomization): boolean {
  if (item.type !== "mcp") return false;
  return item.remotes.some((r) => isGatewayUrl(r.url)) || (item.tags?.includes("fleet-gateway") ?? false);
}

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
            <BodyShort size="small" className="verktoy-subtle">
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
  const [manualClient, setManualClient] = useState<ManualClient>("vscode");

  if (item.type !== "mcp") return null;

  const enableCommand = `nav-pilot mcp enable ${item.serverId}`;
  const hasLaunchRecipe = item.remotes.length > 0 || (item.packages?.length ?? 0) > 0;
  const tools = item.tools ?? [];
  const offByDefault = tools.filter((tool) => ["external", "host-exec"].includes(riskOf(item.toolRisk, tool)));
  const hostExec = tools.filter((tool) => riskOf(item.toolRisk, tool) === "host-exec");
  const isGitHub = item.remotes.some(
    (remote) => remote.url.replace(/\/+$/, "") === "https://api.githubcopilot.com/mcp"
  );
  const serverConfig = getMcpServerConfig(item);
  const vsCodeCmd = getVsCodeAddMcpCommand(item);
  const mcpAddFields = getMcpAddFields(item);

  return (
    <VStack gap="space-16">
      {isFleetGatewayMcp(item) && (
        <BodyShort size="small">
          Serveren går gjennom fleet-gateway.{" "}
          <NextLink href="/nav-pilot/guider/nav-mcp-servere" className={linkClass}>
            Koble til Navs MCP-servere
          </NextLink>
        </BodyShort>
      )}

      {hasLaunchRecipe && (
        <Box
          paddingBlock="space-16"
          style={{
            borderTop: "1px solid var(--ax-border-neutral-subtle)",
            borderBottom: "1px solid var(--ax-border-neutral-subtle)",
          }}
        >
          <VStack gap="space-12">
            <Heading size="xsmall" level="4">
              Installering
            </Heading>
            <Box background="info-soft" borderRadius="8" padding="space-12">
              <VStack gap="space-8">
                <BodyShort size="small">
                  Med nav-pilot ({NAV_PILOT_MCP_TOOLS_MIN_VERSION} eller nyere) kjører du:
                </BodyShort>
                <CommandBlock command={enableCommand} />
                <BodyShort size="small">
                  Kommandoen legger serveren inn i oppsettet for Copilot CLI og OpenCode (de du har installert), spør om
                  adressene serveren trenger i sandkassen, og sier fra om noe mangler.
                </BodyShort>
                {offByDefault.length > 0 && (
                  <BodyShort size="small">
                    Den slår bare på verktøy som leser eller endrer prosjektet. Vil du også ha{" "}
                    {offByDefault.length === 1 ? "verktøyet" : "verktøyene"} som er av, kjører du kommandoen i en
                    terminal og velger dem, eller legger til <code className="verktoy-code-inline">--tools</code> med
                    alle verktøyene du vil ha på, skilt med komma:{" "}
                    <code className="verktoy-code-inline">{enableCommand} --tools navn1,navn2</code>.
                    {hostExec.length > 0 && (
                      <>
                        {" "}
                        Verktøy som kjører utenfor sandkassen, krever i tillegg at du svarer ja, eller{" "}
                        <code className="verktoy-code-inline">--allow-host-exec</code>.
                      </>
                    )}
                  </BodyShort>
                )}
                {isGitHub && (
                  <BodyShort size="small">
                    GitHub-serveren får et endepunkt som bare leser. Velger du et verktøy som skriver til GitHub, kan
                    agenten skrive via MCP, forbi cplts kontroll av <code className="verktoy-code-inline">gh</code>.
                  </BodyShort>
                )}
                <BodyShort size="small">
                  Virker ikke serveren? Kjør <code className="verktoy-code-inline">nav-pilot mcp list</code> for å se
                  hva som er galt og hvordan du retter det.
                </BodyShort>
              </VStack>
            </Box>
          </VStack>
        </Box>
      )}

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
                className={`inline-flex items-center gap-1 text-sm ${linkClass}`}
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
                className={`inline-flex items-center gap-1 text-sm ${linkClass}`}
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
            {[...new Set(item.remotes.map((remote) => remote.type))].map((type) => (
              <Tag key={type} size="xsmall" variant="neutral">
                {transportLabel(type)}
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
                    <HStack gap="space-8" align="start" justify="space-between" wrap={false}>
                      <code className="text-xs break-all flex-1">{example.prompt}</code>
                      <CopyButton size="xsmall" copyText={example.prompt} />
                    </HStack>
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
                  <CommandBlock key={`${command}-${commandIndex}`} command={command} />
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
                <HStack gap="space-4" align="center" wrap>
                  <Tag size="xsmall" variant="neutral">
                    {pkg.registryType}
                  </Tag>
                  <BodyShort size="small" weight="semibold" className="break-all min-w-0">
                    {pkg.identifier}
                  </BodyShort>
                </HStack>
                {pkg.runtimeHint && (
                  <BodyShort size="small" className="verktoy-subtle">
                    Runtime: {pkg.runtimeHint}
                  </BodyShort>
                )}
                <BodyShort size="small" className="verktoy-subtle">
                  Transport: {transportLabel(pkg.transport.type)}
                </BodyShort>
                {pkg.packageArguments && pkg.packageArguments.length > 0 && (
                  <VStack gap="space-4">
                    <BodyShort size="small" weight="semibold">
                      Argumenter:
                    </BodyShort>
                    {pkg.packageArguments.map((arg) => (
                      <BodyShort key={arg.name ?? arg.value} size="small" className="verktoy-subtle">
                        <code className="verktoy-code-inline">{arg.name ?? arg.value}</code>
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

      {hasLaunchRecipe && (
        <VStack gap="space-12">
          <VStack gap="space-4">
            <Heading size="xsmall" level="4">
              Manuelt oppsett
            </Heading>
            <BodyShort size="small" className="verktoy-subtle">
              For VS Code og IntelliJ, eller hvis du ikke bruker nav-pilot.
            </BodyShort>
          </VStack>

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

          <Chips size="small">
            <Chips.Toggle
              checkmark={false}
              selected={manualClient === "vscode"}
              onClick={() => setManualClient("vscode")}
            >
              VS Code
            </Chips.Toggle>
            <Chips.Toggle
              checkmark={false}
              selected={manualClient === "intellij"}
              onClick={() => setManualClient("intellij")}
            >
              IntelliJ
            </Chips.Toggle>
            {serverConfig && (
              <Chips.Toggle checkmark={false} selected={manualClient === "cli"} onClick={() => setManualClient("cli")}>
                Copilot CLI
              </Chips.Toggle>
            )}
          </Chips>

          {manualClient === "vscode" && (
            <VStack gap="space-8">
              {item.installUrl && (
                <a
                  href={item.installUrl}
                  className={`inline-flex items-center gap-1 text-sm font-semibold ${linkClass}`}
                >
                  <DownloadIcon fontSize="1rem" aria-hidden />
                  Installer fra MCP-registeret
                </a>
              )}
              {vsCodeCmd && (
                <>
                  <BodyShort size="small" className="verktoy-subtle">
                    Alternativt kan du bruke kommandoen:
                  </BodyShort>
                  <CommandBlock command={vsCodeCmd} />
                </>
              )}
              {serverConfig && (
                <>
                  <BodyShort size="small" className="verktoy-subtle">
                    Eller legg til i <code className="verktoy-code-inline">.vscode/mcp.json</code> under{" "}
                    <code className="verktoy-code-inline">&quot;servers&quot;</code>:
                  </BodyShort>
                  <CommandBlock command={serverConfig} />
                </>
              )}
            </VStack>
          )}

          {manualClient === "intellij" && (
            <VStack gap="space-8">
              <BodyShort size="small">
                Åpne Copilot Chat i IntelliJ og klikk på <strong>MCP-register-ikonet</strong> for å søke etter og
                installere serveren direkte fra registeret.
              </BodyShort>
              {serverConfig && (
                <>
                  <BodyShort size="small" className="verktoy-subtle">
                    Alternativt kan du legge til manuelt i{" "}
                    <code className="verktoy-code-inline">~/.config/github-copilot/intellij/mcp.json</code> under{" "}
                    <code className="verktoy-code-inline">&quot;servers&quot;</code>:
                  </BodyShort>
                  <CommandBlock command={serverConfig} />
                </>
              )}
            </VStack>
          )}

          {manualClient === "cli" && serverConfig && (
            <VStack gap="space-8">
              {mcpAddFields && (
                <VStack gap="space-4">
                  <BodyShort size="small">
                    Kjør <code className="verktoy-code-inline">/mcp add</code> og fyll inn:
                  </BodyShort>
                  <Box background="neutral-soft" borderRadius="8" padding="space-12">
                    <VStack gap="space-4">
                      <BodyShort size="small">
                        <strong>Server Name:</strong> <code className="verktoy-code-inline">{mcpAddFields.name}</code>
                      </BodyShort>
                      <BodyShort size="small">
                        <strong>Server Type:</strong> <code className="verktoy-code-inline">{mcpAddFields.type}</code>
                      </BodyShort>
                      {mcpAddFields.url && (
                        <BodyShort size="small">
                          <strong>URL:</strong> <code className="verktoy-code-inline">{mcpAddFields.url}</code>
                        </BodyShort>
                      )}
                      {mcpAddFields.command && (
                        <BodyShort size="small">
                          <strong>Command:</strong> <code className="verktoy-code-inline">{mcpAddFields.command}</code>
                        </BodyShort>
                      )}
                      {mcpAddFields.env && (
                        <BodyShort size="small">
                          <strong>Environment Variables:</strong>{" "}
                          <code className="verktoy-code-inline">{mcpAddFields.env}</code>
                        </BodyShort>
                      )}
                    </VStack>
                  </Box>
                </VStack>
              )}
              <BodyShort size="small" className="verktoy-subtle">
                Eller legg til i <code className="verktoy-code-inline">~/.copilot/mcp-config.json</code> under{" "}
                <code className="verktoy-code-inline">&quot;mcpServers&quot;</code>:
              </BodyShort>
              <CommandBlock command={serverConfig} />
            </VStack>
          )}
        </VStack>
      )}
    </VStack>
  );
}
