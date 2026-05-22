import React, { Suspense } from "react";
import { getCachedAdoptionData, getCachedStalenessData } from "@/lib/cached-bigquery";
import Tabs from "@/components/tabs";
import {
  CustomizationTypeChart,
  TeamAdoptionChart,
  LanguageAdoptionChart,
  TopCustomizationsChart,
  ToolComparisonChart,
} from "@/components/charts/adoption";
import MetricCard from "@/components/metric-card";
import ErrorState from "@/components/error-state";
import { Box, Heading, HGrid, Skeleton, VStack, BodyShort, Table } from "@navikt/ds-react";
import { TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@navikt/ds-react/Table";
import { PageHero } from "@/components/page-hero";
import TeamTable from "@/components/team-table";
import { formatNumber } from "@/lib/format";
import { calculateTeamStats, calculateLanguageStats, formatAdoptionRate, formatScanDate } from "@/lib/adoption-utils";
import type { AdoptionData, StalenessSummary } from "@/lib/types";
import { getUser } from "@/lib/auth";

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
  const activeOnlyPercent = formatAdoptionRate(summary.adoption_rate_active_only, 1);

  return (
    <VStack gap="space-24">
      <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-16">
        <MetricCard
          value={activeOnlyPercent}
          label="Adopsjonsrate (aktive)"
          helpTitle="Adopsjonsrate for nylig aktive repoer"
          helpText="Andel repoer med commit siste 90 dager som har minst én AI-tilpasning"
          subtitle={`${adoptionPercent} inkl. sovende`}
        />
        <MetricCard
          value={formatNumber(summary.repos_with_any_customization)}
          label="Repoer med tilpasninger"
          helpTitle="Repoer med tilpasninger"
          helpText="Antall aktive repoer med minst én AI-tilpasning"
          subtitle={`av ${formatNumber(summary.active_repos)} aktive`}
        />
        <MetricCard
          value={formatNumber(summary.active_repos_with_recent_commits)}
          label="Nylig aktive repoer"
          helpTitle="Nylig aktive repoer"
          helpText="Repoer med commit siste 90 dager"
          subtitle={`${formatNumber(summary.dormant_repos)} sovende`}
        />
        <MetricCard
          value={summary.avg_customization_count?.toFixed(1) ?? "—"}
          label="Snitt tilpasninger per repo"
          helpTitle="Gjennomsnittlig antall tilpasninger"
          helpText="Gjennomsnittlig antall AI-tilpasningsfiler per aktivt repo"
          subtitle={`Maks: ${summary.max_customization_count}`}
        />
      </HGrid>

      <CustomizationTypeChart data={summary} />
      <ToolComparisonChart data={summary} />
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
          value={formatNumber(data.summary?.repos_with_any_customization ?? 0)}
          label="Tilpassede repoer"
          helpTitle="Tilpassede repoer"
          helpText="Antall unike repoer med minst én Copilot-tilpasning"
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
          value={stats.topActiveLanguage?.language ?? stats.topLanguage?.language ?? "—"}
          label="Høyest adopsjon (aktive)"
          helpTitle="Høyest adopsjon blant aktive repoer"
          helpText={
            stats.topActiveLanguage
              ? `Språket med høyest adopsjonsrate blant repoer med commit siste 90 dager (${formatAdoptionRate(stats.topActiveLanguage.adoption_rate_active_only)})`
              : "Ingen data"
          }
          subtitle={
            stats.topActiveLanguage
              ? `${formatAdoptionRate(stats.topActiveLanguage.adoption_rate_active_only)} av ${stats.topActiveLanguage.recently_active_repos} aktive repoer`
              : undefined
          }
        />
        <MetricCard
          value={formatNumber(data.summary?.repos_with_any_customization ?? 0)}
          label="Repoer med tilpasninger"
          helpTitle="Repoer med tilpasninger"
          helpText="Antall unike repoer med minst én Copilot-tilpasning"
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
  const totalActiveRepoUsages = customizationDetails.reduce((sum, d) => sum + d.active_repo_count, 0);
  const totalRepoUsages = customizationDetails.reduce((sum, d) => sum + d.repo_count, 0);

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
          helpText="Tilpasningsfila som brukes i flest navikt-repoer"
          subtitle={`${formatNumber(topFile.active_repo_count)} aktive / ${formatNumber(topFile.repo_count)} totalt`}
        />
        <MetricCard
          value={formatNumber(totalActiveRepoUsages)}
          label="Tilpasningsfiler i bruk"
          helpTitle="Tilpasningsfiler i bruk"
          helpText="Antall tilpasningsfiler i aktive repoer. Repoer med flere filer telles flere ganger."
          subtitle={`${formatNumber(totalRepoUsages)} totalt`}
        />
      </HGrid>

      <TopCustomizationsChart data={customizationDetails} />
    </VStack>
  );
}

