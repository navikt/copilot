import { Box, BodyShort, ReadMore } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableRow } from "@/components/aksel-table";
import { HeaderRow, linkClass } from "@/components/nav-pilot/doc-page";
import type { DelegationRange } from "@/lib/local-model-results";
import { formatDate } from "@/lib/format";
import type { Bar, LocalModel, Report, ReportIndex } from "@/lib/local-models";

// Charts on /innsikt/lokale-modeller. Plain HTML and CSS rather than chart.js: the
// marks are positioned in percent, so they scale to any width without JS, take
// Aksel tokens (light and dark) straight from CSS, and the timeline's points are
// real links. Every chart has a table or list with the same numbers.

const TASK_CLASS_LABEL: Record<string, string> = {
  "read-qa": "Svar og forklaringer om kode",
  "edit-single": "Endring i én fil",
  "edit-multi-mechanical": "Mekanisk endring over flere filer",
  "create-file": "Ny fil",
  debug: "Feilsøking",
};

// Classes where a failure goes unnoticed get the stricter x_silent bar (design.md §2.1).
const SILENT = new Set(["read-qa", "debug"]);

// One-sided z for the bar's confidence: 0.9 → 1.2816. Matches the lower bounds
// (`lb`) in capabilities.json, e.g. 27/27 → 0.943.
// ponytail: a lookup, not an inverse normal CDF; add one if the bar ever uses another confidence.
function zFor(confidence: number) {
  return ({ 0.9: 1.2816, 0.95: 1.6449 } as Record<number, number>)[confidence] ?? 1.2816;
}

/** Wilson score interval for k of n at the bar's confidence, both ends with the same z. */
export function wilson(k: number, n: number, confidence: number): [number, number] {
  const z = zFor(confidence);
  const p = k / n;
  const z2 = z * z;
  const centre = (p + z2 / (2 * n)) / (1 + z2 / n);
  const half = (z / (1 + z2 / n)) * Math.sqrt((p * (1 - p)) / n + z2 / (4 * n * n));
  return [Math.max(0, centre - half), Math.min(1, centre + half)];
}

const pct = (x: number) => `${Math.round(x * 100)} %`;
const pos = (x: number) => `${(x * 100).toFixed(2)}%`;
// Keeps a centred mark of width 2*inset inside the track at 0 % and 100 %.
const inTrack = (x: number, inset: string) => `clamp(${inset}, ${pos(x)}, calc(100% - ${inset}))`;

function Axis({ ticks, format, max }: { ticks: number[]; format: (t: number) => string; max: number }) {
  return (
    <div className="relative h-5 text-xs" style={{ color: "var(--ax-text-neutral-subtle)" }} aria-hidden>
      {ticks.map((t) => (
        <span
          key={t}
          className="absolute -translate-x-1/2 whitespace-nowrap first:translate-x-0 last:-translate-x-full"
          style={{ left: pos(t / max) }}
        >
          {format(t)}
        </span>
      ))}
    </div>
  );
}

const track = "relative h-7 rounded-sm";
const trackStyle = { background: "var(--ax-bg-neutral-softA)" };
const ink = { color: "var(--ax-text-neutral)" };

type IntervalRow = {
  cls: string;
  mode: string;
  k: number;
  n: number;
  lo: number;
  hi: number;
  bar: number;
  trusted: boolean;
};

function intervalRows(model: LocalModel, bar: Bar): IntervalRow[] {
  return Object.entries(model.classes).flatMap(([cls, c]) =>
    (
      [
        ["Delegert", c.delegate_k, c.delegate_n, c.delegate],
        ["Lokalt", c.local_k, c.local_n, c.local],
      ] as const
    )
      .filter(([, , n]) => n > 0)
      .map(([mode, k, n, verdict]) => {
        const [lo, hi] = wilson(k, n, bar.confidence);
        return {
          cls,
          mode,
          k,
          n,
          lo,
          hi,
          bar: SILENT.has(cls) ? bar.x_silent : bar.x_caught,
          trusted: verdict === "trusted",
        };
      })
  );
}

