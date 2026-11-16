import type { Metadata } from "next";
import { BodyLong, BodyShort } from "@navikt/ds-react";
import { InsightPage, InsightSection } from "@/components/insight-page";
import ErrorState from "@/components/error-state";
import { getUserToken, requireAdmin } from "@/lib/auth";
import { getAdminMonthlyOverview, getCohortRetention, getCopilotPRsMonthly } from "@/lib/cached-bigquery";
import { buildMonthlyOwnerDraft, previousUtcMonth } from "@/lib/monthly-owner-draft";
import { formatDate } from "@/lib/format";

export const metadata: Metadata = { title: "Månedsutkast for Copilot" };

export default async function MonthlyOwnerDraftPage() {
  await requireAdmin();
  const token = await getUserToken();
  if (!token) return <ErrorState message="Mangler innloggingstoken" />;
  const now = new Date();
  const month = previousUtcMonth(now);
  let draft;
  try {
    const admin = await getAdminMonthlyOverview(Number(month.slice(0, 4)), token);
    draft = buildMonthlyOwnerDraft({ now, admin, cohorts: null, prs: null });
    if (draft.ready) {
      let previousError = false;
      const [previous, cohortResult, prResult] = await Promise.all([
        month.endsWith("-01")
          ? getAdminMonthlyOverview(Number(month.slice(0, 4)) - 1, token).catch(() => {
              previousError = true;
              return undefined;
            })
          : Promise.resolve(undefined),
        getCohortRetention(token),
        getCopilotPRsMonthly(token),
      ]);
      draft = buildMonthlyOwnerDraft({
        now,
        admin,
        previous,
        previousError,
        cohorts: cohortResult.error ? null : cohortResult.cohorts,
        prs: prResult.error ? null : prResult.months,
        cohortError: !!cohortResult.error,
        prError: !!prResult.error,
      });
    }
  } catch {
    return <ErrorState message="Kunne ikke hente grunnlaget for månedsutkastet." />;
  }
  return (
    <InsightPage
      title={`Månedsutkast for Copilot, ${draft.month}`}
      description="Kun forhåndsvisning for administratorer av forrige kalendermåned i UTC. Ingenting sendes eller lagres."
      width="prose"
      source="Kostnadene gjelder Copilot i Nav fra den eksisterende månedlige adminoversikten. Ferdig innsamling er ikke kontroll mot faktura. Bruksdata er separate observasjoner fra bruker- og reporapportene, ikke bekreftet komplette av faktureringsinnsamlingen."
    >
      <InsightSection id="manedsutkast" title={draft.ready ? "Utkast til gjennomgang" : "Venter på innsamling"}>
        {draft.ready ? (
          <>
            <BodyShort>
              Innsamlet: {formatDate(draft.loadedAt!)}. Grunnlaget kan oppdateres ved ny innsamling.
            </BodyShort>
            {draft.findings.map((finding) => (
              <BodyLong key={finding}>{finding}</BodyLong>
            ))}
            <BodyLong>
              Retensjon følger stabil bruker-ID på tvers av enterprise og organisasjon, ikke alle i Nav. M1, M3 og M6
              gjelder ulike kohorter, ikke utviklingen for samme gruppe. Bare grupper med minst fem brukere og avsluttet
              observasjonsmåned vises. Kohorter til og med oktober 2025 er utelatt fordi tidligere aktivitet mangler.
              PR-tall utelater private repoer og måler ikke produktivitet. Endring vises ikke fordi likt repo-utvalg
              ikke kan bekreftes.
            </BodyLong>
            <BodyLong>
              Modellfordeling og modellendring er ikke tilgjengelige med bekreftet sammenlignbar månedsdekning.
              Månedlige aktive brukere, lisensutnyttelse, tokens for hele Copilot og månedlig funksjonsfordeling inngår
              ikke i grunnlaget.
            </BodyLong>
          </>
        ) : (
          <BodyLong>
            Utkastet for {draft.month} venter på en eksplisitt ferdig innsamlet måned. En eldre ferdig måned erstatter
            ikke denne måneden.
          </BodyLong>
        )}
      </InsightSection>
    </InsightPage>
  );
}
