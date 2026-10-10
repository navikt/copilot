import { cache, type ReactNode } from "react";
import { BodyShort, ReadMore } from "@navikt/ds-react";
import { List, ListItem } from "@navikt/ds-react/List";
import { ShareChart } from "@/components/charts/MonthlyTrendCharts";
import { getUserSegments } from "@/lib/cached-bigquery";
import {
  annotationsFor,
  formatPp,
  monthLabel,
  chartNet,
  chartShares,
  shareChange,
  visibleMonths,
  type ShareSeries,
} from "@/lib/trends";
import { CHART_ANNOTATIONS } from "../../reisen/milestones";

// One request per page render, shared by every section.
const segmentsFor = cache(getUserSegments);

const HIDDEN =
  "En gruppe med færre enn fem brukere eller team en måned slås sammen med nabogruppen, for eksempel «Under 60 %». Er det fortsatt for få, vises ikke måneden.";

/** `all`: mark every event in the charts, not only the data breaks. */
type Props = { token: string; start: string | null; all?: boolean };

async function load({ token, start }: Props) {
  const { segments, error } = await segmentsFor(token);
  if (error || !segments) return { failed: error ?? "ingen data" };
  const months = (rows: { month: string }[]) =>
    visibleMonths(
      rows.map((r) => r.month),
      start
    ).months;
  const im = months(segments.intensity.months.map((month) => ({ month })));
  const mm = months(segments.mode.months.map((month) => ({ month })));
  const vm = months(segments.movement.months.map((month) => ({ month })));
  const tm = months(segments.team_adoption.months.map((month) => ({ month })));
  const net: ShareSeries = { label: "Netto (opp − ned)", shares: chartNet(segments.movement, vm) };
  return {
    intensity: { months: im, series: chartShares(segments.intensity, im) },
    mode: { months: mm, series: chartShares(segments.mode, mm) },
    movement: { months: vm, series: [...chartShares(segments.movement, vm), net] },
    teams: { months: tm, series: chartShares(segments.team_adoption, tm) },
  };
}

function Method({ children }: { children: ReactNode }) {
  return (
    <ReadMore header="Kilde og metode" size="small">
      {children}
    </ReadMore>
  );
}

function Failed({ error }: { error?: string }) {
  return <BodyShort>{`Kunne ikke hente segmentene: ${error}`}</BodyShort>;
}

/** «Hva har endret seg»: change in share points per group over the chosen period, from the same aggregates. */
export async function SegmentChanges(props: Props) {
  const data = await load(props);
  if ("failed" in data) return <Failed error={data.failed} />;
  const groups = [
    { title: "Intensitet", ...data.intensity },
    { title: "Arbeidsmåte", ...data.mode },
    { title: "Team etter andel aktive", ...data.teams },
    { title: "Bevegelse", months: data.movement.months, series: data.movement.series.slice(-1) },
  ];
  const first = groups.flatMap((g) => g.months).sort()[0];
  if (!first) return <BodyShort>Ingen segmentdata i perioden.</BodyShort>;
  return (
    <>
      <BodyShort size="small" textColor="subtle">
        Endring i prosentpoeng (pp) fra første til siste måned med tall i perioden du har valgt, fra {monthLabel(first)}
        . Den inneværende måneden er ikke ferdig.
      </BodyShort>
      <List size="small">
        {groups.map((g) => {
          const parts = g.series
            .map((s) => ({ label: s.label, pp: shareChange(s) }))
            .filter((p): p is { label: string; pp: number } => p.pp !== null)
            .map((p) => `${p.label.toLowerCase()} ${formatPp(p.pp)}`);
          return (
            <ListItem key={g.title}>
              <strong>{g.title}:</strong> {parts.length ? parts.join(", ") : "for lite data"}
            </ListItem>
          );
        })}
      </List>
    </>
  );
}

