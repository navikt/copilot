import { Alert, BodyShort, Box, Button, Detail, Heading, HStack, Link, Tag, VStack } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import { DocumentTitle, PrintButton } from "@/components/report-controls";
import { formatDate, formatNumber } from "@/lib/format";
import {
  channelName,
  type CompareQuestion,
  type Significance,
  type SurveyComparison,
  type WaveCounts,
  type WaveMeta,
  type WaveStat,
  UNDER_FIVE,
} from "@/lib/survey";
import { fill, Legend, LikertBar, pct } from "./likert-bar";
import { PrintHeader } from "./print-header";

// Oldest wave gray, newest blue: identity also in the text label on every row.
const WAVE_FILLS = [
  "var(--ax-bg-neutral-strong)",
  "var(--ax-bg-accent-strong)",
  "var(--ax-bg-warning-strong)",
  "var(--ax-bg-success-strong)",
];
// Oldest always gray, newest always blue; waves in between get the middle colours.
const waveFill = (i: number, count: number) =>
  i === 0 ? WAVE_FILLS[0] : i === count - 1 ? WAVE_FILLS[1] : WAVE_FILLS[1 + i];
// In print the newest wave is striped too, so it stands apart from the oldest in black and white.
const waveClass = (i: number, count: number) => (i > 0 && i === count - 1 ? " print-stripe" : "");

/** «Utviklerundersøkelsen våren 2026» → «Våren 2026». */
function shortName(title: string) {
  const s = title.replace(/^utviklerundersøkelsen\s+/i, "");
  return s.charAt(0).toUpperCase() + s.slice(1);
}

const num = (x: number, digits = 1) => x.toFixed(digits).replace(".", ",").replace("-", "−");
const signed = (x: number, digits = 1) => (x > 0 ? "+" : "") + num(x, digits);

const HIDDEN = "Færre enn 5 svarte på denne versjonen, så tallene vises ikke.";

/** The single version a wave asked, or undefined when it has none or several. */
const only = (w: WaveCounts | undefined): WaveStat | undefined =>
  w?.versions.length === 1 && w.versions[0].n !== null ? w.versions[0] : undefined;

function Delta({ label, value }: { label: string; value: string }) {
  return (
    <Tag size="xsmall" variant="neutral">
      {label} {value}
    </Tag>
  );
}

function BreakTag() {
  return (
    <Tag size="xsmall" variant="warning">
      Spørsmålet er endret: brudd i serien
    </Tag>
  );
}

function WaveLabel({
  wave,
  i,
  count,
  n,
  version,
}: {
  wave: WaveMeta;
  i: number;
  count: number;
  n: number | null;
  version?: number;
}) {
  return (
    <span className="flex items-center gap-2 text-xs">
      <span
        aria-hidden
        className={`inline-block h-3 w-3 rounded-sm${waveClass(i, count)}`}
        style={fill(waveFill(i, count))}
      />
      <span>
        {shortName(wave.title)}
        {version === undefined ? "" : `, versjon ${version}`} · n {n === null ? "–" : formatNumber(n)}
      </span>
    </span>
  );
}

