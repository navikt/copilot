import type { Metadata } from "next";
import { cache } from "react";
import { Alert, BodyShort, HGrid, VStack } from "@navikt/ds-react";
import { InsightPage, InsightSection } from "@/components/insight-page";
import MetricCard from "@/components/metric-card";
import { PlainTable } from "@/components/charts/chart-figure";
import ErrorState from "@/components/error-state";
import { getUserToken, requireAdmin } from "@/lib/auth";
import {
  getAdminMonthlyOverview,
  getAdminVolume,
  getAdoptionData,
  getBillingModelBreakdown,
  getCopilotPRsMonthly,
  getCopilotUsageMetrics,
  getRepositoryUsage,
  getTeamGrossOverview,
  getTeamNetOverview,
} from "@/lib/cached-bigquery";
import { formatMinutes, formatNumber, formatPercent, formatUSD } from "@/lib/format";
import { teamInsightMonth } from "@/lib/month-utils";
import { monthName } from "@/lib/team-year";
import { buildVerdiMonths, buildVerdiRepos, buildVerdiTeams } from "@/lib/verdi";

export const metadata: Metadata = {
  title: "Verdi og effekt",
  description: "Kostnad per resultat, kodeagent, kodegjennomgang, kontekstforbruk og fordeling per team og repo.",
  robots: { index: false, follow: false },
};

const loadVerdiData = cache(async (token: string) => {
  const year = new Date().getUTCFullYear();
  const teamMonth = teamInsightMonth(undefined);
  const [admin, breakdownRes, usageRes, copilotPRsRes, volume, reposRes, teamGross, teamNet, adoptionRes] =
    await Promise.all([
      getAdminMonthlyOverview(year, token).catch(() => null),
      getBillingModelBreakdown(token, 12).catch(() => ({ breakdown: [], error: "feil" })),
      getCopilotUsageMetrics(token).catch(() => ({ usage: null, error: "feil" })),
      getCopilotPRsMonthly(token).catch(() => ({ months: [], error: "feil" })),
      getAdminVolume(year, token).catch(() => []),
      getRepositoryUsage(token).catch(() => ({ repositories: [], error: "feil" })),
      getTeamGrossOverview(teamMonth, token).catch(() => null),
      getTeamNetOverview(teamMonth, token).catch(() => null),
      getAdoptionData(token).catch(() => ({ data: null, error: "feil" })),
    ]);

  const months = buildVerdiMonths({
    admin,
    breakdown: breakdownRes.error ? null : breakdownRes.breakdown,
    usage: usageRes.usage,
    copilotPRs: copilotPRsRes.error ? null : copilotPRsRes.months,
    volume,
  });
  const repos = buildVerdiRepos(reposRes.repositories);
  const teams = buildVerdiTeams(teamNet, teamGross, adoptionRes.data?.teams);
  const updated = usageRes.usage?.at(-1)?.day ?? admin?.coverage.last_gross_day ?? teamGross?.last_usage_day ?? null;

  return { months, repos, teams, updated };
});

const dash = "–";
const fmtUsd = (v: number | null) => (v !== null ? formatUSD(v) : dash);
const fmtPct = (v: number | null) => (v !== null ? formatPercent(v, 1) : dash);
const fmtRatio = (v: number | null) => (v !== null ? `${formatNumber(v)}:1` : dash);
const mLabel = (m: string, preliminary: boolean) => `${monthName(m)}${preliminary ? " (foreløpig)" : ""}`;

