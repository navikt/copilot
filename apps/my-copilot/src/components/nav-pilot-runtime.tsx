import { cache } from "react";
import { getNavPilotTelemetry } from "@/lib/cached-bigquery";
import MetricCard from "@/components/metric-card";
import { HGrid, BodyShort, VStack } from "@navikt/ds-react";
import RankedBars from "@/components/charts/RankedBars";
import { LinkableHeading } from "@/components/linkable-heading";
import { topTakeaway } from "@/lib/ranked";
import { formatDate, formatNumber } from "@/lib/format";
import type { NavPilotTelemetry } from "@/lib/types";
import DeviceClientCoverage from "@/components/device-client-coverage";

export const telemetryFor = cache(getNavPilotTelemetry);

function hasTelemetry(t: NavPilotTelemetry): boolean {
  return (
    t.devices_7d > 0 ||
    Object.values(t.client_available_7d ?? {}).some((n) => n != null) ||
    (t.install_present_7d?.length ?? 0) > 0 ||
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
    (t.command_reach_7d?.length ?? 0) > 0 ||
    (t.top_skills_7d?.length ?? 0) > 0 ||
    (t.top_agents_7d?.length ?? 0) > 0 ||
    (t.top_tools_7d?.length ?? 0) > 0
  );
}

export async function RuntimeTelemetry({ token }: { token: string }) {
  const { telemetry: t, error } = await telemetryFor(token);
  if (error) return <BodyShort>{`Kunne ikke hente bruksdata fra nav-pilot: ${error}`}</BodyShort>;
  if (!t || !hasTelemetry(t)) return <BodyShort>Ingen bruksdata fra nav-pilot ennå.</BodyShort>;

  const totalSkillLoads = t.agent_skill_loads_7d + t.user_skill_loads_7d;
  const totalSessions = t.copilot_sessions_7d + t.opencode_sessions_7d;
  const devices = t.devices_7d || t.devices_1d;
  const deviceSubtitles = [
    t.cli_up_to_date_7d > 0 ? `${formatNumber(t.cli_up_to_date_7d)} oppdatert CLI` : null,
    t.cli_stale_7d > 0 ? `${formatNumber(t.cli_stale_7d)} eldre CLI` : null,
  ].filter(Boolean);
  const windows = [
    { label: "Siste dag", value: t.devices_1d },
    { label: "7 dager", value: t.devices_7d },
    { label: "28 dager", value: t.devices_28d },
  ];
  const byClient = (opencode: number, copilot: number) =>
    [
      { label: "OpenCode", value: opencode },
      { label: "Copilot CLI", value: copilot },
    ].sort((x, y) => y.value - x.value);
  const sessions = byClient(t.opencode_sessions_7d, t.copilot_sessions_7d);
  const turns = byClient(t.opencode_turns_7d, t.copilot_turns_7d);
  // Go sends null for an empty slice.
  const allSkills = t.top_skills_7d ?? [];
  const allAgents = t.top_agents_7d ?? [];
  const allCommands = t.command_reach_7d ?? [];
  const allTools = t.top_tools_7d ?? [];
  const skills = allSkills.slice(0, 10).map((r) => ({ label: r.skill, value: r.loads, note: r.source }));
  const agents = allAgents.slice(0, 10).map((r) => ({ label: r.agent, value: r.selections }));
  const commands = allCommands.slice(0, 10).map((r) => ({ label: r.command, value: r.devices }));
  const tools = allTools.slice(0, 10).map((r) => ({ label: r.tool, value: r.calls }));
  const num = formatNumber;

  return (
    <VStack gap="space-24">
      <BodyShort size="small" textColor="subtle">
        Siste syv dager fra enheter med nav-pilot, uten utviklerbygg og kjente CI-miljøer. Enhetstellinger under fem
        skjules før tallene lagres.
      </BodyShort>

      <HGrid columns={{ xs: 1, md: 2 }} gap="space-24" align="start">
        <MetricCard
          value={devices > 0 ? num(devices) : "–"}
          label={t.devices_1d > 0 && t.devices_7d === 0 ? "Enheter (siste døgn)" : "Enheter (7 dager)"}
          helpTitle="Rapporterende enheter"
          helpText="Unike enheter som har rapportert fra nav-pilot, uten utviklerbygg (version=dev) og kjente CI-miljøer. En enhet er en maskin-ID, ikke en person."
          subtitle={deviceSubtitles.length > 0 ? deviceSubtitles.join(", ") : undefined}
        />
        <RankedBars
          title="Rekkevidde etter vindu"
          takeaway={`${num(t.devices_28d)} enheter siste 28 dager, ${num(t.devices_7d)} siste 7 dager og ${num(t.devices_1d)} siste dag${t.latest_day ? ` (${formatDate(t.latest_day)})` : ""}. Vinduene er rullende og kan ikke summeres.`}
          summary="Enheter som har rapportert fra nav-pilot siste dag, siste 7 dager og siste 28 dager."
          items={windows}
          rows={[["Vindu", "Enheter"], ...windows.map((w) => [w.label, num(w.value)])]}
        />
      </HGrid>

      <LinkableHeading id="oppsett" size="small" level="3">
        Klienter og oppsett
      </LinkableHeading>
      <DeviceClientCoverage telemetry={t} />

      {totalSessions > 0 && (
        <>
          <LinkableHeading id="okter" size="small" level="3">
            Økter per klient
          </LinkableHeading>
          <HGrid columns={{ xs: 1, md: 2 }} gap="space-24">
            <RankedBars
              title="Økter"
              takeaway={`${num(totalSessions)} observerte økter siste 7 dager.`}
              summary="Observerte økter per klient siste syv dager."
              items={sessions}
              rows={[["Klient", "Økter, 7 dager"], ...sessions.map((r) => [r.label, num(r.value)])]}
            />
            <RankedBars
              title="Turer"
              takeaway={`${num(t.copilot_turns_7d + t.opencode_turns_7d)} turer totalt. Turer og økter er ulike mål med hver sin skala.`}
              summary="Observerte turer per klient siste syv dager."
              items={turns}
              color={1}
              rows={[["Klient", "Turer, 7 dager"], ...turns.map((r) => [r.label, num(r.value)])]}
            />
          </HGrid>
        </>
      )}

      {(skills.length > 0 || agents.length > 0) && (
        <>
          <LinkableHeading id="skills-og-subagenter" size="small" level="3">
            Skills og subagenter
          </LinkableHeading>
          <HGrid columns={{ xs: 1, md: 2 }} gap="space-24" align="start">
            <VStack gap="space-16">
              {totalSkillLoads > 0 && (
                <MetricCard
                  value={num(t.agent_skill_loads_7d)}
                  label="Skills valgt av agenten"
                  helpTitle="Automatisk mot manuell bruk av skills"
                  helpText="Skill-innlastinger der agenten selv valgte en skill ut fra instruksjonene, uten at brukeren oppga en skråstrekskommando. Innlastinger, ikke økter eller utviklere."
                  subtitle={`av ${num(totalSkillLoads)} innlastinger siste 7 dager`}
                />
              )}
              {skills.length > 0 && (
                <RankedBars
                  title="Skills etter innlastinger"
                  takeaway={`${topTakeaway(skills, "innlastinger", true)} Kilden står under navnet.`}
                  summary="De mest innlastede skillene siste syv dager, med kilde."
                  items={skills}
                  rows={[
                    ["Skill", "Kilde", "Innlastinger, 7 dager"],
                    ...allSkills.map((r) => [r.skill, r.source, num(r.loads)]),
                  ]}
                />
              )}
            </VStack>
            {agents.length > 0 && (
              <RankedBars
                title="Subagenter etter valg"
                takeaway={topTakeaway(agents, "valg", true)}
                summary="De mest valgte subagentene siste syv dager."
                items={agents}
                color={1}
                rows={[["Subagent", "Valg, 7 dager"], ...allAgents.map((r) => [r.agent, num(r.selections)])]}
              />
            )}
          </HGrid>
        </>
      )}

      {(commands.length > 0 || tools.length > 0) && (
        <>
          <LinkableHeading id="kommandoer-og-verktoy" size="small" level="3">
            Detaljer: kommandoer og verktøy
          </LinkableHeading>
          <HGrid columns={{ xs: 1, md: 2 }} gap="space-24" align="start">
            {commands.length > 0 && (
              <RankedBars
                title="Enheter per kommando"
                takeaway="Hvor mange enheter som har kjørt kommandoen. Enheter, ikke kall."
                summary="Enheter per nav-pilot-kommando siste syv dager."
                items={commands}
                color={2}
                rows={[["Kommando", "Enheter, 7 dager"], ...allCommands.map((r) => [r.command, num(r.devices)])]}
              />
            )}
            {tools.length > 0 && (
              <RankedBars
                title="Verktøykall i økt"
                takeaway={topTakeaway(tools, "kall", true)}
                summary="Verktøykall i økter siste syv dager."
                items={tools}
                color={3}
                rows={[["Verktøy i økt", "Kall, 7 dager"], ...allTools.map((r) => [r.tool, num(r.calls)])]}
              />
            )}
          </HGrid>
        </>
      )}
    </VStack>
  );
}