/** Dumbbell: the mean of every statement per wave on one shared scale. No line across a version break. */
function MeanOverview({ waves, questions }: { waves: WaveMeta[]; questions: CompareQuestion[] }) {
  const rows = questions.flatMap((q) =>
    q.type === "matrix" && q.min !== undefined && q.max !== undefined
      ? (q.items ?? []).map((it) => ({ ...it, min: q.min!, max: q.max! }))
      : []
  );
  if (rows.length === 0) return null;
  const { min, max } = rows[0];
  const x = (v: number) => `${((v - min) / (max - min || 1)) * 100}%`;
  const ticks = Array.from({ length: max - min + 1 }, (_, i) => min + i);
  return (
    <section>
      <VStack gap="space-8">
        <Heading level="2" size="small">
          Snitt per påstand
        </Heading>
        <BodyShort size="small">
          1 er helt uenig, 5 er helt enig. Påstander som er endret mellom rundene, får ingen strek mellom punktene.
        </BodyShort>
        <HStack gap="space-16">
          {waves.map((w, i) => (
            <WaveLabel key={w.id} wave={w} i={i} count={waves.length} n={w.suppressed ? null : w.responses} />
          ))}
        </HStack>
        <div
          aria-hidden
          className="grid grid-cols-1 sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)] gap-x-4 gap-y-1 text-xs"
        >
          <span className="hidden sm:block" />
          <Box paddingInline="space-8">
            <div className="relative h-4">
              {ticks.map((t) => (
                <span key={t} className="absolute -translate-x-1/2" style={{ left: x(t) }}>
                  {t}
                </span>
              ))}
            </div>
          </Box>
          {rows.map((r) => {
            const means = r.waves.map((w) => only(w)?.mean);
            const shown = means.filter((m): m is number => m !== undefined);
            return (
              <div key={r.id} className="contents" data-testid={`dumbbell-${r.id}`}>
                <Box as="span" paddingBlock={{ xs: "space-8 space-0", sm: "space-4" }}>
                  {r.text}
                </Box>
                <Box paddingInline="space-8">
                  <div className="relative h-6">
                    {ticks.map((t) => (
                      <span
                        key={t}
                        className="absolute top-0 bottom-0 w-px"
                        style={{ left: x(t), ...fill("var(--ax-border-neutral-subtle)") }}
                      />
                    ))}
                    {!r.break && shown.length > 1 ? (
                      <span
                        data-testid="dumbbell-line"
                        className="absolute top-1/2 h-0.5 -translate-y-1/2"
                        style={{
                          left: x(Math.min(...shown)),
                          width: `calc(${x(Math.max(...shown))} - ${x(Math.min(...shown))})`,
                          ...fill("var(--ax-border-neutral)"),
                        }}
                      />
                    ) : null}
                    {means.map((m, i) =>
                      m === undefined ? null : (
                        <span
                          key={i}
                          title={`${shortName(waves[i].title)}: snitt ${num(m)}`}
                          className={`absolute top-1/2 h-3 w-3 -translate-x-1/2 -translate-y-1/2 rounded-full${waveClass(i, waves.length)}`}
                          style={{
                            left: x(m),
                            ...fill(waveFill(i, waves.length)),
                            boxShadow: "0 0 0 2px var(--ax-bg-default)",
                          }}
                        />
                      )
                    )}
                  </div>
                </Box>
              </div>
            );
          })}
        </div>
      </VStack>
    </section>
  );
}

const pValue = (p: number) => (p < 0.001 ? "under 0,001" : num(p, 3));

/** The first-to-last test from copilot-survey; it holds when p after Holm correction is under 0,05. */
function SignificanceTag({ sig }: { sig: Significance }) {
  if (sig.p === undefined || sig.p_holm === undefined) return null;
  const holds = sig.p_holm < 0.05;
  return (
    <Tag size="xsmall" variant={holds ? "success" : "neutral"}>
      p {pValue(sig.p)}, korrigert {pValue(sig.p_holm)}: {holds ? "står seg i testen" : "kan skyldes tilfeldigheter"}
    </Tag>
  );
}

/** Δ mean and Δ share agreeing, first wave to last, when both asked the same single version. */
function ScaleDeltas({ waves, broken, sig }: { waves: WaveCounts[]; broken: boolean; sig: Significance }) {
  if (broken) return <BreakTag />;
  const a = only(waves[0]);
  const b = only(waves[waves.length - 1]);
  if (a?.mean === undefined || b?.mean === undefined || a.agree === undefined || b.agree === undefined) return null;
  return (
    <HStack gap="space-8">
      <Delta label="Δ snitt" value={signed(b.mean - a.mean, 2)} />
      <Delta label="Δ enig" value={`${signed((b.agree - a.agree) * 100, 0)} pp`} />
      <SignificanceTag sig={sig} />
    </HStack>
  );
}