export async function Movement(props: Props) {
  const data = await load(props);
  if ("failed" in data) return <Failed error={data.failed} />;
  const { months, series } = data.movement;
  if (!months.length) return <BodyShort>Ingen data om bevegelse i perioden.</BodyShort>;
  return (
    <>
      <BodyShort size="small" textColor="subtle">
        Andelen av brukerne som var aktive både forrige og denne måneden, og som gikk opp, ble i eller gikk ned en
        intensitetsgruppe. Netto er opp minus ned: over null betyr at flere bruker Copilot mer enn før.
      </BodyShort>
      <ShareChart
        months={months}
        series={series}
        stacked={false}
        label="Andel brukere som gikk opp, ble i eller gikk ned en intensitetsgruppe per måned, og netto"
        annotations={annotationsFor(CHART_ANNOTATIONS, months, props.all)}
      />
      <Method>
        Regnet ut i BigQuery fra <code>user_metrics</code> (<code>/usage/segments</code>) per person, men bare summene
        hentes ut. Bare brukere som var aktive begge månedene, er med: de som slutter eller begynner, er ikke bevegelse
        her, men vises i kohortene. Gruppene er de samme som under intensitet. Andelene regnes av alle som var aktive
        begge månedene. {HIDDEN}
      </Method>
    </>
  );
}

export async function Intensity(props: Props) {
  const data = await load(props);
  if ("failed" in data) return <Failed error={data.failed} />;
  const { months, series } = data.intensity;
  if (!months.length) return <BodyShort>Ingen data om intensitet i perioden.</BodyShort>;
  return (
    <>
      <ShareChart
        months={months}
        series={series}
        stacked
        label="Andel aktive brukere som er lette, middels eller tunge brukere per måned"
        annotations={annotationsFor(CHART_ANNOTATIONS, months, props.all)}
      />
      <Method>
        Hver aktiv bruker plasseres etter hvor mange ganger hen selv tok kontakt med Copilot i måneden (
        <code>user_initiated_interaction_count</code>): lett under 20, middels 20–199, tung 200 eller flere. Grensene er
        faste og hentes ikke fra fordelingen, så en endring betyr at folk bruker Copilot annerledes, ikke at grensene
        har flyttet seg. Vi bruker antall interaksjoner og ikke AI Credits, fordi AI Credits bare finnes fra 1. juni
        2026. Den som bare bruker kodeforslag, er lett. {HIDDEN}
      </Method>
    </>
  );
}

export async function WayOfWorking(props: Props) {
  const data = await load(props);
  if ("failed" in data) return <Failed error={data.failed} />;
  const { months, series } = data.mode;
  if (!months.length) return <BodyShort>Ingen data om arbeidsmåte i perioden.</BodyShort>;
  return (
    <>
      <ShareChart
        months={months}
        series={series}
        stacked
        label="Andel aktive brukere etter viktigste arbeidsmåte per måned"
        annotations={annotationsFor(CHART_ANNOTATIONS, months, props.all)}
      />
      <Method>
        Hver aktiv bruker telles én gang per måned, etter den mest selvstendige måten hen brukte Copilot på: CLI (
        <code>used_cli</code>) foran agentmodus (<code>used_agent</code>) foran chat (<code>used_chat</code>). Den som
        ikke har brukt noen av dem, har bare brukt kodeforslag. <code>user_metrics</code> har ikke et eget felt for
        Copilot coding agent per bruker, så den er ikke med. Når et felt mangler for en dag, regnes det som ikke brukt;
        vi har ikke sjekket fra hvilken dato GitHub begynte å levere hvert felt, så tidlige måneder kan undervurdere CLI
        og agentmodus. {HIDDEN}
      </Method>
    </>
  );
}

export async function TeamAdoption(props: Props) {
  const data = await load(props);
  if ("failed" in data) return <Failed error={data.failed} />;
  const { months, series } = data.teams;
  if (!months.length) return <BodyShort>Ingen teamdata i perioden.</BodyShort>;
  return (
    <>
      <ShareChart
        months={months}
        series={series}
        stacked
        label="Andel team med lav, middels eller høy andel aktive medlemmer per måned"
        annotations={annotationsFor(CHART_ANNOTATIONS, months, props.all)}
        shadeBefore="2026-04"
      />
      <Method>
        For hvert team med minst fem medlemmer regner vi andelen medlemmer som var aktive i måneden: lav under 25 %,
        middels 25–59 %, høy 60 % eller mer. Ingen team navngis. Medlemskapet er det som gjelder i dag (siste dag i{" "}
        <code>user_teams</code>), brukt bakover på alle måneder. Et team som har fått nye medlemmer, ser derfor ut til å
        ha hatt dem også før. Månedene før april 2026 er skyggelagt: de kan ikke sammenlignes direkte med månedene etter
        (se «Hendelser»). {HIDDEN}
      </Method>
    </>
  );
}
