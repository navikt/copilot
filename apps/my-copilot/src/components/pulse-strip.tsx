import { Suspense } from "react";
import { Box, HGrid, Heading, BodyShort, HStack, VStack, Skeleton } from "@navikt/ds-react";
import { ArrowRightIcon } from "@navikt/aksel-icons";
import NextLink from "next/link";
import { getAllCustomizations } from "@/lib/customizations";
import { getCopilotUsageMetrics, getAdoptionData } from "@/lib/cached-bigquery";
import { getAggregatedMetrics } from "@/lib/data-utils";
import { getUserToken } from "@/lib/auth";

function HighlightCard({
  href,
  title,
  prefetch,
  compact = false,
  children,
}: {
  href: string;
  title: string;
  prefetch?: boolean;
  compact?: boolean;
  children: React.ReactNode;
}) {
  return (
    <Box
      background="neutral-soft"
      borderRadius="8"
      padding="space-12"
      className={compact ? "self-start" : "h-full"}
      asChild
    >
      <NextLink
        href={href}
        prefetch={prefetch}
        className={`no-underline hover:shadow-md transition-shadow group ${compact ? "" : "h-full"}`}
      >
        <VStack gap="space-4" className={compact ? "" : "h-full"}>
          <HStack gap="space-4" align="center" justify="space-between">
            <BodyShort size="small" weight="semibold">
              {title}
            </BodyShort>
            <ArrowRightIcon
              aria-hidden
              fontSize="1rem"
              className="text-text-subtle opacity-0 group-hover:opacity-100 transition-opacity"
            />
          </HStack>
          {children}
        </VStack>
      </NextLink>
    </Box>
  );
}

function HighlightSkeleton() {
  return (
    <Box background="neutral-soft" borderRadius="8" padding="space-12">
      <Skeleton variant="text" width={100} height={20} />
      <Skeleton variant="rectangle" width="100%" height={40} className="mt-1" />
    </Box>
  );
}

function CustomizationBreakdownCard() {
  const customizations = getAllCustomizations();
  const types = [
    { label: "Agenter", count: customizations.filter((c) => c.type === "agent").length },
    { label: "Skills", count: customizations.filter((c) => c.type === "skill").length },
    { label: "Instruksjoner", count: customizations.filter((c) => c.type === "instruction").length },
    { label: "Prompts", count: customizations.filter((c) => c.type === "prompt").length },
  ].filter((t) => t.count > 0);

  const maxCount = Math.max(...types.map((t) => t.count));

  return (
    <HighlightCard compact href="/verktoy" title={`${customizations.length} tilpasninger`}>
      <VStack gap="space-4">
        {types.map((t) => (
          <div key={t.label} className="flex items-center gap-2">
            <BodyShort size="small" className="w-24 shrink-0">
              {t.label}
            </BodyShort>
            <div className="flex-1 h-2 rounded-full bg-gray-200 overflow-hidden">
              <div className="h-full rounded-full bg-blue-500" style={{ width: `${(t.count / maxCount) * 100}%` }} />
            </div>
            <BodyShort size="small" className="text-text-subtle w-6 text-right">
              {t.count}
            </BodyShort>
          </div>
        ))}
      </VStack>
    </HighlightCard>
  );
}

// A stat as text, not a heading: the numbers sit before the page's H2s (#1193).
function Stat({ children }: { children: React.ReactNode }) {
  return (
    <Heading as="p" size="medium">
      {children}
    </Heading>
  );
}

// Without a login or data there are no numbers to show. The cards used to show
// made-up fallbacks (57/45/30 %), which read as real figures (#1193).
function NoData({ loggedIn }: { loggedIn: boolean }) {
  return (
    <BodyShort size="small" className="text-text-subtle">
      {loggedIn ? "Fant ingen tall akkurat nå." : "Logg inn for å se tallene."}
    </BodyShort>
  );
}

