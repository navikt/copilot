import { Suspense } from "react";
import { Box, HGrid, Heading, BodyShort, HStack, VStack, Skeleton } from "@navikt/ds-react";
import { ArrowRightIcon } from "@navikt/aksel-icons";
import NextLink from "next/link";
import { getAllCustomizations } from "@/lib/customizations";
import { getCachedBigQueryUsage, getCachedAdoptionData } from "@/lib/cached-bigquery";
import { getAggregatedMetrics } from "@/lib/data-utils";
import { getUserToken } from "@/lib/auth";

function HighlightCard({
  href,
  title,
  prefetch,
  children,
}: {
  href: string;
  title: string;
  prefetch?: boolean;
  children: React.ReactNode;
}) {
  return (
    <Box background="neutral-soft" borderRadius="8" padding="space-16" asChild>
      <NextLink href={href} prefetch={prefetch} className="no-underline hover:shadow-md transition-shadow group">
        <VStack gap="space-8">
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
    <Box background="neutral-soft" borderRadius="8" padding="space-16">
      <Skeleton variant="text" width={100} height={20} />
      <Skeleton variant="rectangle" width="100%" height={40} className="mt-2" />
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
    <HighlightCard href="/verktoy" title={`${customizations.length} tilpasninger`}>
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

async function UsageCard() {
  const token = await getUserToken();
  const { usage, error } = token ? await getCachedBigQueryUsage(token) : { usage: null, error: "Not authenticated" };

  const metrics = !error && usage?.length ? getAggregatedMetrics(usage) : null;
  const total = metrics?.monthlyActiveUsers || 1;
  const items = [
    {
      label: "Chat",
      pct: metrics ? Math.round((metrics.monthlyActiveChatUsers / total) * 100) : 57,
      color: "bg-blue-500",
    },
    {
      label: "Agent",
      pct: metrics ? Math.round((metrics.monthlyActiveAgentUsers / total) * 100) : 45,
      color: "bg-violet-500",
    },
    {
      label: "CLI",
      pct: metrics ? Math.round((metrics.dailyActiveCLIUsers / total) * 100) : 30,
      color: "bg-amber-500",
    },
  ];

  return (
    <HighlightCard href="/statistikk" prefetch={false} title="Bruksmønster">
      <HStack gap="space-8" className="w-full" justify="center">
        {items.map((item) => (
          <VStack key={item.label} align="center" gap="space-4" className="flex-1">
            <Heading size="medium" level="3">
              {item.pct} %
            </Heading>
            <HStack gap="space-4" align="center">
              <span className={`inline-block w-2 h-2 rounded-full ${item.color}`} />
              <BodyShort size="small" className="text-text-subtle">
                {item.label}
              </BodyShort>
            </HStack>
          </VStack>
        ))}
      </HStack>
    </HighlightCard>
  );
}

async function StatsCard() {
  const token = await getUserToken();
  const [{ usage, error: usageError }, { data: adoptionData, error: adoptionError }] = token
    ? await Promise.all([getCachedBigQueryUsage(token), getCachedAdoptionData(token)])
    : [
        { usage: null, error: "Not authenticated" },
        { data: null, error: "Not authenticated" },
      ];

  const metrics = !usageError && usage?.length ? getAggregatedMetrics(usage) : null;
  const acceptanceRate = metrics?.overallAcceptanceRate ?? 30;

  const summary = !adoptionError && adoptionData?.summary ? adoptionData.summary : null;
  const adoptionRate =
    summary && summary.active_repos_with_recent_commits > 0
      ? Math.round((summary.repos_with_any_customization / summary.active_repos_with_recent_commits) * 100)
      : 15;

  return (
    <HighlightCard href="/statistikk" prefetch={false} title="Nøkkeltall">
      <VStack gap="space-8">
        <div>
          <Heading size="medium" level="3">
            {acceptanceRate} %
          </Heading>
          <BodyShort size="small" className="text-text-subtle">
            akseptrate for kodeforslag
          </BodyShort>
        </div>
        <div>
          <Heading size="medium" level="3">
            {adoptionRate} %
          </Heading>
          <BodyShort size="small" className="text-text-subtle">
            av repoer har tilpasninger
          </BodyShort>
        </div>
      </VStack>
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
