import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { Alert, Link } from "@navikt/ds-react";
import { InsightPage, InsightSection } from "@/components/insight-page";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import ErrorState, { SectionError } from "@/components/error-state";
import { getUserToken } from "@/lib/auth";
import { formatDate, formatNumber } from "@/lib/format";
import { getPublishedSurveys } from "@/lib/survey";

export const metadata: Metadata = {
  title: "Undersøkelser",
  description: "Resultater fra avsluttede brukerundersøkelser om KI-verktøy i Nav.",
  robots: { index: false, follow: false },
};

export default async function PublishedSurveysPage() {
  const token = await getUserToken();
  if (!token) return <ErrorState title="Undersøkelser" message="Mangler innloggingstoken" />;
  const res = await getPublishedSurveys(token);
  if (res.status === "not-found") notFound();
  const surveys = res.status === "error" ? null : res.data.surveys;

  return (
    <InsightPage
      title="Undersøkelser"
      description="Resultater fra avsluttede brukerundersøkelser om KI-verktøy i Nav."
      intro="Se resultatene fra avsluttede brukerundersøkelser om KI-verktøy i Nav. Hver undersøkelse viser antall svar, svarkanaler og fordeling per spørsmål."
      updated={async () => surveys?.[0]?.ends}
      source={
        <>
          Resultatene kommer fra <code>copilot-survey</code> og viser aggregerte tall for avsluttede og publiserte
          undersøkelser. Alternativer med færre enn fem svar vises som «&lt;5» av hensyn til personvern, og fritekstsvar
          publiseres ikke her. Spørsmål eller innspill kan tas i #ki-produktutvikling på Slack.
        </>
      }
    >
      <InsightSection id="publiserte-undersokelser" title="Publiserte undersøkelser">
        {!surveys ? (
          <SectionError message="Kunne ikke hente undersøkelsene." />
        ) : surveys.length === 0 ? (
          <Alert variant="info">
            Ingen undersøkelser er publisert ennå. Resultatene legges ut her når en undersøkelse er avsluttet og
            gjennomgått.
          </Alert>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHeaderCell scope="col">Undersøkelse</TableHeaderCell>
                <TableHeaderCell scope="col">Periode</TableHeaderCell>
                <TableHeaderCell scope="col" align="right">
                  Svar
                </TableHeaderCell>
              </TableRow>
            </TableHeader>
            <TableBody>
              {surveys.map((s) => (
                <TableRow key={s.id}>
                  <TableDataCell>
                    <Link href={`/innsikt/undersokelser/${s.id}`}>{s.title}</Link>
                  </TableDataCell>
                  <TableDataCell>
                    {formatDate(s.starts)}–{formatDate(s.ends)}
                  </TableDataCell>
                  <TableDataCell align="right">{formatNumber(s.responses)}</TableDataCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </InsightSection>
    </InsightPage>
  );
}
