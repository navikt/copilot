"use client";

import { Dialog, DialogBackdrop, DialogPanel, DialogTitle } from "@headlessui/react";
import { XMarkIcon, ExternalLinkIcon } from "@navikt/aksel-icons";
import { Box, BodyShort, Heading, Tag, HStack, VStack } from "@navikt/ds-react";
import { itemKey } from "@/lib/item-key";
import type { EnrichedCustomization } from "@/lib/enrich-customizations";
import { DOMAIN_CONFIGS } from "@/lib/customization-types";
import { linkClass } from "@/components/nav-pilot/doc-page";
import { Contributors } from "../contributors";
import { TypeBadge, TypeIconBadge } from "../verktoy-type-meta";
import { McpDetails, isFleetGatewayMcp } from "./mcp-details";
import { StaticCustomizationDetails } from "./static-details";

export interface DetailDrawerProps {
  item: EnrichedCustomization | null;
  allItems: EnrichedCustomization[];
  open: boolean;
  onClose: () => void;
  onNavigate?: (item: EnrichedCustomization) => void;
}

// The registry has no owner field, so MCP servers get no owner rather than a guess.
function ownerLabel(item: EnrichedCustomization): string | null {
  return item.type === "mcp" ? null : "nav-pilot";
}

export function DetailDrawer({ item, allItems, open, onClose, onNavigate }: DetailDrawerProps) {
  if (!item) return null;

  const domainConfig = DOMAIN_CONFIGS[item.domain];

  return (
    <Dialog open={open} onClose={onClose} className="relative z-50">
      <DialogBackdrop
        transition
        className="fixed inset-0 bg-black/35 transition-opacity motion-reduce:transition-none data-closed:opacity-0 data-enter:duration-200 data-enter:ease-out data-leave:duration-150 data-leave:ease-in"
      />

      <div className="fixed inset-0 overflow-hidden">
        <div className="absolute inset-0 overflow-hidden">
          <div
            className="pointer-events-none fixed inset-y-0 right-0 flex max-w-full"
            style={{ paddingLeft: "var(--ax-space-24)" }}
          >
            <DialogPanel
              transition
              className="pointer-events-auto w-screen max-w-lg transform transition motion-reduce:transition-none data-closed:translate-x-full data-enter:duration-200 data-enter:ease-out data-leave:duration-150 data-leave:ease-in"
            >
              <div className="verktoy-drawer-panel flex h-full flex-col overflow-y-auto shadow-xl">
                <Box
                  paddingBlock="space-16"
                  paddingInline="space-20"
                  style={{ borderBottom: "1px solid var(--ax-border-neutral-subtle)" }}
                >
                  <HStack gap="space-12" align="start" justify="space-between" wrap={false}>
                    <HStack gap="space-12" align="center" className="min-w-0 flex-1" wrap={false}>
                      <TypeIconBadge type={item.type} size="lg" />
                      <DialogTitle as="div" className="min-w-0">
                        <Heading size="medium" level="3" className="break-words">
                          {item.type === "agent" ? `@${item.name}` : item.name}
                        </Heading>
                      </DialogTitle>
                    </HStack>
                    <button type="button" onClick={onClose} className="verktoy-drawer-close" aria-label="Lukk panel">
                      <XMarkIcon fontSize="1.5rem" aria-hidden />
                    </button>
                  </HStack>
                </Box>

                <Box paddingBlock="space-16" paddingInline="space-20" className="flex-1">
                  <VStack gap="space-16">
                    <HStack gap="space-6" wrap>
                      <TypeBadge type={item.type} size="small" />
                      <Tag size="small" variant="info">
                        {domainConfig.label}
                      </Tag>
                      {isFleetGatewayMcp(item) && (
                        <Tag size="small" variant="neutral">
                          Via fleet-gateway
                        </Tag>
                      )}
                    </HStack>

                    <BodyShort>{item.description}</BodyShort>

                    <HStack gap="space-16" wrap>
                      {item.usageCount > 0 && (
                        <BodyShort size="small" className="verktoy-subtle">
                          Brukt i <strong>{item.usageCount}</strong> {item.usageCount === 1 ? "repo" : "repoer"}
                        </BodyShort>
                      )}
                      {ownerLabel(item) && (
                        <BodyShort size="small" className="verktoy-subtle">
                          Eier: <strong>{ownerLabel(item)}</strong>
                        </BodyShort>
                      )}
                    </HStack>

                    {item.usageCount > 0 && item.usedBy.length > 0 && (
                      <HStack gap="space-4" wrap>
                        {item.usedBy.map((repo, i) => (
                          <span key={repo} className="inline-flex items-center">
                            <a
                              href={`https://github.com/navikt/${repo}`}
                              target="_blank"
                              rel="noopener noreferrer"
                              className={`text-sm ${linkClass}`}
                            >
                              {repo}
                            </a>
                            {i < item.usedBy.length - 1 || item.usageCount > item.usedBy.length ? (
                              <span className="verktoy-subtle">,</span>
                            ) : null}
                          </span>
                        ))}
                        {item.usageCount > item.usedBy.length && (
                          <BodyShort size="small" as="span" className="verktoy-subtle">
                            +{item.usageCount - item.usedBy.length} andre
                          </BodyShort>
                        )}
                      </HStack>
                    )}

                    {item.type === "mcp" ? (
                      <McpDetails item={item} />
                    ) : (
                      <StaticCustomizationDetails item={item} allItems={allItems} onNavigate={onNavigate} />
                    )}

                    {item.type !== "mcp" && <Contributors key={itemKey(item)} itemId={itemKey(item)} />}
                  </VStack>
                </Box>

                <Box
                  paddingBlock="space-12"
                  paddingInline="space-20"
                  style={{ borderTop: "1px solid var(--ax-border-neutral-subtle)" }}
                >
                  <a
                    href={
                      item.type === "mcp" && item.repository?.url
                        ? item.repository.url
                        : `https://github.com/navikt/copilot/blob/main/${item.filePath}`
                    }
                    target="_blank"
                    rel="noopener noreferrer"
                    className={`inline-flex items-center gap-1 text-sm ${linkClass}`}
                  >
                    <ExternalLinkIcon fontSize="1rem" aria-hidden />
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