async function UsageCard() {
  const token = await getUserToken();
  const { usage, error } = token ? await getCopilotUsageMetrics(token) : { usage: null, error: "Not authenticated" };
  const metrics = !error && usage?.length ? getAggregatedMetrics(usage) : null;

  if (!metrics?.monthlyActiveUsers) {
    return (
      <HighlightCard href="/statistikk" prefetch={false} title="Bruksmønster">
        <NoData loggedIn={!!token} />
      </HighlightCard>
    );
  }

  // Chat and agent have monthly counts, CLI only a daily one, so each share
  // uses the base of its own period. One person can count in all three.
  const items = [
    {
      label: "Chat",
      pct: Math.round((metrics.monthlyActiveChatUsers / metrics.monthlyActiveUsers) * 100),
      color: "bg-blue-500",
    },
    {
      label: "Agent",
      pct: Math.round((metrics.monthlyActiveAgentUsers / metrics.monthlyActiveUsers) * 100),
      color: "bg-violet-500",
    },
    {
      label: "CLI",
      // A day with no active users has no CLI share; show a dash, not the whole card as empty.
      pct: metrics.dailyActiveUsers
        ? Math.round((metrics.dailyActiveCLIUsers / metrics.dailyActiveUsers) * 100)
        : undefined,
      color: "bg-amber-500",
    },
  ];

  return (
    <HighlightCard href="/statistikk" prefetch={false} title="Bruksmønster">
      <HStack gap="space-4" className="w-full" justify="space-between">
        {items.map((item) => (
          <VStack key={item.label} align="center" gap="space-2" className="flex-1">
            <Stat>{item.pct === undefined ? "–" : `${item.pct} %`}</Stat>
            <HStack gap="space-4" align="center">
              <span className={`inline-block w-2 h-2 rounded-full ${item.color}`} />
              <BodyShort size="small" className="text-text-subtle">
                {item.label}
              </BodyShort>
            </HStack>
          </VStack>
        ))}
      </HStack>
      <BodyShort size="small" className="text-text-subtle">
        Andel av månedens aktive brukere som brukte chat og agent, og av dagens som brukte CLI. Én bruker kan telle
        flere steder.
      </BodyShort>
    </HighlightCard>
  );
}

async function StatsCard() {
  const token = await getUserToken();
  const [{ usage, error: usageError }, { data: adoptionData, error: adoptionError }] = token
    ? await Promise.all([getCopilotUsageMetrics(token), getAdoptionData(token)])
    : [
        { usage: null, error: "Not authenticated" },
        { data: null, error: "Not authenticated" },
      ];

  const metrics = !usageError && usage?.length ? getAggregatedMetrics(usage) : null;
  const acceptanceRate = metrics?.overallAcceptanceRate;

  const summary = !adoptionError && adoptionData?.summary ? adoptionData.summary : null;
  const adoptionRate =
    summary && summary.active_repos_with_recent_commits > 0
      ? Math.round((summary.repos_with_any_customization / summary.active_repos_with_recent_commits) * 100)
      : undefined;

  if (acceptanceRate === undefined && adoptionRate === undefined) {
    return (
      <HighlightCard compact href="/statistikk" prefetch={false} title="Nøkkeltall">
        <NoData loggedIn={!!token} />
      </HighlightCard>
    );
  }

  return (
    <HighlightCard compact href="/statistikk" prefetch={false} title="Nøkkeltall">
      <div className="grid grid-cols-2 gap-4">
        <div className="min-w-0">
          <Stat>{acceptanceRate === undefined ? "–" : `${acceptanceRate} %`}</Stat>
          <BodyShort size="small" className="text-text-subtle">
            akseptrate for kodeforslag
          </BodyShort>
        </div>
        <div className="min-w-0">
          <Stat>{adoptionRate === undefined ? "–" : `${adoptionRate} %`}</Stat>
          <BodyShort size="small" className="text-text-subtle">
            av repoer har tilpasninger
          </BodyShort>
        </div>
      </div>
    </HighlightCard>
  );
}

export function HighlightCards() {
  return (
    <HGrid columns={{ xs: 1, sm: 3 }} gap="space-12">
      <CustomizationBreakdownCard />
      <Suspense fallback={<HighlightSkeleton />}>
        <UsageCard />
      </Suspense>
      <Suspense fallback={<HighlightSkeleton />}>
        <StatsCard />
      </Suspense>
    </HGrid>
  );
}
