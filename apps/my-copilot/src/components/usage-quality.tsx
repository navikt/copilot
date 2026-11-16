import { BodyShort } from "@navikt/ds-react";
import { backendRequest } from "@/lib/backend-api";
import { formatNumber } from "@/lib/format";
import type { UsageQualityDay } from "@/lib/usage-quality";
import { NumbersFold, PlainTable } from "./charts/chart-figure";

const count = (value: number | null | undefined) => (value == null ? "Ukjent" : formatNumber(value));

export function UsageQualityTable({ days }: { days: UsageQualityDay[] }) {
  if (!days.length) return <BodyShort>Ingen dagsrapporter om datakvalitet ennå.</BodyShort>;
  const latest = days[days.length - 1];
  const telemetryDays = days.filter((d) => d.missing_client_telemetry_user_count != null).length;
  const versionDays = days.filter((d) => d.below_usage_metrics_minimum_user_count != null).length;
  return (
    <>
      <BodyShort>
        Siste rapportdag {latest.day}, {latest.scope}: {latest.scope_id}. Antall brukere uten klienttelemetri:{" "}
        {count(latest.missing_client_telemetry_user_count)}. Antall under minstekravene til IDE eller Copilot-utvidelse:{" "}
        {count(latest.below_usage_metrics_minimum_user_count)}. Tallene kan overlappe og skal ikke legges sammen, heller
        ikke på tvers av dager.
      </BodyShort>
      <BodyShort>
        Brukere uten klienttelemetri teller fortsatt som aktive, men detaljer om kode, funksjoner og modeller kan
        mangle. Flagget sier ikke hvorfor telemetrien mangler.
      </BodyShort>
      <BodyShort size="small" textColor="subtle">
        Av {days.length} dagsrapporter i de siste 28 kalenderdagene har {telemetryDays} telemetritall og {versionDays}{" "}
        versjonstall. Ukjent betyr utilgjengelig, ikke null brukere. Eldre dager kan mangle tall. Hver dag viser én
        kilde, enterprise nav når den finnes, ellers organisasjonen navikt. Vi erstatter ikke ukjente enterprise-tall
        med organisasjonstall. Organisasjonstall dekker ikke hele enterprise.
      </BodyShort>
      <NumbersFold summary="Vis datakvalitet per dag">
        <PlainTable
          rows={[
            ["Dag", "Kilde", "Uten klienttelemetri", "Under minstekrav"],
            ...days.map((d) => [
              d.day,
              `${d.scope}: ${d.scope_id}`,
              count(d.missing_client_telemetry_user_count),
              count(d.below_usage_metrics_minimum_user_count),
            ]),
          ]}
        />
      </NumbersFold>
    </>
  );
}

export default async function UsageQuality({ token }: { token: string }) {
  let days: UsageQualityDay[];
  try {
    days = await backendRequest<UsageQualityDay[]>("/api/v1/copilot/usage/quality", token);
  } catch {
    return <BodyShort>Kunne ikke hente datakvalitet.</BodyShort>;
  }
  return <UsageQualityTable days={days} />;
}
