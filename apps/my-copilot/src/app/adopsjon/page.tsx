import React, { Suspense } from "react";
import { getCachedAdoptionData } from "@/lib/cached-bigquery";
import Tabs from "@/components/tabs";
import {
  CustomizationTypeChart,
  TeamAdoptionChart,
  LanguageAdoptionChart,
  TopCustomizationsChart,
} from "@/components/charts/adoption";
import MetricCard from "@/components/metric-card";
import ErrorState from "@/components/error-state";
import { Box, Heading, HGrid, Skeleton, VStack, BodyShort } from "@navikt/ds-react";
import { PageHero } from "@/components/page-hero";
import TeamTable from "@/components/team-table";
import { formatNumber } from "@/lib/format";
import {
  calculateTeamStats,
  calculateLanguageStats,
  formatAdoptionRate,
  formatScanDate,
} from "@/lib/adoption-utils";
import type { AdoptionData } from "@/lib/types";

// Static header component
function AdoptionHeader() {
  return (
    <PageHero title="Adopsjon" description="AI-tilpasninger på tvers av navikt-repoer. Data fra ukentlig skanning." />
  );
}

// Loading skeleton
function LoadingSkeleton() {
  return (
    <VStack gap="space-24">
      <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-16">
        {[1, 2, 3, 4].map((i) => (
          <Skeleton key={i} variant="rounded" height={100} />
        ))}
      </HGrid>
      <Skeleton variant="rounded" height={400} />
    </VStack>
  );
}

// Overview tab content
function OverviewContent({ data }: { data: AdoptionData }) {
  const { summary } = data;

  if (!summary) {
    return <ErrorState message="Ingen adopsjonsdata tilgjengelig" />;
  }

  const adoptionPercent = formatAdoptionRate(summary.adoption_rate, 1);

  return (
    <VStack gap="space-24">
      <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-16">
        <MetricCard
          value={adoptionPercent}
          label="Adopsjonsrate"
          helpTitle="Adopsjonsrate"
          helpText="Andel aktive repoer med minst én Copilot-tilpasning"
        />
        <MetricCard
          value={formatNumber(summary.repos_with_any_customization)}
          label="Repoer med tilpasninger"
          helpTitle="Repoer med tilpasninger"
          helpText="Antall aktive repoer med minst én Copilot-tilpasning"
          subtitle={`av ${formatNumber(summary.active_repos)} aktive`}
        />
        <MetricCard
          value={formatNumber(summary.repos_with_copilot_instructions)}
          label="copilot-instructions.md"
          helpTitle="Copilot Instructions"
          helpText="Repoer med .github/copilot-instructions.md"
        />
      </HGrid>

      <CustomizationTypeChart data={summary} />
    </VStack>
  );
}

// Team tab content
function TeamContent({ data }: { data: AdoptionData }) {
  const { teams } = data;

  if (!teams || teams.length === 0) {
    return <ErrorState message="Ingen teamdata tilgjengelig" />;
  }

  const stats = calculateTeamStats(teams);

  return (
    <VStack gap="space-24">
      <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-16">
        <MetricCard
          value={formatNumber(stats.totalTeams)}
          label="Team totalt"
          helpTitle="Team totalt"
          helpText="Antall team med minst ett aktivt repo"
        />
        <MetricCard
          value={formatNumber(stats.teamsWithAdoption)}
          label="Team med tilpasninger"
          helpTitle="Team med tilpasninger"
          helpText="Team som har minst ett repo med Copilot-tilpasninger"
          subtitle={`${stats.adoptionPercent.toFixed(0)}% av team`}
        />
        <MetricCard
          value={formatNumber(stats.totalReposWithCustomizations)}
          label="Repoer totalt"
          helpTitle="Tilpassede repoer"
          helpText="Sum av repoer med Copilot-tilpasninger på tvers av team"
        />
      </HGrid>

      <TeamAdoptionChart data={teams} maxTeams={10} />

      <Box background="default" padding="space-20" borderRadius="8" className="border border-gray-200">
        <VStack gap="space-16">
          <Heading size="small" level="4">
            Alle team
          </Heading>
          <TeamTable teams={teams.filter((t) => t.active_repos > 0)} />
        </VStack>
      </Box>
    </VStack>
  );
}

