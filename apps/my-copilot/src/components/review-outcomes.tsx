import { BodyShort } from "@navikt/ds-react";
import { formatNumber } from "@/lib/format";
import type { RepositoryUsage } from "@/lib/types";

type Outcomes = Pick<
  RepositoryUsage,
  | "review_comments_evaluated"
  | "review_comments_addressed"
  | "review_merged_no_human_change_request"
  | "review_outcome_days"
  | "review_outcome_first_day"
  | "review_outcome_last_day"
  | "review_breakdowns"
  | "review_comment_days"
>;

export default function ReviewOutcomes({ repositories }: { repositories: Outcomes[] }) {
  const reported = repositories.filter((r) => (r.review_outcome_days ?? 0) > 0);
  if (!reported.length) return <BodyShort>Ingen rapporterte utfall fra Copilot-kodegjennomgang ennå.</BodyShort>;
  const evaluated = reported.reduce((sum, r) => sum + (r.review_comments_evaluated ?? 0), 0);
  const addressed = reported.reduce((sum, r) => sum + (r.review_comments_addressed ?? 0), 0);
  const complete = reported.every(
    (r) =>
      r.review_comments_evaluated != null &&
      r.review_comments_addressed != null &&
      r.review_comment_days === r.review_outcome_days
  );
  const rate =
    complete && evaluated > 0
      ? `${((100 * addressed) / evaluated).toLocaleString("nb-NO", { maximumFractionDigits: 1 })} %`
      : "Ikke tilgjengelig";
  const merged = reported.every((r) => r.review_merged_no_human_change_request != null)
    ? formatNumber(reported.reduce((sum, r) => sum + r.review_merged_no_human_change_request!, 0))
    : "Ikke tilgjengelig";
  const first = reported.flatMap((r) => (r.review_outcome_first_day ? [r.review_outcome_first_day] : [])).sort()[0];
  const last = reported
    .flatMap((r) => (r.review_outcome_last_day ? [r.review_outcome_last_day] : []))
    .sort()
    .at(-1);
  const breakdowns = new Map<
    string,
    { dimension: string; label: string; evaluated: number; addressed: number; complete: boolean }
  >();
  for (const row of reported.flatMap((r) => r.review_breakdowns ?? [])) {
    const key = JSON.stringify([row.dimension, row.label]);
    const total = breakdowns.get(key) ?? {
      dimension: row.dimension,
      label: row.label,
      evaluated: 0,
      addressed: 0,
      complete: true,
    };
    total.evaluated += row.evaluated ?? 0;
    total.addressed += row.addressed ?? 0;
    total.complete &&= row.evaluated != null && row.addressed != null;
    breakdowns.set(key, total);
  }
  const dimensions: Record<string, string> = {
    severity: "Alvorlighetsgrad",
    review_tier: "Gjennomgangsnivå",
    comment_type: "Kommentartype",
    customization: "Tilpasning",
  };
  return (
    <>
      <BodyShort weight="semibold">Oppfølging av Copilot-kommentarer</BodyShort>
      <BodyShort>
        Andel fulgt opp: {rate}.{" "}
        {complete
          ? `${formatNumber(addressed)} av ${formatNumber(evaluated)} evaluerte kommentarer.`
          : "Kommentartall mangler."}{" "}
        PR-er flettet uten senere endringskrav fra en person: {merged}.
      </BodyShort>
      <BodyShort>
        Gjelder {formatNumber(reported.length)} av {formatNumber(repositories.length)} synlige repoer som oppfyller
        aktivitetskravet.
        {first && last
          ? ` Rapporterte utfall fra ${first} til ${last}, med mulige hull.`
          : " Rapporteringsperiode mangler."}{" "}
        GitHub kunngjorde feltene 5. november 2026. Eldre rader uten feltene er ukjente, ikke null aktivitet. Summer
        dekker bare rapporterte felt. Manglende felt på enkelte dager kan gi ufullstendige summer.
      </BodyShort>
      <BodyShort>
        Kommentarene telles den dagen PR-en flettes. Fulgt opp omfatter brukte forslag, manuelle rettelser, hjelp fra
        Copilot og svar om en senere rettelse. Dette er ikke det samme som brukte kodeforslag. Kommentarer på
        bot-opprettede PR-er evalueres vanligvis ikke, men slike PR-er inngår i tallet uten senere endringskrav. Tallene
        måler ikke kodekvalitet eller produktivitet og erstatter ikke menneskelig gjennomgang.
      </BodyShort>
      {breakdowns.size > 0 && (
        <ul>
          {[...breakdowns.entries()].map(([key, row]) => (
            <li key={key}>
              {dimensions[row.dimension] ?? row.dimension}: {row.label}.{" "}
              {row.complete
                ? `${formatNumber(row.addressed)} av ${formatNumber(row.evaluated)} kommentarer fulgt opp.`
                : "Kommentartall mangler."}
            </li>
          ))}
        </ul>
      )}
    </>
  );
}