/** One statement: one diverging bar per wave, stacked on the same scale. */
function Statement({
  waves,
  text,
  counts,
  broken,
  sig,
  q,
}: {
  waves: WaveMeta[];
  text: string;
  counts: WaveCounts[];
  broken: boolean;
  sig: Significance;
  q: CompareQuestion;
}) {
  const min = q.min ?? 1;
  const max = q.max ?? 5;
  const labels = q.labels ?? Array.from({ length: max - min + 1 }, (_, i) => `${min + i}`);
  return (
    <VStack gap="space-4">
      <HStack gap="space-8" align="center" justify="space-between">
        <Heading level="3" size="xsmall">
          {text}
        </Heading>
        <ScaleDeltas waves={counts} broken={broken} sig={sig} />
      </HStack>
      {counts.map((w, i) =>
        w.versions.map((v) => (
          <div key={`${i}-${v.version}`}>
            <WaveLabel wave={waves[i]} i={i} count={waves.length} n={v.n} version={broken ? v.version : undefined} />
            {v.hidden ? (
              <Detail textColor="subtle">{HIDDEN}</Detail>
            ) : (
              <LikertBar v={v} min={min} max={max} labels={labels} />
            )}
          </div>
        ))
      )}
      <Excluded waves={waves} counts={counts} />
      <Numbers waves={waves} counts={counts} label={(v) => labels[Number(v) - min] ?? v} />
    </VStack>
  );
}

function Excluded({ waves, counts }: { waves: WaveMeta[]; counts: WaveCounts[] }) {
  const notes = counts.flatMap((w, i) =>
    w.excluded ? [`${shortName(waves[i].title)}: ${formatNumber(w.excluded)}`] : []
  );
  if (notes.length === 0) return null;
  return (
    <Detail textColor="subtle">
      Holdt utenfor fordi de ikke bruker KI-verktøy (som i siste runde): {notes.join(", ")}.
    </Detail>
  );
}

/** Choice and multi: per option one bar per wave, share of those who answered, and Δ pp. */
function Options({ waves, q }: { waves: WaveMeta[]; q: CompareQuestion }) {
  const counts = q.waves ?? [];
  const values = [...new Set(counts.flatMap((w) => w.versions.flatMap((v) => (v.options ?? []).map((o) => o.value))))];
  // One bar series per wave and version; a wave that asked several versions gets one per version.
  const series = counts.flatMap((w, i) => w.versions.map((v) => ({ i, v })));
  const share = (v: WaveStat | undefined, value: string) => {
    const c = v?.options?.find((o) => o.value === value)?.count;
    return v?.n && c != null ? { share: c / v.n, count: c } : undefined;
  };
  return (
    <VStack gap="space-8">
      {q.break ? <BreakTag /> : null}
      <HStack gap="space-16">
        {series.map(({ i, v }) => (
          <WaveLabel
            key={`${i}-${v.version}`}
            wave={waves[i]}
            i={i}
            count={waves.length}
            n={v.n}
            version={q.break || counts[i].versions.length > 1 ? v.version : undefined}
          />
        ))}
      </HStack>
      <div aria-hidden className="grid grid-cols-1 sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)] gap-x-4 gap-y-1 text-xs">
        {values.map((value) => {
          const first = share(only(counts[0]), value);
          const last = share(only(counts[counts.length - 1]), value);
          return (
            <div key={value} className="contents">
              <Box as="span" paddingBlock={{ xs: "space-8 space-0", sm: "space-0" }}>
                {value}
                {!q.break && first && last ? (
                  <Box
                    as="span"
                    paddingInline="space-8 space-0"
                    className="whitespace-nowrap"
                    style={{ color: "var(--ax-text-neutral-subtle)" }}
                  >
                    {signed((last.share - first.share) * 100, 0)} pp
                  </Box>
                ) : null}
              </Box>
              <div className="flex flex-col gap-[2px]">
                {series.map(({ i, v }) => {
                  const s = share(v, value);
                  return (
                    <div key={`${i}-${v.version}`} className="flex items-center gap-2">
                      <div
                        className={`h-3 rounded-r-sm${waveClass(i, waves.length)}`}
                        style={{ width: `${(s?.share ?? 0) * 75}%`, ...fill(waveFill(i, waves.length)) }}
                      />
                      <span className="whitespace-nowrap">
                        {s ? `${pct(s.share)} (${formatNumber(s.count)})` : "–"}
                      </span>
                    </div>
                  );
                })}
              </div>
            </div>
          );
        })}
      </div>
      <Excluded waves={waves} counts={counts} />
      <Numbers waves={waves} counts={counts} label={(v) => v} />
    </VStack>
  );
}

