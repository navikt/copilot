import { Box } from "@navikt/ds-react";
import { formatNumber } from "@/lib/format";
import type { VersionResult } from "@/lib/survey";
import { likert, type Segment } from "./[id]/likert";

// Diverging scale: red for disagree, gray neutral, green for agree; [fill, ink].
const FILLS = [
  ["var(--ax-bg-danger-strong)", "var(--ax-text-danger-contrast)"],
  ["var(--ax-bg-danger-moderate-pressed)", "var(--ax-text-neutral)"],
  ["var(--ax-bg-neutral-moderate-pressed)", "var(--ax-text-neutral)"],
  ["var(--ax-bg-success-moderate-pressed)", "var(--ax-text-neutral)"],
  ["var(--ax-bg-success-strong)", "var(--ax-text-success-contrast)"],
];
export const pct = (share: number) => `${Math.round(share * 100)} %`;

/**
 * One statement's bar: negatives left of the zero line, neutral split across it,
 * positives right. Decorative: the numbers are in the table under «Vis tall».
 */
export function LikertBar({ v, min, max, labels }: { v: VersionResult; min: number; max: number; labels: string[] }) {
  const { segments, mean } = likert(v, min, max);
  const mid = (min + max) / 2;
  const seg = (s: Segment, width: number, label = true) => {
    const i = Math.round(((s.value - min) / (max - min || 1)) * (FILLS.length - 1));
    const text = `${pct(s.share)} (${formatNumber(s.count)})`;
    return (
      <div
        key={s.value}
        data-testid={label ? `segment-${s.value}` : undefined}
        title={`${labels[s.value - min] ?? s.value}: ${text}`}
        className="@container h-7 overflow-hidden whitespace-nowrap text-xs leading-7 text-center"
        style={{ width: `${width * 100}%`, background: FILLS[i][0], color: FILLS[i][1] }}
      >
        {/* Narrow segments show their number in the tooltip and the table only. */}
        {label ? <span className="hidden @min-[4.5rem]:inline">{text}</span> : null}
      </div>
    );
  };
  const shown = segments.filter((s) => s.count);
  return (
    <div aria-hidden className="grid grid-cols-3 items-center gap-x-0">
      <div className="flex justify-end gap-[2px] border-r-2" style={{ borderColor: "var(--ax-border-neutral-strong)" }}>
        {shown.filter((s) => s.value <= mid).map((s) => seg(s, s.value === mid ? s.share / 2 : s.share))}
      </div>
      <div className="flex gap-[2px]">
        {shown
          .filter((s) => s.value >= mid)
          .map((s) => (s.value === mid ? seg(s, s.share / 2, false) : seg(s, s.share)))}
      </div>
      <Box paddingInline="space-12 space-0" className="flex items-center gap-2 text-xs">
        {mean === null ? null : <span>Snitt {mean.toFixed(1).replace(".", ",")}</span>}
      </Box>
    </div>
  );
}

export function Legend({ labels }: { labels: string[] }) {
  return (
    <div aria-hidden className="flex flex-wrap gap-x-4 gap-y-1 text-xs">
      {labels.map((l, i) => (
        <span key={l} className="flex items-center gap-1">
          <span
            className="inline-block h-3 w-3 rounded-sm"
            style={{ background: FILLS[Math.round((i / (labels.length - 1 || 1)) * (FILLS.length - 1))][0] }}
          />
          {l}
        </span>
      ))}
    </div>
  );
}
