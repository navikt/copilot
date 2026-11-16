import { Alert, BodyShort } from "@navikt/ds-react";
import { formatDate } from "@/lib/format";
import type { AdminMonthlyOverview } from "@/lib/types";

export function AdminSourceStatus({ data }: { data?: AdminMonthlyOverview | null }) {
  const coverage = data?.coverage;
  const lastNet = data?.months.find((m) => m.month === coverage?.last_net_month);
  const loadedAt = lastNet && (lastNet.basis === "net" || lastNet.basis === "premium") ? lastNet.loaded_at : "";

  return (
    <>
      {coverage?.gross_stale ? (
        <Alert variant="warning" size="small">
          Foreløpig forbruk er sist hentet for{" "}
          {coverage.last_gross_day ? formatDate(coverage.last_gross_day) : "ukjent dato"}, mer enn to døgn siden.
          Tallene for denne måneden kan være for lave.
        </Alert>
      ) : (
        <BodyShort size="small" textColor="subtle">
          Siste dag med foreløpig forbruk: {coverage?.last_gross_day ? formatDate(coverage.last_gross_day) : "ukjent"}.
        </BodyShort>
      )}
      <BodyShort size="small" textColor="subtle">
        {loadedAt
          ? `Månedsdata for ${lastNet?.month} er innlest ${formatDate(loadedAt)}.`
          : "Innlesingsdato for månedsdata: ukjent."}{" "}
        Status for pågående innsamling er ukjent.
      </BodyShort>
    </>
  );
}