/** The accessible view: every count and share per wave and version. */
function Numbers({ waves, counts, label }: { waves: WaveMeta[]; counts: WaveCounts[]; label: (v: string) => string }) {
  const cols = counts.flatMap((w, i) =>
    w.versions.map((v) => ({
      v,
      name: `${shortName(waves[i].title)}${w.versions.length > 1 || counts.some((c) => c.versions.some((x) => x.version !== v.version)) ? `, versjon ${v.version}` : ""}`,
    }))
  );
  const values = [...new Set(cols.flatMap((c) => (c.v.options ?? []).map((o) => o.value)))];
  return (
    <details>
      <summary className="cursor-pointer text-sm">Vis tall</summary>
      <div className="overflow-x-auto">
        <Table size="small">
          <TableHeader>
            <TableRow>
              <TableHeaderCell scope="col">Svar</TableHeaderCell>
              {cols.map((c) => (
                <TableHeaderCell key={c.name} scope="col" align="right">
                  {c.name} (n {c.v.n === null ? "–" : formatNumber(c.v.n)})
                </TableHeaderCell>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {values.map((value) => (
              <TableRow key={value}>
                <TableDataCell>{label(value)}</TableDataCell>
                {cols.map((c) => {
                  const count = c.v.options?.find((o) => o.value === value)?.count;
                  return (
                    <TableDataCell key={c.name} align="right">
                      {count == null ? "–" : `${formatNumber(count)} (${c.v.n ? pct(count / c.v.n) : "–"})`}
                    </TableDataCell>
                  );
                })}
              </TableRow>
            ))}
            {cols.some((c) => c.v.mean !== undefined) ? (
              <TableRow>
                <TableDataCell>Snitt</TableDataCell>
                {cols.map((c) => (
                  <TableDataCell key={c.name} align="right">
                    {c.v.mean === undefined ? "–" : num(c.v.mean)}
                  </TableDataCell>
                ))}
              </TableRow>
            ) : null}
          </TableBody>
        </Table>
      </div>
    </details>
  );
}

function Waves({ waves, waveHref }: { waves: WaveMeta[]; waveHref: (id: string) => string }) {
  const channels = waves.some((w) => w.channels);
  return (
    <div className="overflow-x-auto">
      <Table size="small">
        <caption className="text-left">
          <BodyShort size="small">Rundene som sammenlignes</BodyShort>
        </caption>
        <TableHeader>
          <TableRow>
            <TableHeaderCell scope="col">Runde</TableHeaderCell>
            <TableHeaderCell scope="col">Periode</TableHeaderCell>
            <TableHeaderCell scope="col" align="right">
              Svar
            </TableHeaderCell>
            {channels ? <TableHeaderCell scope="col">Besvart via</TableHeaderCell> : null}
          </TableRow>
        </TableHeader>
        <TableBody>
          {waves.map((w) => (
            <TableRow key={w.id}>
              <TableDataCell>
                <Link href={waveHref(w.id)}>{w.title}</Link>
              </TableDataCell>
              <TableDataCell>
                {formatDate(w.starts)}–{formatDate(w.ends)}
              </TableDataCell>
              <TableDataCell align="right">{w.suppressed ? UNDER_FIVE : formatNumber(w.responses)}</TableDataCell>
              {channels ? (
                <TableDataCell>
                  {Object.entries(w.channels ?? {})
                    .sort((a, b) => b[1] - a[1])
                    .map(([c, n]) => `${channelName(c)}: ${formatNumber(n)}`)
                    .join(", ") || "–"}
                </TableDataCell>
              ) : null}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

/** The PDF's first lines: one per wave. Print only. */
export function ComparisonPrintHeader({ waves }: { waves: WaveMeta[] }) {
  return (
    <PrintHeader
      title={`Sammenligning: ${waves.map((w) => shortName(w.title)).join(" og ")}`}
      lines={waves.map(
        (w) =>
          `${w.title}: ${formatDate(w.starts)}–${formatDate(w.ends)} · ${w.suppressed ? UNDER_FIVE : formatNumber(w.responses)} svar`
      )}
    />
  );
}

/**
 * Waves side by side, question by question, as copilot-survey's compare gives them.
 * Channels are shown when present; the published comparison has none.
 */
export function ComparisonView({
  data,
  csvHref,
  waveHref,
  groups,
}: {
  data: SurveyComparison;
  csvHref: string;
  waveHref: (id: string) => string;
  /** Which answers the statements count in each wave. */
  groups: string;
}) {
  const { surveys: waves, questions } = data;
  const title = waves.map((w) => shortName(w.title)).join(" og ");
  return (
    <VStack gap="space-32">
      <DocumentTitle title={`Sammenligning ${title}`} />
      {/* Here, not in the hero: only a comparison that loaded is worth saving. */}
      <HStack gap="space-8" className="print-hidden">
        <PrintButton>Lagre som PDF</PrintButton>
        <Button as="a" href={csvHref} size="small" variant="secondary">
          Last ned sammenligning (CSV)
        </Button>
      </HStack>
      <Waves waves={waves} waveHref={waveHref} />
      <Alert variant="info" size="small">
        Rundene ble samlet inn på ulike måter{waves.some((w) => w.channels) ? " (se «Besvart via»)" : ""}. En forskjell
        mellom rundene kan komme av metoden og ikke bare av endring over tid. {groups} Vi tester hver påstand mellom
        første og siste runde (Mann–Whitney). «Står seg i testen» betyr at forskjellen er statistisk signifikant
        (korrigert p under 0,05), også etter at vi har tatt hensyn til at vi tester mange påstander (Holm).
      </Alert>
      {waves.some((w) => w.suppressed) ? (
        <Alert variant="info">En av rundene har under 5 svar. Den vises uten tall.</Alert>
      ) : null}
      <MeanOverview waves={waves} questions={questions} />
      {questions.map((q, qi) => (
        <section key={q.id} className={qi === 0 ? "print-break" : undefined}>
          <VStack gap="space-16">
            <Heading level="2" size="small">
              {q.text}
            </Heading>
            {q.type === "matrix" ? (
              <>
                {q.labels ? <Legend labels={q.labels} /> : null}
                {q.items?.map((it) => (
                  <Statement
                    key={it.id}
                    waves={waves}
                    text={it.text}
                    counts={it.waves}
                    broken={it.break}
                    sig={it}
                    q={q}
                  />
                ))}
              </>
            ) : q.type === "scale" ? (
              <Statement waves={waves} text={q.text} counts={q.waves ?? []} broken={!!q.break} sig={q} q={q} />
            ) : (
              <Options waves={waves} q={q} />
            )}
          </VStack>
        </section>
      ))}
    </VStack>
  );
}
