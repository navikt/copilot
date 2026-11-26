"use client";

import { Alert, BodyShort, Box, CopyButton, HStack, Heading, Tabs, Tag, VStack } from "@navikt/ds-react";
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
import { isFleetGatewayMcp, isGatewayUrl, isLocalMcp, riskOf } from "./mcp-meta";
import { EFFECT, LEVELS_HREF, effectOf, type Effect } from "@/lib/mcp-labels";
import { enableCommand } from "@/components/mcp/servers";

// nav-pilot mcp enable turns on registry risk read and write (the API value, not the label); the rest stay off until the user picks them.
const ON_BY_DEFAULT = ["read", "write"];
const onByDefault = (risk: string) => ON_BY_DEFAULT.includes(risk);
const ORDER: Effect[] = ["read", "propose", "files", "acts", "unsandboxed"];

function McpToolList({
  tools,
  toolRisk,
  local,
}: {
  tools: string[];
  toolRisk?: Record<string, McpToolRisk>;
  local: boolean;
}) {
  const effect = (tool: string) => effectOf(riskOf(toolRisk, tool), local);
  const groups = ORDER.map((e) => {
    const inGroup = tools.filter((tool) => effect(tool) === e);
    return {
      effect: e,
      count: inGroup.length,
      on: inGroup.filter((tool) => onByDefault(riskOf(toolRisk, tool))).length,
    };
  }).filter((group) => group.count > 0);
  return (
    <VStack gap="space-8">
      <Heading size="xsmall" level="4">
        Verktøy ({tools.length})
      </Heading>
      <ul className="mcp-tool-list">
        {tools.map((tool) => {
          const e = EFFECT[effect(tool)];
          return (
            <li key={tool}>
              <code className="verktoy-code-inline">{tool}</code>
              <Tag size="xsmall" variant={e.tag}>
                {e.label}
              </Tag>
            </li>
          );
        })}
      </ul>
      <VStack gap="space-4">
        {groups.map((group) => (
          <BodyShort key={group.effect} size="small" className="verktoy-subtle">
            <strong>
              {EFFECT[group.effect].label} ({group.count})
            </strong>
            : {EFFECT[group.effect].text}{" "}
            {group.on === group.count ? "På som standard." : group.on ? "Delvis på som standard." : "Av som standard."}
          </BodyShort>
        ))}
        <BodyShort size="small" className="verktoy-subtle">
          <NextLink href={LEVELS_HREF} className={linkClass}>
            Om verktøynivåene
          </NextLink>
        </BodyShort>
      </VStack>
    </VStack>
  );
}

