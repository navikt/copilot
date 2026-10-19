import { Suspense, cache } from "react";
import type { Metadata } from "next";
import { getNavPilotTelemetry } from "@/lib/cached-bigquery";
import { getUser, getUserToken } from "@/lib/auth";
import MetricCard from "@/components/metric-card";
import ErrorState from "@/components/error-state";
import { HGrid, Skeleton, BodyShort, Table } from "@navikt/ds-react";
import { TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@navikt/ds-react/Table";
import { InsightPage, InsightSection } from "@/components/insight-page";
import { formatNumber, formatShare } from "@/lib/format";
import type { NavPilotTelemetry } from "@/lib/types";

export const metadata: Metadata = {
  title: "nav-pilot i bruk",
  description: "Hvordan nav-pilot brukes: enheter, klienter, økter og hvilke skills agenten velger.",
};

const telemetryFor = cache(getNavPilotTelemetry);

function hasTelemetry(t: NavPilotTelemetry): boolean {
  return (
    t.devices_7d > 0 ||
    t.devices_1d > 0 ||
    t.devices_28d > 0 ||
    t.user_scope_devices_7d > 0 ||
    t.repo_scope_devices_7d > 0 ||
    t.copilot_devices_7d > 0 ||
    t.opencode_devices_7d > 0 ||
    t.copilot_sessions_7d > 0 ||
    t.opencode_sessions_7d > 0 ||
    t.agent_skill_loads_7d > 0 ||
    t.user_skill_loads_7d > 0 ||
    t.command_reach_7d.length > 0 ||
    t.top_skills_7d.length > 0 ||
    t.top_agents_7d.length > 0 ||
    t.top_tools_7d.length > 0
  );
}

async function RuntimeTelemetry({ token }: { token: string }) {
  const { telemetry: t, error } = await telemetryFor(token);
  if (error) return <BodyShort>{`Kunne ikke hente bruksdata fra nav-pilot: ${error}`}</BodyShort>;
  if (!t || !hasTelemetry(t)) return <BodyShort>Ingen bruksdata fra nav-pilot ennå.</BodyShort>;

  const totalSkillLoads = t.agent_skill_loads_7d + t.user_skill_loads_7d;
  const totalSessions = t.copilot_sessions_7d + t.opencode_sessions_7d;
  const totalTurns = t.copilot_turns_7d + t.opencode_turns_7d;
  const devices = t.devices_7d || t.devices_1d;
  const deviceSubtitles = [
    t.devices_28d > 0 ? `${formatNumber(t.devices_28d)} siste 28 dager` : null,
    t.cli_up_to_date_7d > 0 ? `${formatNumber(t.cli_up_to_date_7d)} oppdatert CLI` : null,
    t.cli_stale_7d > 0 ? `${formatNumber(t.cli_stale_7d)} eldre CLI` : null,
  ].filter(Boolean);
  const scopeSubtitles = [
    t.repo_scope_devices_7d > 0 ? `mot ${formatNumber(t.repo_scope_devices_7d)} med repo-oppsett` : null,
    t.rtk_devices_7d > 0 ? `${formatNumber(t.rtk_devices_7d)} med rtk` : null,
  ].filter(Boolean);
  const tableCount =
    (t.top_skills_7d.length > 0 ? 1 : 0) + (t.top_agents_7d.length > 0 ? 1 : 0) + (t.top_tools_7d.length > 0 ? 1 : 0);

  return (
    <>
      <BodyShort size="small" textColor="subtle">
        Siste syv dager fra enheter med nav-pilot, uten utviklerbygg og kjente CI-miljøer. Rader med færre enn fem
        enheter i et døgn skjules før tallene lagres.
      </BodyShort>
      <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-16">
        <MetricCard
          value={devices > 0 ? formatNumber(devices) : "–"}
          label={t.devices_1d > 0 && t.devices_7d === 0 ? "Enheter (siste døgn)" : "Enheter (7 dager)"}
          helpTitle="Rapporterende enheter"
          helpText="Unike enheter som har rapportert fra nav-pilot, uten utviklerbygg (version=dev) og kjente CI-miljøer. En enhet er en maskin-ID, ikke en person."
          subtitle={deviceSubtitles.length > 0 ? deviceSubtitles.join(", ") : undefined}
        />
        <MetricCard
          value={t.user_scope_devices_7d > 0 ? formatNumber(t.user_scope_devices_7d) : "–"}
          label="Personlig oppsett"
          helpTitle="Personlig oppsett mot repo-oppsett"
          helpText="Enheter med aktive tilpasninger installert i brukeroppsettet (~/.copilot) siste syv dager, sammenlignet med installasjoner i enkeltrepoer."
          subtitle={scopeSubtitles.length > 0 ? scopeSubtitles.join(", ") : undefined}
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
      {totalSessions > 0 && (
        <BodyShort size="small" textColor="subtle">
          Observerte økter siste syv dager: {formatNumber(totalSessions)} økter med {formatNumber(totalTurns)} turer
          totalt
          {t.opencode_sessions_7d > 0 && ` · OpenCode: ${formatNumber(t.opencode_sessions_7d)} økter`}
          {t.copilot_sessions_7d > 0 && ` · Copilot CLI: ${formatNumber(t.copilot_sessions_7d)} økter`}.
        </BodyShort>
      )}
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
      {tableCount > 0 && (
        <HGrid columns={{ xs: 1, lg: tableCount }} gap="space-16">
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
          {t.top_agents_7d.length > 0 && (
            <div className="overflow-x-auto">
              <Table size="small">
                <TableHeader>
                  <TableRow>
                    <TableHeaderCell>Subagent</TableHeaderCell>
                    <TableHeaderCell align="right">Valg (7 dager)</TableHeaderCell>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {t.top_agents_7d.slice(0, 10).map((item) => (
                    <TableRow key={item.agent}>
                      <TableDataCell className="font-mono">{item.agent}</TableDataCell>
                      <TableDataCell align="right">{formatNumber(item.selections)}</TableDataCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
          {t.top_tools_7d.length > 0 && (
            <div className="overflow-x-auto">
              <Table size="small">
                <TableHeader>
                  <TableRow>
                    <TableHeaderCell>Verktøy i økt</TableHeaderCell>
                    <TableHeaderCell align="right">Kall (7 dager)</TableHeaderCell>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {t.top_tools_7d.slice(0, 10).map((item) => (
                    <TableRow key={item.tool}>
                      <TableDataCell className="font-mono">{item.tool}</TableDataCell>
                      <TableDataCell align="right">{formatNumber(item.calls)}</TableDataCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </HGrid>
      )}
    </>
  );
}

export default async function NavPilotPage() {
  await getUser();
  const token = await getUserToken();
  if (!token) return <ErrorState message="Mangler innloggingstoken" />;

  return (
    <InsightPage
      title="nav-pilot i bruk"
      description="Bruksdata fra nav-pilot de siste syv dagene."
      intro="Her ser du hvor mange enheter som kjører nav-pilot, hvilke klienter de har satt opp, hvor mange økter de har, og hvilke skills, subagenter og verktøy som brukes mest."
      updated={async () => (await telemetryFor(token)).telemetry?.latest_day}
      source={
        <>
          Tallene kommer fra copilot-api (<code>/mimir/telemetry</code>). nav-pilot sender telemetri til Mimir,
          metrikklageret på Nais, og hver natt samles dagen i BigQuery-tabellen <code>mimir_daily</code>. En enhet er en
          maskin-ID, ikke en person.
        </>
      }
    >
      <InsightSection id="faktisk-bruk" title="Siste syv dager">
        <Suspense fallback={<Skeleton variant="rectangle" height={300} />}>
          <RuntimeTelemetry token={token} />
        </Suspense>
      </InsightSection>
    </InsightPage>
  );
}
