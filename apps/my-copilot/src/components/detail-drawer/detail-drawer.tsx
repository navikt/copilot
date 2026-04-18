"use client";

import { Dialog, DialogBackdrop, DialogPanel, DialogTitle } from "@headlessui/react";
import { XMarkIcon, ExternalLinkIcon } from "@navikt/aksel-icons";
import { Box, BodyShort, Heading, Tag, HStack, VStack } from "@navikt/ds-react";
import type { EnrichedCustomization } from "@/lib/enrich-customizations";
import { DOMAIN_CONFIGS, TYPE_LABELS } from "@/lib/customization-types";
import { Contributors } from "../contributors";
import { McpDetails } from "./mcp-details";
import { StaticCustomizationDetails } from "./static-details";

export interface DetailDrawerProps {
  item: EnrichedCustomization | null;
  allItems: EnrichedCustomization[];
  open: boolean;
  onClose: () => void;
  onNavigate?: (item: EnrichedCustomization) => void;
}

export function DetailDrawer({ item, allItems, open, onClose, onNavigate }: DetailDrawerProps) {
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

                    {item.type !== "mcp" && <Contributors key={item.id} itemId={item.id} />}

                    {item.type === "mcp" ? (
                      <McpDetails item={item} />
                    ) : (
                      <StaticCustomizationDetails item={item} allItems={allItems} onNavigate={onNavigate} />
                    )}
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
