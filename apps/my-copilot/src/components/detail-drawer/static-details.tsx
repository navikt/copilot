"use client";

import { useMemo, useState } from "react";
import { BodyShort, Box, Chips, CopyButton, HStack, Heading, Tag, VStack } from "@navikt/ds-react";
import { DownloadIcon } from "@navikt/aksel-icons";
import type { EnrichedCustomization } from "@/lib/enrich-customizations";
import { normalizeExample } from "@/lib/manifest-types";
import { getNavPilotAddCommand, getGhSkillInstallCommand, CLIENT_SUPPORT, INSTALL_DIRS } from "@/lib/install-commands";
import type { CustomizationType } from "@/lib/customization-types";
import { linkClass } from "@/components/nav-pilot/doc-page";
import { CommandBlock, ToolList } from "./shared";

type StaticClient = "nav-pilot" | "vscode" | "gh" | "github";

const CLIENT_LABELS: Record<StaticClient, string> = {
  "nav-pilot": "nav-pilot",
  vscode: "VS Code",
  gh: "GitHub CLI",
  github: "GitHub.com",
};

function AgentReferences({
  references,
  allItems,
  onNavigate,
}: {
  references: string[];
  allItems: EnrichedCustomization[];
  onNavigate?: (item: EnrichedCustomization) => void;
}) {
  const agentMap = new Map(allItems.filter((i) => i.type === "agent").map((i) => [i.id, i]));
  const resolved = references.map((ref) => ({ id: ref, item: agentMap.get(ref) }));

  return (
    <VStack gap="space-8">
      <Heading size="xsmall" level="4">
        Refererer til
      </Heading>
      <HStack gap="space-4" wrap>
        {resolved.map(({ id, item }) =>
          item && onNavigate ? (
            <button
              key={id}
              type="button"
              onClick={() => onNavigate(item)}
              className="bg-transparent border-none cursor-pointer"
            >
              <Tag size="xsmall" variant="info" className="cursor-pointer hover:underline">
                @{id}
              </Tag>
            </button>
          ) : (
            <Tag key={id} size="xsmall" variant="neutral">
              @{id}
            </Tag>
          )
        )}
      </HStack>
    </VStack>
  );
}

