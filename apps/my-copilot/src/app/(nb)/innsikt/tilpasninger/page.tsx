import { Suspense, cache, type ReactNode } from "react";
import type { Metadata } from "next";
import { getAdoptionData, getNavPilotTelemetry, getStalenessData } from "@/lib/cached-bigquery";
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
import { formatNumber, formatPercent, formatShare } from "@/lib/format";
import type { AdoptionData, NavPilotTelemetry } from "@/lib/types";
import { calculateLanguageStats } from "@/lib/adoption-utils";
import { getAllCustomizations } from "@/lib/customizations";

export const metadata: Metadata = {
  title: "Tilpasninger",
  description: "Hvor mange navikt-repoer som har KI-tilpasninger, og om de er i synk med kilden.",
};

// Every section reads the same requests; cache() makes them run once per render.
const adoptionFor = cache(getAdoptionData);
const stalenessFor = cache(getStalenessData);
const telemetryFor = cache(getNavPilotTelemetry);

const fallback = <Skeleton variant="rectangle" height={300} />;

// Two cards side by side from sm, so a section never leaves an empty grid cell.
const cards = (children: ReactNode) => (
  <HGrid columns={{ xs: 1, sm: 2 }} gap="space-16">
    {children}
  </HGrid>
);

async function Adoption({ token, children }: { token: string; children: (data: AdoptionData) => ReactNode }) {
  const { data, error } = await adoptionFor(token);
  if (error) return <BodyShort>{`Kunne ikke hente tilpasningene: ${error}`}</BodyShort>;
  if (!data) return <BodyShort>Ingen data om tilpasninger ennå.</BodyShort>;
  return children(data);
}

function hasTelemetry(t: NavPilotTelemetry): boolean {
  return (
    t.devices_7d > 0 ||
    t.devices_1d > 0 ||
    t.user_scope_devices_7d > 0 ||
    t.copilot_devices_7d > 0 ||
    t.opencode_devices_7d > 0 ||
    t.agent_skill_loads_7d > 0 ||
    t.user_skill_loads_7d > 0
  );
}