// Language tab content
function LanguageContent({ data }: { data: AdoptionData }) {
  const { languages } = data;

  if (!languages || languages.length === 0) {
    return <ErrorState message="Ingen språkdata tilgjengelig" />;
  }

  const stats = calculateLanguageStats(languages);

  return (
    <VStack gap="space-24">
      <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-16">
        <MetricCard
          value={formatNumber(stats.totalLanguages)}
          label="Språk totalt"
          helpTitle="Språk totalt"
          helpText="Antall programmeringsspråk med minst 5 repoer"
        />
        <MetricCard
          value={stats.topLanguage?.language ?? "—"}
          label="Høyest adopsjon"
          helpTitle="Høyest adopsjon"
          helpText={stats.topLanguage ? `Språket med høyest Copilot-adopsjonsrate (${formatAdoptionRate(stats.topLanguage.adoption_rate)})` : "Ingen data"}
          subtitle={stats.topLanguage ? `${formatAdoptionRate(stats.topLanguage.adoption_rate)} av ${stats.topLanguage.total_repos} repoer` : undefined}
        />
        <MetricCard
          value={formatNumber(stats.totalReposWithCustomizations)}
          label="Repoer med tilpasninger"
          helpTitle="Repoer med tilpasninger"
          helpText="Sum av repoer på tvers av alle språk"
        />
      </HGrid>

      <LanguageAdoptionChart data={languages} maxLanguages={15} />
    </VStack>
  );
}

// Topp-tilpasninger tab content
function TopCustomizationsContent({ data }: { data: AdoptionData }) {
  const { customizationDetails } = data;

  if (!customizationDetails || customizationDetails.length === 0) {
    return <ErrorState message="Ingen data om tilpasninger tilgjengelig" />;
  }

  const totalFiles = customizationDetails.length;
  const topFile = customizationDetails[0];

  return (
    <VStack gap="space-24">
      <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-16">
        <MetricCard
          value={formatNumber(totalFiles)}
          label="Unike tilpasninger"
          helpTitle="Unike tilpasninger"
          helpText="Antall unike filer (agenter, skills, instruksjoner, prompts) på tvers av alle repoer"
        />
        <MetricCard
          value={topFile.file_name}
          label="Mest brukte"
          helpTitle="Mest brukte tilpasning"
          helpText={`Den mest brukte tilpasningen på tvers av navikt-repoer`}
          subtitle={`${formatNumber(topFile.repo_count)} repoer`}
        />
        <MetricCard
          value={formatNumber(new Set(customizationDetails.map((d) => d.category)).size)}
          label="Kategorier"
          helpTitle="Kategorier"
          helpText="Antall kategorier med tilpasninger (agenter, skills, instruksjoner, prompts)"
        />
      </HGrid>

      <TopCustomizationsChart data={customizationDetails} />
    </VStack>
  );
}

// Cached data component
async function CachedAdoptionData() {
  const { data, error } = await getCachedAdoptionData();

  if (error) return <ErrorState message={`Feil ved henting av adopsjonsdata: ${error}`} />;
  if (!data) return <ErrorState message="Ingen adopsjonsdata tilgjengelig" />;

  const scanDate = data.summary ? formatScanDate(data.summary.scan_date) : null;

  const tabs = [
    {
      id: "oversikt",
      label: "Oversikt",
      content: <OverviewContent data={data} />,
    },
    {
      id: "tilpasninger",
      label: "Topp-tilpasninger",
      content: <TopCustomizationsContent data={data} />,
    },
    {
      id: "team",
      label: "Team",
      content: <TeamContent data={data} />,
    },
    {
      id: "sprak",
      label: "Språk",
      content: <LanguageContent data={data} />,
    },
  ];

  return (
    <VStack gap="space-24">
      {scanDate && (
        <BodyShort className="text-gray-600">
          Siste skanning: {scanDate} • Viser Copilot-tilpasninger på tvers av navikt-repoer
        </BodyShort>
      )}
      <Tabs tabs={tabs} />
    </VStack>
  );
}

export default function AdoptionPage() {
  return (
    <>
      <AdoptionHeader />
      <main className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <Suspense fallback={<LoadingSkeleton />}>
            <CachedAdoptionData />
          </Suspense>
        </Box>
      </main>
    </>
  );
}
