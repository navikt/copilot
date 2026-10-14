import { Box, VStack, BodyShort, HStack } from "@navikt/ds-react";
import { itemKey } from "@/lib/item-key";
import {
  ArrowRightIcon,
  WrenchIcon,
  RocketIcon,
  BranchingIcon,
  FileTextIcon,
  ChatIcon,
  ClockIcon,
} from "@navikt/aksel-icons";
import NextLink from "next/link";
import { getAllCustomizations } from "@/lib/customizations";
import { getRecentlyUpdatedCustomizations } from "@/lib/recent-updates";
import type { CustomizationType } from "@/lib/customization-types";
import { WeeklyTip } from "@/components/weekly-tip";

const TYPE_ICONS: Record<CustomizationType, typeof RocketIcon> = {
  agent: RocketIcon,
  skill: BranchingIcon,
  instruction: FileTextIcon,
  prompt: ChatIcon,
  mcp: WrenchIcon,
};

const RECENT_UPDATES_LIMIT = 8;

function RecentUpdates() {
  const updates = getRecentlyUpdatedCustomizations(RECENT_UPDATES_LIMIT);
  const fallbackItems =
    updates.length === 0
      ? getAllCustomizations()
          .filter((c) => c.type !== "mcp")
          .slice(0, RECENT_UPDATES_LIMIT)
      : [];

  return (
    <Box background="default" borderColor="neutral-subtleA" borderWidth="1" borderRadius="8" padding="space-16">
      <VStack gap="space-12">
        <HStack gap="space-8" align="center">
          {updates.length === 0 ? (
            <>
              <WrenchIcon aria-hidden fontSize="1rem" className="text-text-subtle" />
              <BodyShort size="small" weight="semibold" className="uppercase tracking-wide text-text-subtle">
                Verktøy du bør kjenne til
              </BodyShort>
            </>
          ) : (
            <>
              <ClockIcon aria-hidden fontSize="1rem" className="text-text-subtle" />
              <BodyShort size="small" weight="semibold" className="uppercase tracking-wide text-text-subtle">
                Sist oppdatert
              </BodyShort>
            </>
          )}
        </HStack>
        <div className="divide-y divide-border-subtle">
          {updates.length === 0
            ? fallbackItems.map((item) => {
                const Icon = TYPE_ICONS[item.type];
                return (
                  <Box key={`${item.type}-${item.id}`} paddingBlock="space-12">
                    <NextLink
                      href={`/verktoy?type=${item.type}&item=${encodeURIComponent(itemKey(item))}`}
                      className="block no-underline hover:underline"
                    >
                      <VStack gap="space-4">
                        <HStack gap="space-4" align="center">
                          <Icon aria-hidden fontSize="0.875rem" className="text-text-subtle shrink-0" />
                          <BodyShort size="small" weight="semibold">
                            {item.name}
                          </BodyShort>
                        </HStack>
                        <BodyShort size="small" className="text-text-subtle line-clamp-2">
                          {item.description}
                        </BodyShort>
                      </VStack>
                    </NextLink>
                  </Box>
                );
              })
            : updates.map(({ item, commitMessage, date }) => {
                const Icon = TYPE_ICONS[item.type];
                return (
                  <Box key={`${item.type}-${item.id}-${date}-${commitMessage}`} paddingBlock="space-12">
                    <NextLink
                      href={`/verktoy?type=${item.type}&item=${encodeURIComponent(itemKey(item))}`}
                      className="block no-underline hover:underline"
                    >
                      <VStack gap="space-4">
                        <HStack gap="space-4" align="center" justify="space-between">
                          <HStack gap="space-4" align="center">
                            <Icon aria-hidden fontSize="0.875rem" className="text-text-subtle shrink-0" />
                            <BodyShort size="small" weight="semibold">
                              {item.name}
                            </BodyShort>
                          </HStack>
                          <BodyShort size="small" className="text-text-subtle opacity-60 tabular-nums">
                            {date}
                          </BodyShort>
                        </HStack>
                        <BodyShort size="small" className="text-text-subtle line-clamp-2">
                          {commitMessage}
                        </BodyShort>
                      </VStack>
                    </NextLink>
                  </Box>
                );
              })}
        </div>
        <NextLink href="/verktoy" className="text-sm no-underline hover:underline flex items-center gap-1">
          Se alle verktøy <ArrowRightIcon aria-hidden fontSize="0.875rem" />
        </NextLink>
      </VStack>
    </Box>
  );
}

export function Sidebar() {
  return (
    <aside aria-label="Redaksjonelt innhold" className="hidden lg:block">
      <div className="sticky top-8">
        <VStack gap="space-16">
          <WeeklyTip />
          <RecentUpdates />
        </VStack>
      </div>
    </aside>
  );
}

export function SidebarCompact() {
  return (
    <Box paddingBlock="space-16" className="lg:hidden">
      <RecentUpdates />
    </Box>
  );
}
