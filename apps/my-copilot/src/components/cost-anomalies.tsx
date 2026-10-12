import { BodyShort, Link } from "@navikt/ds-react";
import { List, ListItem } from "@navikt/ds-react/List";
import { InsightSection } from "@/components/insight-page";
import { getUserToken } from "@/lib/auth";
import { backendRequest } from "@/lib/backend-api";
import { formatDate, formatUSD } from "@/lib/format";

// One finding from copilot-api's GET /api/v1/copilot/billing/anomalies (admin only).
export interface Anomaly {
  rule: "A1" | "A3" | "A4" | "A7";
  date: string;
  value: number;
  threshold: number;
  model?: string;
}

// Data gaps first (K12): the other rules mean nothing without data. A4 (forecast over
// budget) comes from the forecast work (#1691) and gets its slot here.
const order: Anomaly["rule"][] = ["A7", "A4", "A1", "A3"];

export function anomalyText(a: Anomaly): { text: string; href: string } {
  switch (a.rule) {
    case "A7":
      if (/^\d{4}-\d{2}$/.test(a.date))
        return {
          text: `Fakturert forbruk for ${a.date} er ikke lest inn, selv om det skal være klart innen den ${a.threshold}..`,
          href: "/innsikt/team",
        };
      if (a.threshold > 0)
        return {
          text: a.date
            ? `Ingen nye forbruksdata siden ${formatDate(a.date)}. Vanligvis kommer det nye data innen ${a.threshold} døgn.`
            : "Det finnes ingen forbruksdata.",
          href: "/innsikt/bruk#kostnad",
        };
      return { text: `Forbruksdata mangler for ${formatDate(a.date)}.`, href: "/innsikt/bruk#kostnad" };
    case "A1":
      return {
        text: `Forbruket ${formatDate(a.date)} var ${formatUSD(a.value)}. Medianen for samme type dag de siste 28 dagene er ${formatUSD(a.threshold)}.`,
        href: "/innsikt/bruk#kostnad",
      };
    case "A3":
      return {
        text: `Ny modell: ${a.model} kostet ${formatUSD(a.value)} den ${formatDate(a.date)}, første dag med mer enn ${formatUSD(a.threshold)}.`,
        href: "/innsikt/bruk#kostnad",
      };
    case "A4":
      return { text: `Prognosen for ${a.date} er over budsjettet.`, href: "/innsikt/kostnad" };
  }
}

export function AnomalyList({ anomalies }: { anomalies: Anomaly[] | null }) {
  if (anomalies === null) return <BodyShort>Vi klarte ikke å hente avvikene nå.</BodyShort>;
  if (anomalies.length === 0) return <BodyShort>Ingen avvik de siste 30 dagene.</BodyShort>;
  const sorted = [...anomalies].sort(
    (a, b) => order.indexOf(a.rule) - order.indexOf(b.rule) || b.date.localeCompare(a.date)
  );
  return (
    <List>
      {sorted.map((a) => {
        const { text, href } = anomalyText(a);
        return (
          <ListItem key={`${a.rule}-${a.date}-${a.model ?? ""}`}>
            {text} <Link href={href}>Se tallene</Link>
          </ListItem>
        );
      })}
    </List>
  );
}

export async function CostAnomalies() {
  const token = await getUserToken();
  let anomalies: Anomaly[] | null = null;
  if (token) {
    try {
      anomalies = await backendRequest<Anomaly[]>("/api/v1/copilot/billing/anomalies", token);
    } catch (error) {
      console.error("[kostnad] Anomalies failed:", error);
    }
  }
  return (
    <InsightSection id="avvik" title="Avvik">
      <BodyShort>
        Dager med uvanlig høyt forbruk, nye modeller og hull i dataene de siste 30 dagene. Tersklene er startverdier som
        team copilot justerer etter tre måneder.
      </BodyShort>
      <AnomalyList anomalies={anomalies} />
    </InsightSection>
  );
}