async function RuntimeTelemetry({ token }: { token: string }) {
  const { telemetry: t, error } = await telemetryFor(token);
  if (error) return <BodyShort>{`Kunne ikke hente bruksdata fra nav-pilot: ${error}`}</BodyShort>;
  if (!t || !hasTelemetry(t)) return <BodyShort>Ingen bruksdata fra nav-pilot ennå.</BodyShort>;

  const totalSkillLoads = t.agent_skill_loads_7d + t.user_skill_loads_7d;
  const devices = t.devices_7d || t.devices_1d;

  return (
    <>
      <BodyShort size="small" textColor="subtle">
        Siste syv dager fra enheter med nav-pilot, uten utviklerbygg og kjente CI-miljøer. Rader med færre enn fem
        enheter i et døgn skjules før tallene lagres.
      </BodyShort>
      <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-16">
        <MetricCard
          value={devices > 0 ? formatNumber(devices) : "–"}
          label={t.devices_7d > 0 ? "Enheter (7 dager)" : "Enheter (siste døgn)"}
          helpTitle="Rapporterende enheter"
          helpText="Unike enheter som har rapportert fra nav-pilot, uten utviklerbygg (version=dev) og kjente CI-miljøer. En enhet er en maskin-ID, ikke en person."
          subtitle={t.devices_28d > 0 ? `${formatNumber(t.devices_28d)} siste 28 dager` : undefined}
        />
        <MetricCard
          value={t.user_scope_devices_7d > 0 ? formatNumber(t.user_scope_devices_7d) : "–"}
          label="Personlig oppsett"
          helpTitle="Personlig oppsett mot repo-oppsett"
          helpText="Enheter med aktive tilpasninger installert i brukeroppsettet (~/.copilot) siste syv dager, sammenlignet med installasjoner i enkeltrepoer."
          subtitle={
            t.repo_scope_devices_7d > 0 ? `mot ${formatNumber(t.repo_scope_devices_7d)} med repo-oppsett` : undefined
          }
        />
        <MetricCard
          value={t.opencode_devices_7d > 0 ? formatNumber(t.opencode_devices_7d) : "–"}
          label="Enheter med OpenCode"
          helpTitle="Klientoppsett i nav-pilot"
          helpText="Enheter som har rapportert konfigurasjon for OpenCode og GitHub Copilot."
          subtitle={
            t.copilot_devices_7d > 0
              ? `${formatNumber(t.copilot_devices_7d)} med Copilot${t.both_clients_7d > 0 ? `, ${formatNumber(t.both_clients_7d)} med begge` : ""}`
              : undefined
          }
        />
        <MetricCard
          value={totalSkillLoads > 0 ? formatShare(t.agent_skill_loads_7d / totalSkillLoads) : "–"}
          label="Skills valgt av agenten"
          helpTitle="Automatisk mot manuell bruk av skills"
          helpText="Andelen skill-innlastinger der agenten selv valgte en skill ut fra instruksjonene, uten at brukeren oppga en skråstrekskommando."
          subtitle={
            totalSkillLoads > 0
              ? `${formatNumber(t.agent_skill_loads_7d)} av ${formatNumber(totalSkillLoads)} innlastinger`
              : undefined
          }
        />
      </HGrid>
      {t.command_reach_7d.length > 0 && (
        <BodyShort size="small" textColor="subtle">
          Enheter per kommando siste syv dager:{" "}
          {t.command_reach_7d.map((item, idx) => (
            <span key={item.command}>
              {idx > 0 ? ", " : ""}
              <code>{item.command}</code> ({formatNumber(item.devices)})
            </span>
          ))}
          .
        </BodyShort>
      )}
      {t.top_skills_7d.length > 0 && (
        <div className="overflow-x-auto">
          <Table size="small">
            <TableHeader>
              <TableRow>
                <TableHeaderCell>Skill</TableHeaderCell>
                <TableHeaderCell>Kilde</TableHeaderCell>
                <TableHeaderCell align="right">Innlastinger (7 dager)</TableHeaderCell>
              </TableRow>
            </TableHeader>
            <TableBody>
              {t.top_skills_7d.slice(0, 10).map((item) => (
                <TableRow key={`${item.source}-${item.skill}`}>
                  <TableDataCell className="font-mono">{item.skill}</TableDataCell>
                  <TableDataCell>{item.source}</TableDataCell>
                  <TableDataCell align="right">{formatNumber(item.loads)}</TableDataCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </>
  );
}

function Overview({ data }: { data: AdoptionData }) {
  const { summary } = data;
  if (!summary) return <BodyShort>Ingen data om tilpasninger ennå.</BodyShort>;
  return (
    <>
      <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-16">
        <MetricCard
          value={formatShare(summary.adoption_rate_active_only, 1)}
          label="Aktive repoer med tilpasninger"
          helpTitle="Andel aktive repoer med tilpasninger"
          helpText="Andelen repoer med commit de siste 90 dagene som har minst én KI-tilpasning."
          subtitle={`av ${formatNumber(summary.active_repos_with_recent_commits)} aktive repoer`}
        />
        <MetricCard
          value={formatShare(summary.adoption_rate, 1)}
          label="Alle repoer med tilpasninger"
          helpTitle="Andel av alle repoer med tilpasninger"
          helpText="Andelen av alle repoer som ikke er arkivert, som har minst én KI-tilpasning. Også repoer uten commit de siste 90 dagene er med."
          subtitle={`${formatNumber(summary.repos_with_any_customization)} av ${formatNumber(summary.active_repos)} repoer`}
        />
        <MetricCard
          value={formatNumber(summary.repos_with_copilot_instructions)}
          label="copilot-instructions.md"
          helpTitle="Copilot-instruksjoner"
          helpText="Repoer som har .github/copilot-instructions.md."
          subtitle={`av ${formatNumber(summary.active_repos)} repoer`}
        />
        <MetricCard
          value={formatNumber(summary.repos_with_agents_md)}
          label="AGENTS.md"
          helpTitle="AGENTS.md"
          helpText="Repoer som har AGENTS.md. Copilot og andre KI-verktøy leser filen."
          subtitle={`av ${formatNumber(summary.active_repos)} repoer`}
        />
      </HGrid>
      <CustomizationTypeChart data={summary} />
    </>
  );
}

// The shared catalog in navikt/copilot, the number the front page shows.
function Catalog() {
  const items = getAllCustomizations();
  const types = [
    { label: "Agenter", type: "agent" },
    { label: "Skills", type: "skill" },
    { label: "Instruksjoner", type: "instruction" },
    { label: "Prompts", type: "prompt" },
  ].map((t) => ({ ...t, count: items.filter((i) => i.type === t.type).length }));
  return (
    <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-16">
      {types.map((t) => (
        <MetricCard
          key={t.type}
          value={formatNumber(t.count)}
          label={t.label}
          helpTitle={t.label}
          helpText="Antall i den delte samlingen i navikt/copilot, som du finner under Verktøy."
        />
      ))}
    </HGrid>
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
      {cards(
        <>
          <MetricCard
            value={top.file_name}
            label="Mest brukte"
            helpTitle="Mest brukte tilpasning"
            helpText="Tilpasningsfilen som finnes i flest navikt-repoer."
            subtitle={`i ${formatNumber(top.repo_count)} repoer, ${formatNumber(top.active_repo_count)} av dem aktive`}
          />
          <MetricCard
            value={formatNumber(details.length)}
            label="Ulike tilpasningsfiler"
            helpTitle="Ulike tilpasningsfiler"
            helpText="Antall ulike agenter, skills, instruksjoner, prompts og arbeidsflyter som finnes i minst ett repo."
          />
        </>
      )}
      <TopCustomizationsChart data={details} />
    </>
  );
}

function Teams({ data }: { data: AdoptionData }) {
  const { teams } = data;
  if (!teams?.teams.length) return <BodyShort>Ingen teamdata ennå.</BodyShort>;
  return (
    <>
      {cards(
        <>
          <MetricCard
            value={formatNumber(teams.teams_with_adoption)}
            label="Team med tilpasninger"
            helpTitle="Team med tilpasninger"
            helpText="Team som har minst ett repo med KI-tilpasninger. Bare team med minst fem repoer som ikke er arkivert, telles."
            subtitle={`${formatPercent(teams.adoption_pct)} av ${formatNumber(teams.total_teams)} team`}
          />
          <MetricCard
            value={formatNumber(teams.small_teams)}
            label="Små team som ikke vises"
            helpTitle="Små team"
            helpText="Team med færre enn fem repoer som ikke er arkivert. De er ikke med i tallene eller tabellen."
          />
        </>
      )}
      <TeamAdoptionChart data={teams.teams} maxTeams={10} />
      <TeamTable teams={teams.teams} />
    </>
  );
}

function Languages({ data }: { data: AdoptionData }) {
  const { languages } = data;
  if (!languages?.length) return <BodyShort>Ingen språkdata ennå.</BodyShort>;
  // A language with two active repos would top the list at 100 %, so the card needs at least five.
  const { topActiveLanguage: top } = calculateLanguageStats(languages.filter((l) => l.recently_active_repos >= 5));
  const withCustomizations = languages.filter((l) => l.repos_with_customizations > 0).length;
  return (
    <>
      {cards(
        <>
          <MetricCard
            value={top?.language ?? "–"}
            label="Høyest andel blant aktive repoer"
            helpTitle="Språket med høyest andel"
            helpText="Språket der flest av de aktive repoene har KI-tilpasninger. Bare språk med minst fem aktive repoer er med."
            subtitle={
              top
                ? `${formatShare(top.adoption_rate_active_only)} av ${formatNumber(top.recently_active_repos)} aktive repoer`
                : undefined
            }
          />
          <MetricCard
            value={formatNumber(withCustomizations)}
            label="Språk med tilpasninger"
            helpTitle="Språk med tilpasninger"
            helpText="Programmeringsspråk der minst ett repo har KI-tilpasninger. Bare språk med minst fem repoer er med."
            subtitle={`av ${formatNumber(languages.length)} språk`}
          />
        </>
      )}
      <LanguageAdoptionChart data={languages} maxLanguages={15} />
    </>
  );
}

async function Sync({ token }: { token: string }) {
  const { data: staleness, error } = await stalenessFor(token);
  if (error) return <BodyShort>{`Kunne ikke hente synkroniseringen: ${error}`}</BodyShort>;
  if (!staleness) return <BodyShort>Ingen data om synkronisering ennå.</BodyShort>;
  const outOfSync = staleness.files
    .filter((f) => f.out_of_sync_repos > 0)
    .sort((a, b) => b.out_of_sync_repos - a.out_of_sync_repos);
  return (
    <>
      {cards(
        <>
          <MetricCard
            value={formatShare(staleness.sync_rate)}
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
        </>
      )}
      {outOfSync.length === 0 ? (
        <BodyShort>Alle filene som følges, er i synk.</BodyShort>
      ) : (
        <>
          <BodyShort size="small" textColor="subtle">
            Filer som finnes i flere repoer, men ikke er like kilden. De med flest repoer ute av synk står først.
          </BodyShort>
          <div className="overflow-x-auto">
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
                    <TableDataCell className="font-mono">{file.file_name}</TableDataCell>
                    <TableDataCell>{file.category}</TableDataCell>
                    <TableDataCell align="right">{formatNumber(file.total_repos)}</TableDataCell>
                    <TableDataCell align="right">{formatNumber(file.in_sync_repos)}</TableDataCell>
                    <TableDataCell align="right">{formatNumber(file.out_of_sync_repos)}</TableDataCell>
                    <TableDataCell align="right">{formatShare(file.sync_rate)}</TableDataCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </>
      )}
    </>
  );
}

export default async function TilpasningerPage() {
  await getUser();
  const token = await getUserToken();
  if (!token) return <ErrorState message="Mangler innloggingstoken" />;

  return (
    <InsightPage
      title="Tilpasninger"
      description="KI-tilpasninger i navikt-repoene, fra en ukentlig skanning."
      intro="Her ser du hvor mange av navikt-repoene som har KI-tilpasninger som instruksjoner, agenter, skills og prompts, hvilke som brukes mest, og om kopiene er i synk med kilden."
      updated={async () => (await adoptionFor(token)).data?.summary?.scan_date}
      source={
        <>
          Tallene kommer fra copilot-api, som leser en ukentlig skanning av navikt-repoene og daglig aggregerte
          bruksdata fra Mimir i BigQuery: <code>/adoption/summary</code>, <code>/adoption/teams</code>,{" "}
          <code>/adoption/languages</code>, <code>/customizations/details</code>, <code>/adoption/staleness</code> og{" "}
          <code>/mimir/telemetry</code>. «Sist oppdatert» er datoen for skanningen. Arkiverte repoer er ikke med. «Alle
          repoer» er resten, og et repo er aktivt når det har fått en commit de siste 90 dagene. En KI-tilpasning er en
          fil for Copilot eller et annet KI-verktøy, for eksempel CLAUDE.md eller .cursorrules. Tallene for repoene
          gjelder siste skanning, ikke en periode.
        </>
      }
    >
      <InsightSection id="oversikt" title="Oversikt">
        <Suspense fallback={fallback}>
          <Adoption token={token}>{(data) => <Overview data={data} />}</Adoption>
        </Suspense>
      </InsightSection>
      <InsightSection id="samlingen" title={`${getAllCustomizations().length} delte tilpasninger`}>
        <Catalog />
      </InsightSection>
      <InsightSection id="faktisk-bruk" title="Faktisk bruk i nav-pilot">
        <Suspense fallback={fallback}>
          <RuntimeTelemetry token={token} />
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