export default async function AdminVerdiPage() {
  await requireAdmin();
  const token = await getUserToken();
  if (!token) return <ErrorState title="Verdi og effekt" message="Mangler innloggingstoken" />;

  const { months, repos, teams, updated } = await loadVerdiData(token);
  const prMonths = months.filter((m) => m.prTotalCreated > 0 || m.prReviewedByCopilot > 0);
  const latestPR = prMonths.at(-1) ?? null;
  const tokenMonths = months.filter((m) => m.cliInputTokens > 0 || m.appInputTokens > 0);
  const latestToken = tokenMonths.at(-1) ?? null;

  return (
    <InsightPage
      title="Verdi og effekt"
      description="Kostnad per resultat, kodeagent, kodegjennomgang, kontekstforhold og fordeling per team og repo."
      intro="Kostnadssiden viser hva Copilot koster. Her ser du hva vi får igjen i pull requests, kodegjennomgang og tokenforbruk, basert på tallene vi allerede samler inn. Grupper med færre enn fem aktive brukere eller færre enn fem pull requests vises ikke enkeltvis."
      updated={async () => updated}
      source={
        <>
          Kostnadstallene kommer fra <code>/admin/billing/monthly</code>, <code>/admin/billing/volume</code> og{" "}
          <code>/billing/model-breakdown</code> (netto USD etter rabatt, uten lisenser). Pull request-tallene kommer fra{" "}
          <code>/usage/metrics</code> (GitHubs <code>pull_requests</code>-felt fra 17. juli 2026),{" "}
          <code>/usage/copilot-prs</code> og <code>/usage/repositories</code> (<code>v_repository_usage</code>,
          offentlige og interne repoer med minst fem PR-er). Tokentallene kommer fra <code>/admin/billing/volume</code>.
          Teamkostnad og tilpasninger kommer fra <code>/usage/team-net</code>, <code>/usage/team-gross</code> og{" "}
          <code>/adoption/teams</code> med femgrense.
        </>
      }
    >
      <InsightSection id="kostnad-per-resultat" title="Kostnad per resultat">
        <BodyShort size="small" textColor="subtle">
          Fakturadata skiller ut varelinjene for kodeagenten (<code>Copilot Cloud Agent</code>) og automatisk
          kodegjennomgang (<code>code-review-model</code>), mens IDE-chat og CLI deler de øvrige modell-SKU-ene. Derfor
          vises direkte kostnad for kodeagent og kodegjennomgang ved siden av total netto kostnad delt på alle mergede
          pull requests.
        </BodyShort>
        {latestPR ? (
          <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-16">
            <MetricCard
              value={fmtUsd(latestPR.usdPerMergedPR)}
              label="USD per merget PR (total)"
              subtitle={mLabel(latestPR.month, latestPR.preliminary)}
              helpTitle="Total kostnad per merget PR"
              helpText="Månedens totale netto kostnad delt på alle mergede pull requests i måneden."
            />
            <MetricCard
              value={fmtUsd(latestPR.usdPerAgentMergedPRDirect ?? latestPR.usdPerAgentMergedPR)}
              label={
                latestPR.usdPerAgentMergedPRDirect !== null
                  ? "USD per merget agent-PR (SKU)"
                  : "USD per merget agent-PR (grovt)"
              }
              subtitle={mLabel(latestPR.month, latestPR.preliminary)}
              helpTitle="Kostnad per merget kodeagent-PR"
              helpText="Månedens netto kostnad for Copilot Cloud Agent delt på pull requests opprettet av kodeagenten som ble merget."
            />
            <MetricCard
              value={fmtUsd(latestPR.usdPerReviewDirect)}
              label="USD per kodegjennomgang (modell)"
              subtitle={mLabel(latestPR.month, latestPR.preliminary)}
              helpTitle="Direkte modellkostnad per kodegjennomgang"
              helpText="Månedens netto kostnad for code-review-model delt på antall pull requests gjennomgått av Copilot."
            />
            <MetricCard
              value={fmtUsd(latestPR.usdPerAppliedSuggestion)}
              label="USD per forslag tatt i bruk (modell)"
              subtitle={mLabel(latestPR.month, latestPR.preliminary)}
              helpTitle="Direkte modellkostnad per forslag tatt i bruk"
              helpText="Månedens netto kostnad for code-review-model delt på antall endringsforslag fra Copilot som ble tatt i bruk."
            />
          </HGrid>
        ) : (
          <BodyShort>Ingen månedlige PR- og kostnadstall ennå.</BodyShort>
        )}
        {prMonths.length > 0 && (
          <PlainTable
            rows={[
              [
                "Måned",
                "Netto kostnad",
                "Mergede PR-er",
                "USD / PR (total)",
                "USD / agent-PR (SKU)",
                "USD / gjennomgang (modell)",
                "USD / forslag tatt i bruk (modell)",
                "USD / agent-PR (grovt)",
              ],
              ...prMonths.map((m) => [
                mLabel(m.month, m.preliminary),
                fmtUsd(m.netUsd),
                formatNumber(m.prTotalMerged),
                fmtUsd(m.usdPerMergedPR),
                fmtUsd(m.usdPerAgentMergedPRDirect),
                fmtUsd(m.usdPerReviewDirect),
                fmtUsd(m.usdPerAppliedSuggestion),
                fmtUsd(m.usdPerAgentMergedPR),
              ]),
            ]}
          />
        )}
      </InsightSection>

      <InsightSection id="kodeagent" title="Kodeagentens effekt">
        <BodyShort size="small" textColor="subtle">
          Hvor mange pull requests Copilot coding agent oppretter, forholdet mellom mergede og opprettede PR-er i
          måneden, median tid til merge og direkte SKU-kostnad per merget agent-PR. Siden PR-er kan opprettes i én måned
          og merges i en senere måned, kan forholdstallet overstige 100 %. Kjøringer som avbrytes uten at det opprettes
          en PR, koster fortsatt kreditter, så kostnad per merget PR fanger opp både vellykkede og avbrutte kjøringer.
        </BodyShort>
        {latestPR ? (
          <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-16">
            <MetricCard
              value={formatNumber(latestPR.prCreatedByCopilot)}
              label="PR-er opprettet av agent"
              subtitle={mLabel(latestPR.month, latestPR.preliminary)}
              helpTitle="PR-er opprettet av agent"
              helpText="Pull requests som Copilot coding agent har opprettet i løpet av måneden."
            />
            <MetricCard
              value={fmtPct(latestPR.agentMergeSharePct)}
              label="Mergede mot opprettede"
              subtitle={`${formatNumber(latestPR.prMergedCopilotAuthored)} merget i måneden`}
              helpTitle="Mergede mot opprettede (flytrate)"
              helpText="Agent-PR-er merget i måneden delt på agent-PR-er opprettet i samme måned. Kan overstige 100 % når PR-er fra tidligere måneder merges."
            />
            <MetricCard
              value={formatMinutes(latestPR.avgMedianMinutesToMergeCopilot)}
              label="Tid til merge (agent)"
              subtitle={`Alle PR-er: ${formatMinutes(latestPR.avgMedianMinutesToMerge)}`}
              helpTitle="Tid til merge (agent)"
              helpText="PR-vektet gjennomsnitt av daglig mediantid fra opprettelse til merge for agent-PR-er."
            />
            <MetricCard
              value={fmtUsd(latestPR.usdPerAgentMergedPRDirect ?? latestPR.usdPerAgentMergedPR)}
              label={
                latestPR.usdPerAgentMergedPRDirect !== null
                  ? "USD per merget agent-PR (SKU)"
                  : "USD per merget agent-PR (grovt)"
              }
              subtitle={
                latestPR.cloudAgentNetUsd !== null
                  ? `SKU-kostnad: ${fmtUsd(latestPR.cloudAgentNetUsd)}`
                  : "Hele månedskostnaden delt på agent-PR"
              }
              helpTitle="USD per merget agent-PR"
              helpText="Månedens nettobeløp for Copilot Cloud Agent delt på antall mergede agent-PR-er."
            />
          </HGrid>
        ) : (
          <BodyShort>Ingen data om kodeagenten ennå.</BodyShort>
        )}
        {prMonths.length > 0 && (
          <PlainTable
            rows={[
              [
                "Måned",
                "Opprettet av agent",
                "Merget",
                "Mergede / opprettede",
                "Tid til merge (agent)",
                "Tid til merge (alle)",
                "Agentkostnad (SKU)",
                "USD / merget agent-PR",
              ],
              ...prMonths.map((m) => [
                mLabel(m.month, m.preliminary),
                formatNumber(m.prCreatedByCopilot),
                formatNumber(m.prMergedCopilotAuthored),
                fmtPct(m.agentMergeSharePct),
                formatMinutes(m.avgMedianMinutesToMergeCopilot),
                formatMinutes(m.avgMedianMinutesToMerge),
                fmtUsd(m.cloudAgentNetUsd),
                fmtUsd(m.usdPerAgentMergedPRDirect ?? m.usdPerAgentMergedPR),
              ]),
            ]}
          />
        )}
      </InsightSection>

      <InsightSection id="kodegjennomgang" title="Verdi av kodegjennomgang">
        <BodyShort size="small" textColor="subtle">
          Andel av Copilots gjennomgangskommentarer som blir tatt i bruk, tid til merge og hva modellbruken til
          kodegjennomgang koster per gjennomgått PR og per forslag tatt i bruk.
        </BodyShort>
        {latestPR ? (
          <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-16">
            <MetricCard
              value={formatNumber(latestPR.prReviewedByCopilot)}
              label="PR-er gjennomgått av Copilot"
              subtitle={mLabel(latestPR.month, latestPR.preliminary)}
              helpTitle="PR-er gjennomgått av Copilot"
              helpText="Pull requests som har fått minst én kodegjennomgang fra Copilot i løpet av måneden."
            />
            <MetricCard
              value={fmtPct(latestPR.reviewAppliedSharePct)}
              label="Forslag tatt i bruk"
              subtitle={`${formatNumber(latestPR.copilotAppliedSuggestions)} av ${formatNumber(latestPR.copilotSuggestions)} forslag`}
              helpTitle="Forslag tatt i bruk"
              helpText="Andel av Copilots endringsforslag i kodegjennomgang som utvikleren har tatt direkte i bruk."
            />
            <MetricCard
              value={fmtUsd(latestPR.codeReviewNetUsd)}
              label="Modellkostnad kodegjennomgang"
              subtitle={
                latestPR.codeReviewSharePct !== null
                  ? `${formatPercent(latestPR.codeReviewSharePct, 1)} av månedskostnaden`
                  : mLabel(latestPR.month, latestPR.preliminary)
              }
              helpTitle="Modellkostnad kodegjennomgang"
              helpText="Fakturert nettobeløp for modellfamilien som brukes til kodegjennomgang i GitHub."
            />
            <MetricCard
              value={fmtUsd(latestPR.usdPerAppliedSuggestion)}
              label="USD per forslag tatt i bruk"
              subtitle={`Per gjennomgang: ${fmtUsd(latestPR.usdPerReviewDirect)}`}
              helpTitle="USD per forslag tatt i bruk"
              helpText="Direkte modellkostnad for kodegjennomgang delt på antall endringsforslag som ble tatt i bruk."
            />
          </HGrid>
        ) : (
          <BodyShort>Ingen data om kodegjennomgang ennå.</BodyShort>
        )}
        {prMonths.length > 0 && (
          <PlainTable
            rows={[
              [
                "Måned",
                "Gjennomgått",
                "Merget",
                "Forslag",
                "Tatt i bruk",
                "Andel tatt i bruk",
                "Tid til merge (gjennomgått)",
                "Modellkostnad",
                "USD / gjennomgang",
                "USD / forslag tatt i bruk",
              ],
              ...prMonths.map((m) => [
                mLabel(m.month, m.preliminary),
                formatNumber(m.prReviewedByCopilot),
                formatNumber(m.prMergedCopilotReviewed),
                formatNumber(m.copilotSuggestions),
                formatNumber(m.copilotAppliedSuggestions),
                fmtPct(m.reviewAppliedSharePct),
                formatMinutes(m.avgMedianMinutesToMergeReviewed),
                fmtUsd(m.codeReviewNetUsd),
                fmtUsd(m.usdPerReviewDirect),
                fmtUsd(m.usdPerAppliedSuggestion),
              ]),
            ]}
          />
        )}
      </InsightSection>

      <InsightSection id="kontekst" title="Kontekstforhold (inndata mot utdata)">
        <Alert variant="info" size="small">
          GitHub skiller ikke ut hurtigbufrede tokener i <code>usage_metrics</code>, og rapporterer bare tokener for
          Copilot CLI og Copilot-appen. Et høyt forhold mellom inndata og utdata kan skyldes lange økter uten
          komprimering, eller at modellen leser mye kode for å gjøre en liten, målrettet endring.
        </Alert>
        {latestToken ? (
          <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-16">
            <MetricCard
              value={fmtRatio(latestToken.cliRatio)}
              label="Forhold inn:ut i CLI"
              subtitle={mLabel(latestToken.month, latestToken.preliminary)}
              helpTitle="Forhold inn:ut i CLI"
              helpText="Antall inndatatokener sendt per utdatatoken generert i Copilot CLI."
            />
            <MetricCard
              value={formatNumber(latestToken.cliInputTokens)}
              label="Inndatatokener i CLI"
              subtitle={`Utdata: ${formatNumber(latestToken.cliOutputTokens)}`}
              helpTitle="Inndatatokener i CLI"
              helpText="Samlet antall inndatatokener (kontekst og ledetekst) i Copilot CLI denne måneden."
            />
            <MetricCard
              value={fmtRatio(latestToken.appRatio)}
              label="Forhold inn:ut i Copilot-appen"
              subtitle={mLabel(latestToken.month, latestToken.preliminary)}
              helpTitle="Forhold inn:ut i Copilot-appen"
              helpText="Antall inndatatokener sendt per utdatatoken generert i Copilot-appen."
            />
            <MetricCard
              value={formatNumber(latestToken.appInputTokens)}
              label="Inndatatokener i Copilot-appen"
              subtitle={`Utdata: ${formatNumber(latestToken.appOutputTokens)}`}
              helpTitle="Inndatatokener i Copilot-appen"
              helpText="Samlet antall inndatatokener i Copilot-appen denne måneden."
            />
          </HGrid>
        ) : (
          <BodyShort>Ingen tokentall ennå.</BodyShort>
        )}
        {tokenMonths.length > 0 && (
          <PlainTable
            rows={[
              ["Måned", "CLI inn", "CLI ut", "CLI inn:ut", "App inn", "App ut", "App inn:ut"],
              ...tokenMonths.map((m) => [
                mLabel(m.month, m.preliminary),
                formatNumber(m.cliInputTokens),
                formatNumber(m.cliOutputTokens),
                fmtRatio(m.cliRatio),
                formatNumber(m.appInputTokens),
                formatNumber(m.appOutputTokens),
                fmtRatio(m.appRatio),
              ]),
            ]}
          />
        )}
      </InsightSection>

      <InsightSection id="team-og-repo" title="Kostnad og resultat per team og repo">
        <VStack gap="space-16">
          <BodyShort size="small" textColor="subtle">
            Fakturadata har ikke repo, så oversikten kobler kostnad og tilpasningsgrad per team (minst fem aktive
            brukere) med PR-resultater per repo (offentlige og interne repoer med minst fem PR-er).
          </BodyShort>
          {teams && teams.teams.length > 0 ? (
            <>
              <BodyShort size="small">
                Teamkostnad og repotilpasning i {monthName(teams.month)} (
                {teams.basis === "net" ? "fakturert netto" : "brutto anslag"}).
              </BodyShort>
              <PlainTable
                rows={[
                  [
                    "Team",
                    "Aktive brukere",
                    "Kostnad",
                    "Kostnad per bruker",
                    "Aktive repoer",
                    "Repoer med tilpasninger",
                  ],
                  ...teams.teams
                    .slice(0, 20)
                    .map((t) => [
                      t.team_slug,
                      formatNumber(t.users),
                      formatUSD(t.amount_usd),
                      formatUSD(t.per_user_usd),
                      t.activeRepos !== null ? formatNumber(t.activeRepos) : dash,
                      t.reposWithCustomizations !== null && t.adoptionPct !== null
                        ? `${formatNumber(t.reposWithCustomizations)} (${formatPercent(t.adoptionPct, 0)})`
                        : dash,
                    ]),
                  ...(teams.smallTeams > 0
                    ? [
                        [
                          "Andre team (< 5 aktive brukere)",
                          `${formatNumber(teams.smallTeams)} team`,
                          "Skjult (< 5)",
                          dash,
                          dash,
                          dash,
                        ],
                      ]
                    : []),
                ]}
              />
            </>
          ) : (
            <BodyShort>Ingen teamkostnad tilgjengelig ennå.</BodyShort>
          )}
          {repos.length > 0 ? (
            <>
              <BodyShort size="small">
                Repoer med flest mergede pull requests der Copilot har bidratt som forfatter eller kontrollør (minst fem
                PR-er totalt i repoet).
              </BodyShort>
              <PlainTable
                rows={[
                  [
                    "Repo",
                    "PR-er merget",
                    "Skrevet av agent (merget)",
                    "Gjennomgått av Copilot (merget)",
                    "Forslag tatt i bruk",
                    "Tid til merge (agent)",
                    "Tid til merge (alle)",
                  ],
                  ...repos
                    .slice(0, 20)
                    .map((r) => [
                      r.repoName,
                      formatNumber(r.totalMerged),
                      formatNumber(r.mergedCopilotAuthored),
                      formatNumber(r.mergedCopilotReviewed),
                      fmtPct(r.suggestionApplyPct),
                      formatMinutes(r.avgMedianMinutesToMergeCopilot),
                      formatMinutes(r.avgMedianMinutesToMerge),
                    ]),
                ]}
              />
            </>
          ) : (
            <BodyShort>Ingen repodata tilgjengelig ennå.</BodyShort>
          )}
        </VStack>
      </InsightSection>

      <InsightSection id="metode" title="Metode og kjente begrensninger">
        <VStack gap="space-12">
          <BodyShort size="small">
            <strong>Arbeid utenfor pull requests:</strong> Feilsøking, kodeforståelse, oppslag og utforskning i IDE og
            terminal ender ikke nødvendigvis i en pull request eller et akseptert endringsforslag. Lav PR-telling betyr
            derfor ikke nødvendigvis lav nytte.
          </BodyShort>
          <BodyShort size="small">
            <strong>Delte modell-SKU-er for IDE og CLI:</strong> GitHub skiller ut <code>Copilot Cloud Agent</code> og{" "}
            <code>code-review-model</code> som egne varelinjer, men bruker ellers samme modellnavn på tvers av IDE-chat
            og CLI. Direkte kostnad per funksjon finnes derfor for kodeagenten og kodegjennomgang, men ikke for IDE og
            CLI hver for seg.
          </BodyShort>
          <BodyShort size="small">
            <strong>Seleksjonsskjevhet i tid til merge:</strong> Kodeagenten settes oftest på små, avgrensede oppgaver,
            og PR-er som lukkes uten merge inngår ikke i mediantiden. Kortere tid til merge for agent-PR-er enn for alle
            PR-er kan derfor skyldes oppgavestørrelse like mye som verktøyet.
          </BodyShort>
          <BodyShort size="small">
            <strong>Goodharts lov:</strong> Tall som kostnad per PR, forslag tatt i bruk og tid til merge er signaler
            for feilsøking og prioritering, ikke måltall det skal styres etter. Hvis målet blir mange eller billige
            PR-er, endres praksisen rundt PR-størrelse uten at den faktiske verdien øker.
          </BodyShort>
        </VStack>
      </InsightSection>
    </InsightPage>
  );
}
