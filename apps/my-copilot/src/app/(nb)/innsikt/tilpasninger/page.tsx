import { Suspense, cache, type ReactNode } from "react";
import type { Metadata } from "next";
import { getAdoptionData, getStalenessData } from "@/lib/cached-bigquery";
import { getUser, getUserToken } from "@/lib/auth";
import {
  CustomizationTypeChart,
  TeamAdoptionChart,
  LanguageAdoptionChart,
  TopCustomizationsChart,
  ToolComparisonChart,
} from "@/components/charts/adoption";
import MetricCard from "@/components/metric-card";
import ErrorState from "@/components/error-state";
import { HGrid, Skeleton, BodyShort, Table } from "@navikt/ds-react";
import { TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@navikt/ds-react/Table";
import { InsightPage, InsightSection } from "@/components/insight-page";
import TeamTable from "@/components/team-table";
import { formatNumber } from "@/lib/format";
import type { AdoptionData } from "@/lib/types";
import { calculateTeamStats, calculateLanguageStats, formatAdoptionRate, formatScanDate } from "@/lib/adoption-utils";

export const metadata: Metadata = {
  title: "Tilpasninger",
  description: "Hvor mange navikt-repoer som har KI-tilpasninger, og om de er i synk med kilden.",
};

// Every section reads the same two requests; cache() makes them run once per render.
const adoptionFor = cache(getAdoptionData);
const stalenessFor = cache(getStalenessData);

const fallback = <Skeleton variant="rectangle" height={300} />;

async function Adoption({ token, children }: { token: string; children: (data: AdoptionData) => ReactNode }) {
  const { data, error } = await adoptionFor(token);
  if (error) return <BodyShort>{`Kunne ikke hente adopsjonsdata: ${error}`}</BodyShort>;
  if (!data) return <BodyShort>Ingen adopsjonsdata ennå.</BodyShort>;
  return children(data);
}

function Overview({ data }: { data: AdoptionData }) {
  const { summary } = data;
  if (!summary) return <BodyShort>Ingen adopsjonsdata ennå.</BodyShort>;
  return (
    <>
      <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-16">
        <MetricCard
          value={formatAdoptionRate(summary.adoption_rate_active_only, 1)}
          label="Adopsjonsrate (aktive repoer)"
          helpTitle="Adopsjonsrate for aktive repoer"
          helpText="Andelen repoer med commit siste 90 dager som har minst én Copilot-tilpasning."
          subtitle={`${formatAdoptionRate(summary.adoption_rate, 1)} med inaktive repoer`}
        />
        <MetricCard
          value={formatNumber(summary.repos_with_any_customization)}
          label="Repoer med tilpasninger"
          helpTitle="Repoer med tilpasninger"
          helpText="Aktive repoer med minst én Copilot-tilpasning."
          subtitle={`av ${formatNumber(summary.active_repos)} aktive`}
        />
        <MetricCard
          value={formatNumber(summary.repos_with_copilot_instructions)}
          label="copilot-instructions.md"
          helpTitle="Copilot-instruksjoner"
          helpText="Repoer med .github/copilot-instructions.md."
        />
      </HGrid>
      <CustomizationTypeChart data={summary} />
    </>
  );
}

function Tools({ data }: { data: AdoptionData }) {
  return data.summary ? <ToolComparisonChart data={data.summary} /> : <BodyShort>Ingen data ennå.</BodyShort>;
}

function TopCustomizations({ data }: { data: AdoptionData }) {
  const details = data.customizationDetails;
  if (!details?.length) return <BodyShort>Ingen data om tilpasninger ennå.</BodyShort>;
  const top = details[0];
  return (
    <>
      <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-16">
        <MetricCard
          value={top.file_name}
          label="Mest brukte"
          helpTitle="Mest brukte tilpasning"
          helpText="Tilpasningsfilen som finnes i flest navikt-repoer."
          subtitle={`${formatNumber(top.active_repo_count)} aktive / ${formatNumber(top.repo_count)} totalt`}
        />
      </HGrid>
      <TopCustomizationsChart data={details} />
    </>
  );
}

function Teams({ data }: { data: AdoptionData }) {
  const { teams } = data;
  if (!teams?.length) return <BodyShort>Ingen teamdata ennå.</BodyShort>;
  const stats = calculateTeamStats(teams);
  return (
    <>
      <HGrid columns={{ xs: 1, sm: 2 }} gap="space-16">
        <MetricCard
          value={formatNumber(stats.teamsWithAdoption)}
          label="Team med tilpasninger"
          helpTitle="Team med tilpasninger"
          helpText="Team som har minst ett repo med Copilot-tilpasninger."
          subtitle={`${stats.adoptionPercent.toFixed(0)} % av ${formatNumber(stats.totalTeams)} team`}
        />
      </HGrid>
      <TeamAdoptionChart data={teams} maxTeams={10} />
      <TeamTable teams={teams.filter((t) => t.active_repos > 0)} />
    </>
  );
}

function Languages({ data }: { data: AdoptionData }) {
  const { languages } = data;
  if (!languages?.length) return <BodyShort>Ingen språkdata ennå.</BodyShort>;
  const { topActiveLanguage: top, topLanguage } = calculateLanguageStats(languages);
  return (
    <>
      <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-16">
        <MetricCard
          value={top?.language ?? topLanguage?.language ?? "–"}
          label="Høyest adopsjon (aktive)"
          helpTitle="Høyest adopsjon blant aktive repoer"
          helpText="Språket med høyest adopsjonsrate blant repoer med commit siste 90 dager. Bare språk med minst fem repoer er med."
          subtitle={
            top
              ? `${formatAdoptionRate(top.adoption_rate_active_only)} av ${formatNumber(top.recently_active_repos)} aktive repoer`
              : undefined
          }
        />
      </HGrid>
      <LanguageAdoptionChart data={languages} maxLanguages={15} />
    </>
  );
}

async function Sync({ token }: { token: string }) {
  const { data: staleness, error } = await stalenessFor(token);
  if (error) return <BodyShort>{`Kunne ikke hente synkroniseringsdata: ${error}`}</BodyShort>;
  if (!staleness) return <BodyShort>Ingen synkroniseringsdata ennå.</BodyShort>;
  const outOfSync = staleness.files
    .filter((f) => f.out_of_sync_repos > 0)
    .sort((a, b) => b.out_of_sync_repos - a.out_of_sync_repos);
  return (
    <>
      <HGrid columns={{ xs: 1, sm: 2 }} gap="space-16">
        <MetricCard
          value={`${(staleness.sync_rate * 100).toFixed(0)} %`}
          label="I synk"
          helpTitle="Andel i synk"
          helpText="Andelen tilpasningsfiler i repoene som er like kilden de er kopiert fra."
          subtitle={`${formatNumber(staleness.in_sync_count)} av ${formatNumber(staleness.total_file_instances)} filer`}
        />
        <MetricCard
          value={formatNumber(staleness.out_of_sync_count)}
          label="Ute av synk"
          helpTitle="Ute av synk"
          helpText="Tilpasningsfiler i repoene som ikke er like kilden, og som kan trenge oppdatering."
        />
      </HGrid>
      {outOfSync.length === 0 ? (
        <BodyShort>Alle filene som følges, er i synk.</BodyShort>
      ) : (
        <>
          <BodyShort size="small" textColor="subtle">
            Filer som finnes i flere repoer, men ikke er like kilden. De med flest repoer ute av synk står først.
          </BodyShort>
          <Table size="small">
            <TableHeader>
              <TableRow>
                <TableHeaderCell>Fil</TableHeaderCell>
                <TableHeaderCell>Kategori</TableHeaderCell>
                <TableHeaderCell align="right">Repoer</TableHeaderCell>
                <TableHeaderCell align="right">I synk</TableHeaderCell>
                <TableHeaderCell align="right">Ute av synk</TableHeaderCell>
                <TableHeaderCell align="right">Andel i synk</TableHeaderCell>
              </TableRow>
            </TableHeader>
            <TableBody>
              {outOfSync.slice(0, 30).map((file) => (
                <TableRow key={`${file.category}-${file.file_name}`}>
                  <TableDataCell className="font-mono text-sm">{file.file_name}</TableDataCell>
                  <TableDataCell>{file.category}</TableDataCell>
                  <TableDataCell align="right">{formatNumber(file.total_repos)}</TableDataCell>
                  <TableDataCell align="right">{formatNumber(file.in_sync_repos)}</TableDataCell>
                  <TableDataCell align="right" className="font-semibold">
                    {formatNumber(file.out_of_sync_repos)}
                  </TableDataCell>
                  <TableDataCell align="right">{(file.sync_rate * 100).toFixed(0)} %</TableDataCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </>
      )}
    </>
  );
}

async function ScanDate({ token }: { token: string }) {
  const { data } = await adoptionFor(token);
  return data?.summary ? formatScanDate(data.summary.scan_date) : "kunne ikke hentes";
}

export default async function TilpasningerPage() {
  await getUser();
  const token = await getUserToken();
  if (!token) return <ErrorState message="Mangler innloggingstoken" />;

  return (
    <InsightPage
      title="Tilpasninger"
      description="KI-tilpasninger i navikt-repoene, fra en ukentlig skanning."
      intro="Her ser du hvor mange av navikt-repoene som har Copilot-tilpasninger som instruksjoner, agenter, skills og prompts, hvilke som brukes mest, og om kopiene er i synk med kilden."
      updated={
        <Suspense fallback="henter …">
          <ScanDate token={token} />
        </Suspense>
      }
      source={
        <>
          Tallene kommer fra copilot-api, som leser en ukentlig skanning av navikt-repoene fra BigQuery:{" "}
          <code>/adoption/summary</code>, <code>/adoption/teams</code>, <code>/adoption/languages</code>,{" "}
          <code>/customizations/details</code> og <code>/adoption/staleness</code>. Et repo er aktivt når det har fått
          en commit de siste 90 dagene. Tallene gjelder siste skanning, ikke en periode.
        </>
      }
    >
      <InsightSection id="oversikt" title="Oversikt">
        <Suspense fallback={fallback}>
          <Adoption token={token}>{(data) => <Overview data={data} />}</Adoption>
        </Suspense>
      </InsightSection>
      <InsightSection id="verktoy" title="Tilpasninger per verktøy">
        <Suspense fallback={fallback}>
          <Adoption token={token}>{(data) => <Tools data={data} />}</Adoption>
        </Suspense>
      </InsightSection>
      <InsightSection id="tilpasninger" title="Mest brukte tilpasninger">
        <Suspense fallback={fallback}>
          <Adoption token={token}>{(data) => <TopCustomizations data={data} />}</Adoption>
        </Suspense>
      </InsightSection>
      <InsightSection id="team" title="Team">
        <Suspense fallback={fallback}>
          <Adoption token={token}>{(data) => <Teams data={data} />}</Adoption>
        </Suspense>
      </InsightSection>
      <InsightSection id="sprak" title="Språk">
        <Suspense fallback={fallback}>
          <Adoption token={token}>{(data) => <Languages data={data} />}</Adoption>
        </Suspense>
      </InsightSection>
      <InsightSection id="synkronisering" title="Synkronisering">
        <Suspense fallback={fallback}>
          <Sync token={token} />
        </Suspense>
      </InsightSection>
    </InsightPage>
  );
}
