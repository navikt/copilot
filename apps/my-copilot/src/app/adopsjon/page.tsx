import React, { Suspense } from "react";
import { getCachedAdoptionData } from "@/lib/cached-bigquery";
import Tabs from "@/components/tabs";
import { CustomizationTypeChart, TeamAdoptionChart, LanguageAdoptionChart } from "@/components/charts/adoption";
import MetricCard from "@/components/metric-card";
import ErrorState from "@/components/error-state";
import { Box, Heading, HGrid, Skeleton, VStack, BodyShort, Table } from "@navikt/ds-react";
import { TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@navikt/ds-react/Table";
import { PageHero } from "@/components/page-hero";
import { formatNumber } from "@/lib/format";
import type { AdoptionData, TeamAdoption } from "@/lib/types";

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

  const adoptionPercent = (summary.adoption_rate * 100).toFixed(1);

  return (
    <VStack gap="space-24">
      <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-16">
        <MetricCard
          value={`${adoptionPercent}%`}
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

  // Filter teams with at least 1 active repo
  const activeTeams = teams.filter((t) => t.active_repos > 0);
  const teamsWithAdoption = activeTeams.filter((t) => t.repos_with_customizations > 0);

  return (
    <VStack gap="space-24">
      <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-16">
        <MetricCard
          value={formatNumber(activeTeams.length)}
          label="Team totalt"
          helpTitle="Team totalt"
          helpText="Antall team med minst ett aktivt repo"
        />
        <MetricCard
          value={formatNumber(teamsWithAdoption.length)}
          label="Team med tilpasninger"
          helpTitle="Team med tilpasninger"
          helpText="Team som har minst ett repo med Copilot-tilpasninger"
          subtitle={`${((teamsWithAdoption.length / activeTeams.length) * 100).toFixed(0)}% av team`}
        />
        <MetricCard
          value={formatNumber(teamsWithAdoption.reduce((sum, t) => sum + t.repos_with_customizations, 0))}
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
          <TeamTable teams={activeTeams} />
        </VStack>
      </Box>
    </VStack>
  );
}

// Team table component
function TeamTable({ teams }: { teams: TeamAdoption[] }) {
  const sortedTeams = [...teams].sort((a, b) => b.repos_with_customizations - a.repos_with_customizations);

  return (
    <Table size="small">
      <TableHeader>
        <TableRow>
          <TableHeaderCell>Team</TableHeaderCell>
          <TableHeaderCell align="right">Aktive repoer</TableHeaderCell>
          <TableHeaderCell align="right">Med tilpasninger</TableHeaderCell>
          <TableHeaderCell align="right">Adopsjonsrate</TableHeaderCell>
        </TableRow>
      </TableHeader>
      <TableBody>
        {sortedTeams.slice(0, 50).map((team) => (
          <TableRow key={team.team_slug}>
            <TableDataCell>{team.team_name || team.team_slug}</TableDataCell>
            <TableDataCell align="right">{team.active_repos}</TableDataCell>
            <TableDataCell align="right">{team.repos_with_customizations}</TableDataCell>
            <TableDataCell align="right">{(team.adoption_rate * 100).toFixed(0)}%</TableDataCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

// Language tab content
function LanguageContent({ data }: { data: AdoptionData }) {
  const { languages } = data;

  if (!languages || languages.length === 0) {
    return <ErrorState message="Ingen språkdata tilgjengelig" />;
  }

  const topLanguage = languages.reduce((best, lang) => (lang.adoption_rate > best.adoption_rate ? lang : best));

  return (
    <VStack gap="space-24">
      <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-16">
        <MetricCard
          value={formatNumber(languages.length)}
          label="Språk totalt"
          helpTitle="Språk totalt"
          helpText="Antall programmeringsspråk med minst 5 repoer"
        />
        <MetricCard
          value={topLanguage.language}
          label="Høyest adopsjon"
          helpTitle="Høyest adopsjon"
          helpText={`Språket med høyest Copilot-adopsjonsrate (${(topLanguage.adoption_rate * 100).toFixed(0)}%)`}
          subtitle={`${(topLanguage.adoption_rate * 100).toFixed(0)}% av ${topLanguage.total_repos} repoer`}
        />
        <MetricCard
          value={formatNumber(languages.reduce((sum, l) => sum + l.repos_with_customizations, 0))}
          label="Repoer med tilpasninger"
          helpTitle="Repoer med tilpasninger"
          helpText="Sum av repoer på tvers av alle språk"
        />
      </HGrid>

      <LanguageAdoptionChart data={languages} maxLanguages={15} />
    </VStack>
  );
}

// Cached data component
async function CachedAdoptionData() {
  const { data, error } = await getCachedAdoptionData();

  if (error) return <ErrorState message={`Feil ved henting av adopsjonsdata: ${error}`} />;
  if (!data) return <ErrorState message="Ingen adopsjonsdata tilgjengelig" />;

  const scanDate = data.summary
    ? new Date(data.summary.scan_date).toLocaleDateString("nb-NO", {
        day: "numeric",
        month: "long",
        year: "numeric",
      })
    : null;

  const tabs = [
    {
      id: "oversikt",
      label: "Oversikt",
      content: <OverviewContent data={data} />,
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
