import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { Box, Link, Tag } from "@navikt/ds-react";
import { PageHero } from "@/components/page-hero";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import ErrorState from "@/components/error-state";
import { getUserToken, requireAdmin } from "@/lib/auth";
import { formatDate, formatNumber } from "@/lib/format";
import { getAdminSurveys, type SurveySummary } from "@/lib/survey";

export const metadata: Metadata = {
  title: "Undersøkelser",
  description: "Resultater fra brukerundersøkelsene.",
  robots: { index: false, follow: false },
};

const statusTag: Record<SurveySummary["status"], "info" | "success" | "neutral"> = {
  kommende: "info",
  åpen: "success",
  stengt: "neutral",
};

export default async function SurveysPage() {
  await requireAdmin();
  const token = await getUserToken();
  if (!token) return <ErrorState title="Undersøkelser" message="Mangler innloggingstoken" />;
  const res = await getAdminSurveys(token);
  if (res.status === "not-found") notFound();
  if (res.status === "error") return <ErrorState title="Undersøkelser" message="Kunne ikke hente undersøkelsene." />;

  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero title="Undersøkelser" description="Resultater fra brukerundersøkelsene." />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <Table>
            <TableHeader>
              <TableRow>
                <TableHeaderCell scope="col">Undersøkelse</TableHeaderCell>
                <TableHeaderCell scope="col">Periode</TableHeaderCell>
                <TableHeaderCell scope="col">Status</TableHeaderCell>
                <TableHeaderCell scope="col" align="right">
                  Svar
                </TableHeaderCell>
              </TableRow>
            </TableHeader>
            <TableBody>
              {res.data.surveys.map((s) => (
                <TableRow key={s.id}>
                  <TableDataCell>
                    <Link href={`/admin/undersokelser/${s.id}`}>{s.title}</Link>
                  </TableDataCell>
                  <TableDataCell>
                    {formatDate(s.starts)}–{formatDate(s.ends)}
                  </TableDataCell>
                  <TableDataCell>
                    <Tag variant={statusTag[s.status]} size="small">
                      {s.status}
                    </Tag>
                  </TableDataCell>
                  <TableDataCell align="right">{formatNumber(s.responses)}</TableDataCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Box>
      </div>
    </main>
  );
}