export function StaticCustomizationDetails({
  item,
  allItems,
  onNavigate,
}: {
  item: EnrichedCustomization;
  allItems: EnrichedCustomization[];
  onNavigate?: (item: EnrichedCustomization) => void;
}) {
  const navPilot = useMemo(() => (item.type === "mcp" ? null : getNavPilotAddCommand(item)), [item]);

  const availableClients = useMemo<StaticClient[]>(() => {
    if (item.type === "mcp") return [];
    const list: StaticClient[] = [];
    if (navPilot) list.push("nav-pilot");
    if (item.installUrl) list.push("vscode");
    if (CLIENT_SUPPORT[item.type].includes("gh")) list.push("gh");
    if (CLIENT_SUPPORT[item.type].includes("github")) list.push("github");
    return list;
  }, [item, navPilot]);

  const [selectedClient, setSelectedClient] = useState<StaticClient>(availableClients[0] ?? "nav-pilot");

  if (item.type === "mcp") return null;

  const activeClient = availableClients.includes(selectedClient) ? selectedClient : availableClients[0];

  return (
    <VStack gap="space-16">
      {availableClients.length > 0 && activeClient && (
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

            <Chips size="small">
              {availableClients.map((client) => (
                <Chips.Toggle
                  key={client}
                  checkmark={false}
                  selected={activeClient === client}
                  onClick={() => setSelectedClient(client)}
                >
                  {CLIENT_LABELS[client]}
                </Chips.Toggle>
              ))}
            </Chips>

            {activeClient === "nav-pilot" && navPilot && (
              <VStack gap="space-12">
                <VStack gap="space-4">
                  <BodyShort size="small" weight="semibold">
                    I repoet (delt med teamet):
                  </BodyShort>
                  <CommandBlock command={navPilot.repo} />
                </VStack>
                <VStack gap="space-4">
                  <BodyShort size="small" weight="semibold">
                    Personlig (alle repoer):
                  </BodyShort>
                  <CommandBlock command={navPilot.user} />
                </VStack>
                <BodyShort size="small" className="verktoy-subtle">
                  Oppdater med <code className="verktoy-code-inline">nav-pilot sync</code>.
                </BodyShort>
              </VStack>
            )}

            {activeClient === "vscode" && item.installUrl && (
              <VStack gap="space-8">
                <a
                  href={item.installUrl}
                  className={`inline-flex items-center gap-1 text-sm font-semibold ${linkClass}`}
                >
                  <DownloadIcon fontSize="1rem" aria-hidden />
                  Installer med ett klikk
                </a>
                <BodyShort size="small" className="verktoy-subtle">
                  {item.type === "agent" && "Skriv @ og navnet i Copilot Chat for å bruke den."}
                  {item.type === "instruction" && `Lastes automatisk for filer som matcher ${item.applyTo}.`}
                  {item.type === "prompt" && `Kjør med ${item.invocation} i Copilot Chat.`}
                </BodyShort>
              </VStack>
            )}

            {activeClient === "gh" && (
              <VStack gap="space-12">
                <VStack gap="space-4">
                  <BodyShort size="small" weight="semibold">
                    Installer med <code className="verktoy-code-inline">gh skill</code> (krever gh ≥2.90.0):
                  </BodyShort>
                  <CommandBlock command={getGhSkillInstallCommand(item)} />
                </VStack>
                <VStack gap="space-4">
                  <BodyShort size="small" className="verktoy-subtle">
                    Nyttige flagg:
                  </BodyShort>
                  <CommandBlock
                    command={`--scope user    # personlig (alle repoer)\n--pin v1.0.0    # lås til versjon eller SHA\n--agent <name>  # velg agent (copilot, claude, cursor …)`}
                  />
                </VStack>
                <BodyShort size="small" className="verktoy-subtle">
                  Oppdater med <code className="verktoy-code-inline">gh skill update</code>.
                </BodyShort>
              </VStack>
            )}

            {activeClient === "github" && (
              <VStack gap="space-8">
                <BodyShort size="small">
                  Skal du bruke denne på GitHub.com (for eksempel i pull requests eller Copilot Chat på web), må filen
                  ligge i repoet ditt.
                </BodyShort>
                <BodyShort size="small">
                  1. Last ned filen manuelt eller via <code className="verktoy-code-inline">nav-pilot</code>.
                  <br />
                  {item.type !== "skill" ? (
                    <>
                      2. Legg den i{" "}
                      <code className="verktoy-code-inline">
                        {INSTALL_DIRS[item.type as Exclude<CustomizationType, "mcp">]}
                      </code>{" "}
                      i default branch.
                      <br />
                    </>
                  ) : (
                    <>
                      2. Legg den i <code className="verktoy-code-inline">.github/skills/{item.name}</code> i default
                      branch.
                      <br />
                    </>
                  )}
                  3. Neste gang du bruker Copilot Chat på web i repoet, er den klar til bruk.
                </BodyShort>
              </VStack>
            )}
          </VStack>
        </Box>
      )}

      {item.type === "agent" && item.tools.length > 0 && <ToolList tools={item.tools} />}

      {item.type === "agent" && item.agentReferences && item.agentReferences.length > 0 && (
        <AgentReferences references={item.agentReferences} allItems={allItems} onNavigate={onNavigate} />
      )}

      {(item.type === "agent" || item.type === "prompt") && item.model && item.model.length > 0 && (
        <VStack gap="space-8">
          <Heading size="xsmall" level="4">
            Modell
          </Heading>
          <HStack gap="space-4" wrap>
            {item.model.map((m) => (
              <Tag key={m} size="xsmall" variant="neutral">
                {m}
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
          <div>
            <code className="verktoy-code-inline">{item.applyTo}</code>
          </div>
        </VStack>
      )}

      {item.type === "prompt" && (
        <VStack gap="space-8">
          <Heading size="xsmall" level="4">
            Aktivering
          </Heading>
          <div>
            <code className="verktoy-code-inline">{item.invocation}</code>
          </div>
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
    </VStack>
  );
}
