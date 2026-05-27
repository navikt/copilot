import { Box, VStack, BodyShort, HStack } from "@navikt/ds-react";
import {
  ArrowRightIcon,
  PadlockLockedIcon,
  WrenchIcon,
  ArrowUndoIcon,
  RocketIcon,
  BranchingIcon,
  FileTextIcon,
  ChatIcon,
  ClockIcon,
} from "@navikt/aksel-icons";
import NextLink from "next/link";
import { getAllCustomizations } from "@/lib/customizations";
import { getRecentlyUpdatedCustomizations } from "@/lib/recent-updates";
import { NAV_ITEMS } from "@/lib/nav-items";
import { WeeklyTip } from "./weekly-tip";
import type { CustomizationType } from "@/lib/customization-types";

const TYPE_ICONS: Record<CustomizationType, typeof RocketIcon> = {
  agent: RocketIcon,
  skill: BranchingIcon,
  instruction: FileTextIcon,
  prompt: ChatIcon,
  mcp: WrenchIcon,
};

function RecentUpdates() {
  const updates = getRecentlyUpdatedCustomizations(3);

  if (updates.length === 0) {
    const customizations = getAllCustomizations().filter((c) => c.type !== "mcp");
    const items = customizations.slice(0, 3);
    return (
      <VStack gap="space-12">
        <HStack gap="space-4" align="center">
          <WrenchIcon aria-hidden fontSize="1rem" className="text-text-subtle" />
          <BodyShort size="small" weight="semibold" className="uppercase tracking-wide text-text-subtle">
            Verktøy du bør kjenne til
          </BodyShort>
        </HStack>
        <VStack gap="space-8">
          {items.map((item) => {
            const Icon = TYPE_ICONS[item.type];
            return (
              <NextLink
                key={item.id}
                href={`/verktoy?type=${item.type}&item=mcp-${item.id}`}
                className="no-underline hover:underline"
              >
                <VStack gap="space-2">
                  <HStack gap="space-4" align="center">
                    <Icon aria-hidden fontSize="0.875rem" className="text-text-subtle shrink-0" />
                    <BodyShort size="small" weight="semibold">
                      {item.name}
                    </BodyShort>
                  </HStack>
                  <BodyShort size="small" className="text-text-subtle line-clamp-1">
                    {item.description}
                  </BodyShort>
                </VStack>
              </NextLink>
            );
          })}
        </VStack>
        <NextLink href="/verktoy" className="text-sm no-underline hover:underline flex items-center gap-1">
          Se alle verktøy
          <ArrowRightIcon aria-hidden fontSize="0.875rem" />
        </NextLink>
      </VStack>
    );
  }

  return (
    <VStack gap="space-12">
      <HStack gap="space-4" align="center">
        <ClockIcon aria-hidden fontSize="1rem" className="text-text-subtle" />
        <BodyShort size="small" weight="semibold" className="uppercase tracking-wide text-text-subtle">
          Sist oppdatert
        </BodyShort>
      </HStack>
      <VStack gap="space-8">
        {updates.map(({ item, commitMessage, date }) => {
          const Icon = TYPE_ICONS[item.type];
          return (
            <NextLink
              key={item.id}
              href={`/verktoy?type=${item.type}&item=mcp-${item.id}`}
              className="no-underline hover:underline"
            >
              <VStack gap="space-2">
                <HStack gap="space-4" align="center">
                  <Icon aria-hidden fontSize="0.875rem" className="text-text-subtle shrink-0" />
                  <BodyShort size="small" weight="semibold">
                    {item.name}
                  </BodyShort>
                </HStack>
                <BodyShort size="small" className="text-text-subtle line-clamp-2">
                  {commitMessage}
                </BodyShort>
                <BodyShort size="small" className="text-text-subtle opacity-60">
                  {date}
                </BodyShort>
              </VStack>
            </NextLink>
          );
        })}
      </VStack>
      <NextLink href="/verktoy" className="text-sm no-underline hover:underline flex items-center gap-1">
        Se alle verktøy
        <ArrowRightIcon aria-hidden fontSize="0.875rem" />
      </NextLink>
    </VStack>
  );
}

function QuickNav() {
  const links = NAV_ITEMS.slice(0, 6);

  return (
    <VStack gap="space-8">
      <HStack gap="space-4" align="center">
        <ArrowUndoIcon aria-hidden fontSize="1rem" className="text-text-subtle" />
        <BodyShort size="small" weight="semibold" className="uppercase tracking-wide text-text-subtle">
          Gå videre
        </BodyShort>
      </HStack>
      <nav aria-label="Hurtignavigasjon">
        <VStack gap="space-4" asChild>
          <ul className="list-none">
            {links.map(({ href, icon: Icon, label, requiresAuth }) => (
              <li key={href}>
                <NextLink
                  href={href}
                  prefetch={requiresAuth ? false : undefined}
                  className="no-underline text-sm hover:underline flex items-center gap-2"
                >
                  <Icon aria-hidden fontSize="1rem" className="text-text-subtle shrink-0" />
                  {label}
                  {requiresAuth && (
                    <PadlockLockedIcon
                      aria-label="Krever innlogging"
                      fontSize="0.75rem"
                      className="text-text-subtle opacity-60"
                    />
                  )}
                </NextLink>
              </li>
            ))}
          </ul>
        </VStack>
      </nav>
    </VStack>
  );
}

export function Sidebar() {
  return (
    <aside aria-label="Redaksjonelt innhold" className="hidden lg:block">
      <div className="sticky top-8">
        <VStack gap="space-12">
          <WeeklyTip />
          <hr className="border-gray-200" />
          <RecentUpdates />
          <hr className="border-gray-200" />
          <QuickNav />
        </VStack>
      </div>
    </aside>
  );
}

export function SidebarCompact() {
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-6 lg:hidden">
      <Box paddingBlock="space-8">
        <WeeklyTip />
      </Box>
      <Box paddingBlock="space-8">
        <RecentUpdates />
      </Box>
    </div>
  );
}
