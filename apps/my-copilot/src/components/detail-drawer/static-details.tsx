"use client";

import { Accordion, BodyShort, Box, CopyButton, HStack, Heading, Tag, VStack } from "@navikt/ds-react";
import { DownloadIcon } from "@navikt/aksel-icons";
import type { EnrichedCustomization } from "@/lib/enrich-customizations";
import { normalizeExample } from "@/lib/manifest-types";
import { getNavPilotAddCommand, getGhSkillInstallCommand, CLIENT_SUPPORT } from "@/lib/install-commands";
import { ToolList, ExclusiveAccordion } from "./shared";

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
              className="bg-transparent border-none cursor-pointer p-0"
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
  if (item.type === "mcp") return null;

  return (
    <VStack gap="space-16">
      {item.type === "agent" && item.tools.length > 0 && <ToolList tools={item.tools} />}

      {item.type === "agent" && item.agentReferences && item.agentReferences.length > 0 && (
        <AgentReferences references={item.agentReferences} allItems={allItems} onNavigate={onNavigate} />
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
          {item.installUrl && (
            <Accordion.Item>
              <Accordion.Header>VS Code</Accordion.Header>
              <Accordion.Content>
                <VStack gap="space-8">
                  <a
                    href={item.installUrl}
                    className="inline-flex items-center gap-1 text-sm font-semibold text-blue-600 hover:underline"
                  >
                    <DownloadIcon fontSize="1rem" aria-hidden />
                    Installer med ett klikk
                  </a>
                  <BodyShort size="small" className="text-gray-500">
                    {item.type === "agent" && "Aktiver med @-mention i Copilot Chat."}
                    {item.type === "instruction" && `Lastes automatisk for filer som matcher ${item.applyTo}.`}
                    {item.type === "prompt" && `Kjør med ${item.invocation} i Copilot Chat.`}
                  </BodyShort>
                </VStack>
              </Accordion.Content>
            </Accordion.Item>
          )}
          {(() => {
            const navPilot = getNavPilotAddCommand(item);
            if (!navPilot) return null;
            return (
              <Accordion.Item>
                <Accordion.Header>nav-pilot</Accordion.Header>
                <Accordion.Content>
                  <VStack gap="space-12">
                    <VStack gap="space-4">
                      <BodyShort size="small" weight="semibold">
                        I repoet (delt med teamet):
                      </BodyShort>
                      <div className="relative">
                        <pre className="text-xs bg-gray-100 rounded p-2 pr-10 overflow-x-auto whitespace-pre-wrap break-all">
                          {navPilot.repo}
                        </pre>
                        <div className="absolute top-1 right-1">
                          <CopyButton size="xsmall" copyText={navPilot.repo} />
                        </div>
                      </div>
                    </VStack>
                    <VStack gap="space-4">
                      <BodyShort size="small" weight="semibold">
                        Personlig (alle repoer):
                      </BodyShort>
                      <div className="relative">
                        <pre className="text-xs bg-gray-100 rounded p-2 pr-10 overflow-x-auto whitespace-pre-wrap break-all">
                          {navPilot.user}
                        </pre>
                        <div className="absolute top-1 right-1">
                          <CopyButton size="xsmall" copyText={navPilot.user} />
                        </div>
                      </div>
                    </VStack>
                  </VStack>
                </Accordion.Content>
              </Accordion.Item>
            );
          })()}
          {CLIENT_SUPPORT[item.type].includes("gh") && (
            <Accordion.Item>
              <Accordion.Header>GitHub CLI</Accordion.Header>
              <Accordion.Content>
                <VStack gap="space-8">
                  <BodyShort size="small">
                    Installer med <code className="text-xs bg-gray-100 rounded px-1">gh skill</code> (krever gh
                    ≥2.90.0):
                  </BodyShort>
                  <div className="relative">
                    <pre className="text-xs bg-gray-100 rounded p-2 pr-10 overflow-x-auto whitespace-pre-wrap break-all">
                      {getGhSkillInstallCommand(item)}
                    </pre>
                    <div className="absolute top-1 right-1">
                      <CopyButton size="xsmall" copyText={getGhSkillInstallCommand(item)} />
                    </div>
                  </div>
                  <BodyShort size="small" className="text-gray-500">
                    Installerer skill med referansefiler til ditt prosjekt. Oppdater med{" "}
                    <code className="text-xs bg-gray-100 rounded px-1">gh skill update</code>.
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