// Only github.com URLs get /tree/main/<subfolder>; unsafe subfolders fall back to the repo root.
export function repoLink(repo?: { url: string; subfolder?: string }): string | undefined {
  const url = safeHttpsUrl(repo?.url);
  if (!repo || !url) return undefined;
  if (!repo.subfolder || !url.startsWith("https://github.com/")) return url;
  const segs = repo.subfolder.split("/");
  if (segs.some((s) => !s || s === "." || s === ".." || /[?#\\]/.test(s))) return url;
  return `${url.replace(/\/+$/, "")}/tree/main/${segs.map(encodeURIComponent).join("/")}`;
}

// https only, parsed: no javascript:/data:/http:, no protocol-relative, no userinfo, no malformed URLs.
export function safeHttpsUrl(url?: string): string | undefined {
  if (!url) return undefined;
  try {
    const u = new URL(url);
    return u.protocol === "https:" && !u.username && !u.password ? url : undefined;
  } catch {
    return undefined;
  }
}

export function McpDetails({ item }: { item: EnrichedCustomization }) {
  if (item.type !== "mcp") return null;

  const enable = enableCommand(item.serverId);
  const hasLaunchRecipe = item.remotes.length > 0 || (item.packages?.length ?? 0) > 0;
  const tools = item.tools ?? [];
  const repoHref = repoLink(item.repository);
  const websiteHref = safeHttpsUrl(item.websiteUrl);
  const offByDefault = tools.filter((tool) => ["external", "host-exec"].includes(riskOf(item.toolRisk, tool)));
  const hostExec = tools.filter((tool) => riskOf(item.toolRisk, tool) === "host-exec");
  const isGitHub = item.remotes.some(
    (remote) => remote.url.replace(/\/+$/, "") === "https://api.githubcopilot.com/mcp"
  );
  const serverConfig = getMcpServerConfig(item);
  const vsCodeCmd = getVsCodeAddMcpCommand(item);
  const mcpAddFields = getMcpAddFields(item);

  const gateway = isFleetGatewayMcp(item);
  // Prefer the gateway address when a server has several remotes.
  const primaryRemote = item.remotes.find((r) => isGatewayUrl(r.url)) ?? item.remotes[0];
  const remoteUrl = primaryRemote?.url ?? "";
  const openCodeConfig = JSON.stringify({ mcp: { [item.serverId]: { type: "remote", url: remoteUrl } } }, null, 2);
  const manualWarning = (vscode = false) =>
    (offByDefault.length > 0 || isGitHub) && (
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
        {vscode
          ? "Kommandoen for VS Code kan ikke velge verktøy. Slå av de du ikke vil ha i verktøyvalget i klienten."
          : "Manuelt oppsett kan ikke velge enkeltverktøy."}
      </Alert>
    );

  return (
    <VStack gap="space-16">
      {gateway && (
        <BodyShort size="small">
          Serveren går gjennom fleet-gateway.{" "}
          <NextLink href="/mcp/bruke" className={linkClass}>
            Bruke Navs MCP-servere
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
              Koble til
            </Heading>
            {/* Owner-approved exception to the no-tabs rule (#2822): one tab per client. */}
            <Tabs defaultValue="navpilot" size="small" className="mcp-tabs">
              <Tabs.List>
                <Tabs.Tab value="navpilot" label="nav-pilot" />
                <Tabs.Tab value="vscode" label="VS Code" />
                <Tabs.Tab value="intellij" label="IntelliJ" />
                {serverConfig && <Tabs.Tab value="cli" label="Copilot CLI" />}
                {remoteUrl && <Tabs.Tab value="opencode" label="OpenCode" />}
                {remoteUrl && <Tabs.Tab value="other" label="Andre klienter" />}
              </Tabs.List>
              <Tabs.Panel value="navpilot">
                <VStack gap="space-8" className="mcp-tab-panel">
                  <BodyShort size="small">
                    Med nav-pilot ({NAV_PILOT_MCP_TOOLS_MIN_VERSION} eller nyere) kjører du:
                  </BodyShort>
                  <CommandBlock command={enable} />
                  <BodyShort size="small">
                    Kommandoen legger serveren inn i oppsettet for Copilot CLI og OpenCode (de du har installert), spør
                    om adressene serveren trenger i sandkassen, og sier fra om noe mangler.
                  </BodyShort>
                  {offByDefault.length > 0 && (
                    <BodyShort size="small">
                      Den slår bare på verktøy som leser eller endrer filer. Vil du også ha{" "}
                      {offByDefault.length === 1 ? "verktøyet" : "verktøyene"} som er av, kjører du kommandoen i en
                      terminal og velger dem, eller legger til <code className="verktoy-code-inline">--tools</code> med
                      alle verktøyene du vil ha på, skilt med komma:{" "}
                      <code className="verktoy-code-inline">{enable} --tools navn1,navn2</code>.
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
                      GitHub-serveren får et endpoint som bare leser. Velger du et verktøy som skriver til GitHub, kan
                      agenten skrive via MCP, forbi cplts kontroll av <code className="verktoy-code-inline">gh</code>.
                    </BodyShort>
                  )}
                  <BodyShort size="small">
                    Virker ikke serveren? Kjør <code className="verktoy-code-inline">nav-pilot mcp list</code> for å se
                    hva som er galt og hvordan du retter det.
                  </BodyShort>
                </VStack>
              </Tabs.Panel>
              <Tabs.Panel value="vscode">
                <VStack gap="space-8" className="mcp-tab-panel">
                  {manualWarning(true)}
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
              </Tabs.Panel>
              <Tabs.Panel value="intellij">
                <VStack gap="space-8" className="mcp-tab-panel">
                  {manualWarning()}
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
              </Tabs.Panel>
              {serverConfig && (
                <Tabs.Panel value="cli">
                  <VStack gap="space-8" className="mcp-tab-panel">
                    {manualWarning()}
                    {mcpAddFields && (
                      <VStack gap="space-4">
                        <BodyShort size="small">
                          Kjør <code className="verktoy-code-inline">/mcp add</code> og fyll inn:
                        </BodyShort>
                        <Box background="neutral-soft" borderRadius="8" padding="space-12">
                          <VStack gap="space-4">
                            <BodyShort size="small">
                              <strong>Server Name:</strong>{" "}
                              <code className="verktoy-code-inline">{mcpAddFields.name}</code>
                            </BodyShort>
                            <BodyShort size="small">
                              <strong>Server Type:</strong>{" "}
                              <code className="verktoy-code-inline">{mcpAddFields.type}</code>
                            </BodyShort>
                            {mcpAddFields.url && (
                              <BodyShort size="small">
                                <strong>URL:</strong> <code className="verktoy-code-inline">{mcpAddFields.url}</code>
                              </BodyShort>
                            )}
                            {mcpAddFields.command && (
                              <BodyShort size="small">
                                <strong>Command:</strong>{" "}
                                <code className="verktoy-code-inline">{mcpAddFields.command}</code>
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
                </Tabs.Panel>
              )}
              {remoteUrl && (
                <Tabs.Panel value="opencode">
                  <VStack gap="space-8" className="mcp-tab-panel">
                    {manualWarning()}
                    <BodyShort size="small">
                      Legg til serveren i <code className="verktoy-code-inline">opencode.json</code>:
                    </BodyShort>
                    <CommandBlock command={openCodeConfig} />
                    {gateway && (
                      <>
                        <CommandBlock command={`opencode mcp auth ${item.serverId}\nopencode`} />
                        <BodyShort size="small" className="verktoy-subtle">
                          Logg ut med <code className="verktoy-code-inline">opencode mcp logout {item.serverId}</code>.
                        </BodyShort>
                      </>
                    )}
                  </VStack>
                </Tabs.Panel>
              )}
              {remoteUrl && (
                <Tabs.Panel value="other">
                  <VStack gap="space-8" className="mcp-tab-panel">
                    {manualWarning()}
                    <BodyShort size="small">Adresse ({transportLabel(primaryRemote.type)}):</BodyShort>
                    <CommandBlock command={remoteUrl} />
                    {gateway && (
                      <BodyShort size="small" className="verktoy-subtle">
                        Klienten må støtte OAuth-innlogging.
                      </BodyShort>
                    )}
                  </VStack>
                </Tabs.Panel>
              )}
            </Tabs>
            {gateway && (
              <BodyShort size="small">
                Krever naisdevice.{" "}
                <NextLink href="/mcp/bruke#innlogging" className={linkClass}>
                  Slik logger du inn
                </NextLink>
              </BodyShort>
            )}
          </VStack>
        </Box>
      )}

      {(websiteHref || repoHref) && (
        <VStack gap="space-8">
          <Heading size="xsmall" level="4">
            Lenker
          </Heading>
          <VStack gap="space-4">
            {websiteHref && (
              <a
                href={websiteHref}
                target="_blank"
                rel="noopener noreferrer"
                className={`inline-flex items-center gap-1 text-sm ${linkClass}`}
              >
                <ExternalLinkIcon fontSize="1rem" aria-hidden />
                Dokumentasjon
              </a>
            )}
            {repoHref && (
              <a
                href={repoHref}
                target="_blank"
                rel="noopener noreferrer"
                className={`inline-flex items-center gap-1 text-sm ${linkClass}`}
              >
                <ExternalLinkIcon fontSize="1rem" aria-hidden />
                Kildekode ({item.repository?.source})
              </a>
            )}
          </VStack>
        </VStack>
      )}

      {tools.length > 0 && <McpToolList tools={tools} toolRisk={item.toolRisk} local={isLocalMcp(item)} />}

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
    </VStack>
  );
}