/** Success rate per task type with its Wilson interval, against the approval bar. */
export function IntervalChart({ model }: { model: LocalModel }) {
  if (!model.bar) return null;
  const rows = intervalRows(model, model.bar);
  if (!rows.length) return null;
  const conf = pct(model.bar.confidence);
  const span = pct(2 * model.bar.confidence - 1); // same z at both ends: one-sided conf is a two-sided 2*conf-1
  const summary = rows
    .map(
      (r) =>
        `${TASK_CLASS_LABEL[r.cls] ?? r.cls}, ${r.mode.toLowerCase()}: ${r.k} av ${r.n}, ${pct(r.lo)}–${pct(r.hi)}, krav ${pct(r.bar)}`
    )
    .join(". ");
  return (
    <figure className="flex flex-col gap-3">
      <figcaption>
        <BodyShort weight="semibold">Andel beståtte kjøringer med usikkerhet, {model.model.split("/").pop()}</BodyShort>
        <BodyShort size="small" textColor="subtle">
          Prikken er andelen beståtte kjøringer. Streken viser et {span}-intervall for hvor den sanne andelen kan ligge.
          Venstre ende er den nedre grensen kravet bruker ({conf} sikkerhet). Få kjøringer gir lang strek. En
          oppgavetype godkjennes først når venstre ende ligger til høyre for den stiplede kravlinjen.
        </BodyShort>
      </figcaption>
      <div role="img" aria-label={`Diagram. ${summary}.`} className="flex flex-col gap-2">
        {rows.map((r) => (
          <div key={`${r.cls}-${r.mode}`} className="grid gap-x-3 gap-y-1 sm:grid-cols-[14rem_1fr]">
            <BodyShort size="small" style={ink}>
              {TASK_CLASS_LABEL[r.cls] ?? r.cls}{" "}
              <span style={{ color: "var(--ax-text-neutral-subtle)" }}>
                · {r.mode.toLowerCase()} · {r.k} av {r.n}
              </span>
              {r.trusted && <strong> · godkjent</strong>}
            </BodyShort>
            <div className={track} style={trackStyle}>
              <div
                className="absolute inset-y-1 border-l-2 border-dashed"
                style={{ left: pos(r.bar), borderColor: "var(--ax-border-neutral-strong)" }}
              />
              <div
                className="absolute top-1/2 h-0.5 -translate-y-1/2"
                style={{ left: pos(r.lo), width: pos(r.hi - r.lo), background: "var(--ax-bg-accent-strong)" }}
              />
              {[r.lo, r.hi].map((x, i) => (
                <div
                  key={i}
                  className="absolute inset-y-2 w-0.5 -translate-x-1/2"
                  style={{ left: inTrack(x, "1px"), background: "var(--ax-bg-accent-strong)" }}
                />
              ))}
              <div
                className="absolute top-1/2 h-3 w-3 -translate-x-1/2 -translate-y-1/2 rounded-full border-2"
                style={{
                  left: inTrack(r.k / r.n, "0.375rem"),
                  background: "var(--ax-bg-accent-strong)",
                  borderColor: "var(--ax-bg-default)",
                }}
              />
            </div>
          </div>
        ))}
        <div className="grid sm:grid-cols-[14rem_1fr] sm:gap-x-3">
          <span />
          <Axis ticks={[0, 0.25, 0.5, 0.75, 1]} format={pct} max={1} />
        </div>
      </div>
      <BodyShort size="small" textColor="subtle">
        Stiplet linje er kravet: {pct(model.bar.x_caught)} av skymodellens andel, {pct(model.bar.x_silent)} for svar og
        feilsøking. Skymodellen besto alle kjøringene i disse målingene, så kravet vises som en fast andel. Manifestet
        oppgir ikke hvor mange kjøringer skymodellen hadde, så den har ingen egen strek.
      </BodyShort>
      <ReadMore header="Tallene som tabell" size="small">
        <div className="overflow-x-auto">
          <Table size="small">
            <HeaderRow
              cells={["Oppgavetype", "Utført", "Bestått", `${span}-intervall`, "Nedre grense", "Krav", "Vurdering"]}
            />
            <TableBody>
              {rows.map((r) => (
                <TableRow key={`${r.cls}-${r.mode}`}>
                  <TableDataCell>{TASK_CLASS_LABEL[r.cls] ?? r.cls}</TableDataCell>
                  <TableDataCell>{r.mode}</TableDataCell>
                  <TableDataCell>
                    {r.k} av {r.n}
                  </TableDataCell>
                  <TableDataCell>
                    {pct(r.lo)}–{pct(r.hi)}
                  </TableDataCell>
                  <TableDataCell>{pct(r.lo)}</TableDataCell>
                  <TableDataCell>{pct(r.bar)}</TableDataCell>
                  <TableDataCell>{r.trusted ? "Godkjent" : "Ikke godkjent"}</TableDataCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </ReadMore>
    </figure>
  );
}

const RANGE_MAX = 4;

function RangeTrack({ range, label }: { range: [number, number]; label: string }) {
  const [lo, hi] = range;
  const x = (v: number) => pos(v / RANGE_MAX);
  return (
    <div className="grid grid-cols-[8.5rem_1fr] items-center gap-2">
      <BodyShort size="small" style={ink}>
        {label} <span style={{ color: "var(--ax-text-neutral-subtle)" }}>{fmtRange(range)}</span>
      </BodyShort>
      <div className={track} style={trackStyle}>
        <div
          className="absolute inset-y-1 border-l-2 border-dashed"
          style={{ left: x(1), borderColor: "var(--ax-border-neutral-strong)" }}
        />
        <div
          className="absolute inset-y-2 rounded-sm"
          style={{
            left: x(lo),
            width: `max(4px, ${pos((hi - lo) / RANGE_MAX)})`,
            background: "var(--ax-bg-accent-strong)",
          }}
        />
      </div>
    </div>
  );
}

const fmtX = (v: number) => `${v.toLocaleString("nb-NO", { maximumFractionDigits: 2 })}×`;
const fmtRange = ([lo, hi]: [number, number]) =>
  `${lo.toLocaleString("nb-NO", { maximumFractionDigits: 2 })}–${fmtX(hi)}`;

/** Low–high cost and time per delegation level, as multiples of the cloud model alone. */
export function DelegationRangeChart({ ranges }: { ranges: DelegationRange[] }) {
  const summary = ranges
    .map(
      (r) =>
        `${r.level}: kostnad ${fmtX(r.cost[0])} til ${fmtX(r.cost[1])}, tid ${fmtX(r.time[0])} til ${fmtX(r.time[1])}`
    )
    .join(". ");
  return (
    <figure className="flex flex-col gap-3">
      <figcaption>
        <BodyShort weight="semibold">Kostnad og tid sammenlignet med skymodellen alene</BodyShort>
        <BodyShort size="small" textColor="subtle">
          Hver stolpe går fra laveste til høyeste måling. Stiplet linje er 1×, like mye som skymodellen alene.
        </BodyShort>
      </figcaption>
      <div role="img" aria-label={`Diagram. ${summary}.`} className="flex flex-col gap-4">
        {ranges.map((r) => (
          <div key={r.level} className="flex flex-col gap-1">
            <BodyShort size="small" weight="semibold" style={ink}>
              <code>{r.level}</code>
            </BodyShort>
            <RangeTrack range={r.cost} label="Kostnad" />
            <RangeTrack range={r.time} label="Tid" />
          </div>
        ))}
        <div className="grid grid-cols-[8.5rem_1fr] gap-2">
          <span />
          <Axis ticks={[0, 1, 2, 3, 4]} format={(t) => `${t}×`} max={RANGE_MAX} />
        </div>
      </div>
      <ul className="flex flex-col gap-1">
        {ranges.map((r) => (
          <li key={r.level}>
            <BodyShort size="small" textColor="subtle">
              <code>{r.level}</code>: målt {formatDate(r.measured)}.{" "}
              <a href={r.source} className={linkClass}>
                Se rapporten
              </a>
              .
            </BodyShort>
          </li>
        ))}
      </ul>
    </figure>
  );
}

type VerdictKey = Exclude<NonNullable<Report["verdict"]>, "none"> | "unset";

// Colour and shape both carry the verdict, and the list under the chart spells it out.
const VERDICT: Record<VerdictKey, { label: string; color: string; shape: string }> = {
  pass: { label: "Bestått", color: "var(--ax-bg-success-strong)", shape: "rounded-full" },
  fail: { label: "Ikke bestått", color: "var(--ax-bg-danger-strong)", shape: "rotate-45" },
  mixed: { label: "Blandet", color: "var(--ax-bg-warning-strong)", shape: "" },
  "not-yet": { label: "Ikke avgjort ennå", color: "var(--ax-bg-info-strong)", shape: "rounded-full" },
  unset: { label: "Uten vurdering", color: "var(--ax-bg-neutral-strong)", shape: "rounded-full scale-50" },
};

const vkey = (r: Report): VerdictKey => (r.verdict && r.verdict !== "none" ? r.verdict : "unset");

function Marker({ k, className = "" }: { k: VerdictKey; className?: string }) {
  const v = VERDICT[k];
  const hollow = k === "not-yet";
  return (
    <span
      aria-hidden
      className={`inline-block h-3 w-3 ${v.shape} ${className}`}
      style={hollow ? { border: `3px solid ${v.color}` } : { background: v.color }}
    />
  );
}

const day = (iso: string) => Date.parse(`${iso}T12:00:00Z`) / 86_400_000;
const short = (iso: string) =>
  new Date(`${iso}T12:00:00Z`).toLocaleDateString("nb-NO", { day: "numeric", month: "short", timeZone: "Europe/Oslo" });

const headline = (r: Report) => (r.headline ? `${r.headline.k} av ${r.headline.n}` : null);

/** Reports over time, coloured and shaped by verdict, each point a link. */
export function ReportTimeline({ index, listUrl }: { index: ReportIndex; listUrl: string }) {
  const { reports } = index;
  const first = day(reports[reports.length - 1].date);
  const span = Math.max(1, day(reports[0].date) - first);
  const byDate = new Map<string, Report[]>();
  for (const r of [...reports].reverse()) byDate.set(r.date, [...(byDate.get(r.date) ?? []), r]);
  const tallest = Math.max(...[...byDate.values()].map((g) => g.length));
  const used = (Object.keys(VERDICT) as VerdictKey[]).filter((k) => reports.some((r) => vkey(r) === k));
  const dates = [...byDate.keys()];
  const ticks = [dates[0], dates[Math.floor(dates.length / 2)], dates[dates.length - 1]];
  return (
    <figure className="flex flex-col gap-3">
      <figcaption>
        <BodyShort weight="semibold">Rapporter over tid</BodyShort>
        <BodyShort size="small" textColor="subtle">
          Hver prikk er én rapport. Rapporter fra samme dag ligger over hverandre. Velg en prikk for å åpne rapporten.
        </BodyShort>
      </figcaption>
      <ul className="flex flex-wrap gap-x-4 gap-y-1" aria-label="Tegnforklaring">
        {used.map((k) => (
          <li key={k} className="flex items-center gap-2">
            <Marker k={k} />
            <BodyShort size="small">
              {VERDICT[k].label} ({reports.filter((r) => vkey(r) === k).length})
            </BodyShort>
          </li>
        ))}
      </ul>
      <Box paddingInline="space-8">
        <div
          className="relative border-b"
          style={{ height: `${tallest * 1.25 + 0.5}rem`, borderColor: "var(--ax-border-neutral)" }}
        >
          {[...byDate.entries()].map(([date, group]) =>
            group.map((r, i) => (
              <a
                key={r.id}
                href={r.url}
                aria-label={`${formatDate(r.date)}: ${r.title}. ${VERDICT[vkey(r)].label}${headline(r) ? `, ${headline(r)}` : ""}.`}
                title={`${short(r.date)}: ${r.title}`}
                className="absolute flex h-5 w-5 -translate-x-1/2 items-center justify-center rounded-full focus-visible:outline-2"
                style={{ left: pos((day(date) - first) / span), bottom: `${i * 1.25 + 0.25}rem` }}
              >
                <Marker k={vkey(r)} />
              </a>
            ))
          )}
        </div>
        <div className="relative mt-1 h-5 text-xs" style={{ color: "var(--ax-text-neutral-subtle)" }} aria-hidden>
          {ticks.map((d, i) => (
            <span
              key={`${d}-${i}`}
              className={`absolute whitespace-nowrap ${i === 0 ? "" : i === 2 ? "-translate-x-full" : "hidden -translate-x-1/2 sm:inline"}`}
              style={{ left: pos((day(d) - first) / span) }}
            >
              {short(d)}
            </span>
          ))}
        </div>
      </Box>
      <BodyShort size="small" textColor="subtle">
        Rapportene er på engelsk. Bare rapporter som oppgir en vurdering, har farge.
      </BodyShort>
      <BodyShort size="small" weight="semibold" as="h3">
        Nyeste rapporter
      </BodyShort>
      <ul className="flex flex-col gap-2">
        {reports.slice(0, 8).map((r) => (
          <li key={r.id} className="grid grid-cols-[1rem_1fr] items-baseline gap-2">
            <Marker k={vkey(r)} className="translate-y-0.5" />
            <BodyShort size="small">
              <span style={{ color: "var(--ax-text-neutral-subtle)" }}>{short(r.date)}</span>{" "}
              <a href={r.url} className={linkClass}>
                {r.title}
              </a>{" "}
              <span style={{ color: "var(--ax-text-neutral-subtle)" }}>
                · {VERDICT[vkey(r)].label}
                {headline(r) && ` · ${headline(r)}`}
              </span>
            </BodyShort>
          </li>
        ))}
      </ul>
      <BodyShort size="small">
        <a href={listUrl} className={linkClass}>
          Alle {reports.length} rapporter
        </a>
      </BodyShort>
    </figure>
  );
}
