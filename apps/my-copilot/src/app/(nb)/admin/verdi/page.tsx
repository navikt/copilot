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
  getBillingModelBreakdown,
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
  const [admin, breakdownRes, usageRes, volume, reposRes, teamGross, teamNet] = await Promise.all([
    getAdminMonthlyOverview(year, token).catch(() => null),
    getBillingModelBreakdown(token, 12).catch(() => ({ breakdown: [], error: "feil" })),
    getCopilotUsageMetrics(token).catch(() => ({ usage: null, error: "feil" })),
    getAdminVolume(year, token).catch(() => []),
    getRepositoryUsage(token).catch(() => ({ repositories: [], error: "feil" })),
    getTeamGrossOverview(teamMonth, token).catch(() => null),
    getTeamNetOverview(teamMonth, token).catch(() => null),
  ]);

  const months = buildVerdiMonths({
    admin,
    breakdown: breakdownRes.error ? null : breakdownRes.breakdown,
    usage: usageRes.usage,
    volume,
  });
  const repos = buildVerdiRepos(reposRes.repositories);
  const teams = buildVerdiTeams(teamNet, teamGross);
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
      description="Kostnad per resultat, kodeagent, kodegjennomgang, kontekstforbruk og fordeling per team og repo."
      intro="Kostnadssiden viser hva Copilot koster. Her ser du hva vi får igjen i pull requests, kodegjennomgang og tokenforbruk, basert på tallene vi allerede samler inn. Grupper med færre enn fem aktive brukere eller færre enn fem pull requests vises ikke enkeltvis."
      updated={async () => updated}
      source={
        <>
          Kostnadstallene kommer fra <code>/admin/billing/monthly</code> og <code>/billing/model-breakdown</code> (netto
          USD etter rabatt, uten lisenser). Pull request-tallene kommer fra <code>/usage/metrics</code> (GitHubs{" "}
          <code>pull_requests</code>-felt fra 17. juli 2026) og <code>/usage/repositories</code> (
          <code>v_repository_usage</code>, offentlige og interne repoer med minst fem PR-er). Tokentallene kommer fra{" "}
          <code>/admin/billing/volume</code>. Teamkostnad kommer fra <code>/usage/team-net</code> og{" "}
          <code>/usage/team-gross</code> med femgrense.
        </>
      }
    >
      <InsightSection id="kostnad-per-resultat" title="Kostnad per resultat">
        <BodyShort size="small" textColor="subtle">
          Fakturadata skiller ut modellkostnaden for automatisk kodegjennomgang (<code>code-review-model</code>), men
          ikke kostnaden for kodeagenten eller IDE-chat hver for seg. Derfor vises også total netto kostnad delt på
          antall resultater som et grovt mål å følge over tid.
        </BodyShort>
        {latestPR ? (
          <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-16">
            <MetricCard
              value={fmtUsd(latestPR.usdPerMergedPR)}
              label="USD per slått sammen PR (grovt)"
              subtitle={mLabel(latestPR.month, latestPR.preliminary)}
              helpTitle="Total kostnad per slått sammen PR"
              helpText="Månedens totale netto kostnad delt på alle sammenslåtte pull requests i måneden."
            />
            <MetricCard
              value={fmtUsd(latestPR.usdPerAgentMergedPR)}
              label="USD per slått sammen agent-PR (grovt)"
              subtitle={mLabel(latestPR.month, latestPR.preliminary)}
              helpTitle="Total kostnad per slått sammen kodeagent-PR"
              helpText="Månedens totale netto kostnad delt på pull requests opprettet av Copilot coding agent som ble slått sammen."
            />
            <MetricCard
              value={fmtUsd(latestPR.usdPerReviewDirect)}
              label="USD per kodegjennomgang (modell)"
              subtitle={mLabel(latestPR.month, latestPR.preliminary)}
              helpTitle="Direkte modellkostnad per kodegjennomgang"
              helpText="Månedens netto kostnad for code-review-model delt på antall pull requests gjennomgått av Copilot."
            />
            <MetricCard
              value={fmtUsd(latestPR.usdPerReviewGrossScope)}
              label="USD per kodegjennomgang (grovt)"
              subtitle={mLabel(latestPR.month, latestPR.preliminary)}
              helpTitle="Total kostnad per kodegjennomgang"
              helpText="Månedens totale netto kostnad delt på antall pull requests gjennomgått av Copilot."
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
                "Slått sammen PR",
                "USD / PR (grovt)",
                "USD / agent-PR (grovt)",
                "USD / gjennomgang (modell)",
                "USD / gjennomgang (grovt)",
              ],
              ...prMonths.map((m) => [
                mLabel(m.month, m.preliminary),
                fmtUsd(m.netUsd),
                formatNumber(m.prTotalMerged),
                fmtUsd(m.usdPerMergedPR),
                fmtUsd(m.usdPerAgentMergedPR),
                fmtUsd(m.usdPerReviewDirect),
                fmtUsd(m.usdPerReviewGrossScope),
              ]),
            ]}
          />
        )}
      </InsightSection>

      <InsightSection id="kodeagent" title="Kodeagentens effekt">
        <BodyShort size="small" textColor="subtle">
          Hvor mange pull requests Copilot coding agent oppretter, forholdet mellom sammenslåtte og opprettede PR-er i
          måneden, og median tid til sammenslåing. Siden PR-er kan opprettes i én måned og slås sammen i en senere
          måned, kan forholdstallet overstige 100 %. Kjøringer som avbrytes uten at det opprettes en PR, telles ikke i
          GitHubs bruksdata.
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
              label="Sammenslåtte mot opprettede"
              subtitle={`${formatNumber(latestPR.prMergedCopilotAuthored)} slått sammen i måneden`}
              helpTitle="Sammenslåtte mot opprettede (flytrate)"
              helpText="Agent-PR-er slått sammen i måneden delt på agent-PR-er opprettet i samme måned. Kan overstige 100 % når PR-er fra tidligere måneder slås sammen."
            />
            <MetricCard
              value={formatMinutes(latestPR.avgMedianMinutesToMergeCopilot)}
              label="Tid til merge (agent)"
              subtitle={`Alle PR-er: ${formatMinutes(latestPR.avgMedianMinutesToMerge)}`}
              helpTitle="Tid til merge (agent)"
              helpText="Gjennomsnitt av daglig mediantid fra opprettelse til sammenslåing for agent-PR-er."
            />
            <MetricCard
              value={fmtUsd(latestPR.usdPerAgentMergedPR)}
              label="USD per slått sammen agent-PR (grovt)"
              subtitle="Hele månedskostnaden delt på agent-PR"
              helpTitle="USD per slått sammen agent-PR"
              helpText="Hele månedens fakturerte beløp delt på antall sammenslåtte agent-PR-er. Grove forholdstall på tvers av måneder."
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
                "Slått sammen",
                "Sammenslåtte / opprettede",
                "Tid til merge (agent)",
                "Tid til merge (alle)",
              ],
              ...prMonths.map((m) => [
                mLabel(m.month, m.preliminary),
                formatNumber(m.prCreatedByCopilot),
                formatNumber(m.prMergedCopilotAuthored),
                fmtPct(m.agentMergeSharePct),
                formatMinutes(m.avgMedianMinutesToMergeCopilot),
                formatMinutes(m.avgMedianMinutesToMerge),
              ]),
            ]}
          />
        )}
      </InsightSection>

      <InsightSection id="kodegjennomgang" title="Verdi av kodegjennomgang">
        <BodyShort size="small" textColor="subtle">
          Andel av Copilots gjennomgangskommentarer som blir tatt i bruk, og hva modellbruken til kodegjennomgang koster
          per måned og per gjennomgått PR.
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
              value={fmtUsd(latestPR.usdPerReviewDirect)}
              label="USD per gjennomgang (modell)"
              subtitle={`Tid til merge: ${formatMinutes(latestPR.avgMedianMinutesToMergeReviewed)}`}
              helpTitle="USD per gjennomgang (modell)"
              helpText="Direkte modellkostnad for kodegjennomgang delt på antall PR-er gjennomgått av Copilot."
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
                "Slått sammen",
                "Forslag",
                "Tatt i bruk",
                "Andel tatt i bruk",
                "Modellkostnad",
                "USD / gjennomgang",
              ],
              ...prMonths.map((m) => [
                mLabel(m.month, m.preliminary),
                formatNumber(m.prReviewedByCopilot),
                formatNumber(m.prMergedCopilotReviewed),
                formatNumber(m.copilotSuggestions),
                formatNumber(m.copilotAppliedSuggestions),
                fmtPct(m.reviewAppliedSharePct),
                fmtUsd(m.codeReviewNetUsd),
                fmtUsd(m.usdPerReviewDirect),
              ]),
            ]}
          />
        )}
      </InsightSection>

      <InsightSection id="kontekst" title="Kontekstsløsing (inndata mot utdata)">
        <Alert variant="info" size="small">
          GitHub skiller ikke ut hurtigbufrede tokener i <code>usage_metrics</code>, og rapporterer bare tokener for
          Copilot CLI og Copilot-appen. Høyt forhold mellom inndata og utdata betyr at store mengder kontekst sendes inn
          for hver linje som skrives ut.
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
            Fakturadata har ikke repo, så fase 1 viser kostnad per team (minst fem aktive brukere) og PR-resultater per
            repo (offentlige og interne repoer med minst fem PR-er).
          </BodyShort>
          {teams && teams.teams.length > 0 ? (
            <>
              <BodyShort size="small">
                Teamkostnad i {monthName(teams.month)} ({teams.basis === "net" ? "fakturert netto" : "brutto anslag"}).
                {teams.smallTeams > 0 &&
                  ` Andre team med færre enn fem aktive brukere (${formatNumber(teams.smallTeams)} team) er utelatt av hensyn til personvern.`}
              </BodyShort>
              <PlainTable
                rows={[
                  ["Team", "Aktive brukere", "Kostnad", "Kostnad per bruker"],
                  ...teams.teams
                    .slice(0, 20)
                    .map((t) => [
                      t.team_slug,
                      formatNumber(t.users),
                      formatUSD(t.amount_usd),
                      formatUSD(t.per_user_usd),
                    ]),
                ]}
              />
            </>
          ) : (
            <BodyShort>Ingen teamkostnad tilgjengelig ennå.</BodyShort>
          )}
          {repos.length > 0 ? (
            <>
              <BodyShort size="small">
                Repoer med flest sammenslåtte pull requests der Copilot har bidratt som forfatter eller kontrollør
                (minst fem PR-er totalt i repoet).
              </BodyShort>
              <PlainTable
                rows={[
                  [
                    "Repo",
                    "PR-er slått sammen",
                    "Skrevet av agent (slått sammen)",
                    "Gjennomgått av Copilot (slått sammen)",
                    "Forslag tatt i bruk",
                    "Tid til merge (agent)",
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
                    ]),
                ]}
              />
            </>
          ) : (
            <BodyShort>Ingen repodata tilgjengelig ennå.</BodyShort>
          )}
        </VStack>
      </InsightSection>
    </InsightPage>
  );
}