// Sync/staleness tab content
function SyncContent({ staleness }: { staleness: StalenessSummary }) {
  const syncPercent = `${(staleness.sync_rate * 100).toFixed(0)}%`;
  const outOfSyncFiles = staleness.files
    .filter((f) => f.out_of_sync_repos > 0)
    .sort((a, b) => b.out_of_sync_repos - a.out_of_sync_repos);

  return (
    <VStack gap="space-24">
      <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-16">
        <MetricCard
          value={formatNumber(staleness.total_files)}
          label="Sporede filer"
          helpTitle="Sporede filer"
          helpText="Antall unike filer (agenter, instruksjoner, prompts, skills) som spores for synkronisering"
        />
        <MetricCard
          value={formatNumber(staleness.total_file_instances)}
          label="Fil-instanser totalt"
          helpTitle="Fil-instanser"
          helpText="Totalt antall fil-instanser på tvers av alle repoer"
        />
        <MetricCard
          value={syncPercent}
          label="Synkroniseringsrate"
          helpTitle="Synkroniseringsrate"
          helpText="Andel fil-instanser som er i synk med kanonisk kilde"
          subtitle={`${formatNumber(staleness.in_sync_count)} av ${formatNumber(staleness.total_file_instances)}`}
        />
        <MetricCard
          value={formatNumber(staleness.out_of_sync_count)}
          label="Ute av synk"
          helpTitle="Ute av synk"
          helpText="Fil-instanser som ikke matcher kanonisk kilde og kan trenge oppdatering"
        />
      </HGrid>

      {outOfSyncFiles.length > 0 && (
        <Box background="default" padding="space-20" borderRadius="8" className="border border-gray-200">
          <VStack gap="space-16">
            <Heading size="small" level="4">
              Filer ute av synk
            </Heading>
            <BodyShort size="small" className="text-gray-600">
              Filer som finnes i flere repoer men ikke matcher kanonisk kilde. Sortert etter antall repoer ute av synk.
            </BodyShort>
            <Table size="small">
              <TableHeader>
                <TableRow>
                  <TableHeaderCell>Fil</TableHeaderCell>
                  <TableHeaderCell>Kategori</TableHeaderCell>
                  <TableHeaderCell align="right">Totalt repoer</TableHeaderCell>
                  <TableHeaderCell align="right">I synk</TableHeaderCell>
                  <TableHeaderCell align="right">Ute av synk</TableHeaderCell>
                  <TableHeaderCell align="right">Synk-rate</TableHeaderCell>
                </TableRow>
              </TableHeader>
              <TableBody>
                {outOfSyncFiles.slice(0, 30).map((file) => (
                  <TableRow key={`${file.category}-${file.file_name}`}>
                    <TableDataCell className="font-mono text-sm">{file.file_name}</TableDataCell>
                    <TableDataCell>{file.category}</TableDataCell>
                    <TableDataCell align="right">{file.total_repos}</TableDataCell>
                    <TableDataCell align="right">{file.in_sync_repos}</TableDataCell>
                    <TableDataCell align="right" className="font-semibold text-red-600">
                      {file.out_of_sync_repos}
                    </TableDataCell>
                    <TableDataCell align="right">{(file.sync_rate * 100).toFixed(0)}%</TableDataCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </VStack>
        </Box>
      )}

      {outOfSyncFiles.length === 0 && (
        <Box background="neutral-soft" padding="space-20" borderRadius="8">
          <BodyShort className="text-center text-gray-600">Alle sporede filer er i synk 🎉</BodyShort>
        </Box>
      )}
    </VStack>
  );
}

// Cached data component
async function CachedAdoptionData() {
  const [{ data, error }, { data: staleness, error: stalenessError }] = await Promise.all([
    getCachedAdoptionData(),
    getCachedStalenessData(),
  ]);

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
    ...(staleness && !stalenessError
      ? [
          {
            id: "synkronisering",
            label: "Synkronisering",
            content: <SyncContent staleness={staleness} />,
          },
        ]
      : []),
  ];

  return (
    <VStack gap="space-24">
      {scanDate && <BodyShort className="text-gray-600">Siste skanning: {scanDate}</BodyShort>}
      <Tabs tabs={tabs} />
    </VStack>
  );
}

export default async function AdoptionPage() {
  await getUser();

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
