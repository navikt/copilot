"use client";

import {
  ArrowRightIcon,
  CheckmarkIcon,
  ChevronRightIcon,
  FileCodeIcon,
  PersonIcon,
  SparklesIcon,
  TerminalIcon,
} from "@navikt/aksel-icons";
import { Detail, HStack, VStack } from "@navikt/ds-react";
import type { OverlayComponent } from "@/lib/public-videos";

type VideoOverlayRendererProps = {
  overlays?: OverlayComponent[];
};

// Per-episode accent. Gives every short its own visual identity while keeping a
// single shared visual language (scrim + chips + rows). Falls back to the first
// colour so unknown/short ids still render coherently.
const ACCENTS = ["#66d4cf", "#9af0a8", "#ffd485", "#c6a8ff", "#7cc7ff", "#ff9db1"] as const;

function accentForEpisode(episode: string | undefined): string {
  const n = Number.parseInt(episode ?? "", 10);
  if (Number.isFinite(n) && n > 0) {
    return ACCENTS[(n - 1) % ACCENTS.length];
  }
  return ACCENTS[0];
}

// A "marker" badge is a short glyph (✓, !, ★ …) that belongs in the top rail
// next to the episode number rather than in the content panel.
function isGlyph(labels: string[]): boolean {
  const label = labels[0] ?? "";
  return label.length <= 2;
}

// Pick a leading icon from the shape of the first label so code/commands/agents
// read at a glance without spelling everything out.
function iconForLabels(labels: string[], monospace?: boolean) {
  const first = labels[0] ?? "";
  if (first.startsWith("@")) return PersonIcon;
  if (first.startsWith("/")) return TerminalIcon;
  if (monospace && first.includes(".")) return FileCodeIcon;
  if (monospace) return TerminalIcon;
  return SparklesIcon;
}

const SCRIM = "rgba(12, 14, 18, 0.72)";
const CHIP_BG = "rgba(255, 255, 255, 0.14)";
const CHIP_TEXT = "rgba(255, 255, 255, 0.92)";
const MAX_CONTENT_ROWS = 4;

function EpisodePill({ label, accent }: { label: string; accent: string }) {
  return (
    <span
      className="font-mono"
      style={{
        background: accent,
        color: "#10141a",
        fontWeight: 700,
        fontSize: "0.78rem",
        lineHeight: 1,
        letterSpacing: "0.02em",
        padding: "0.28rem 0.5rem",
        borderRadius: "0.4rem",
      }}
    >
      {label}
    </span>
  );
}

function GlyphBadge({ label, accent }: { label: string; accent: string }) {
  const isCheck = label === "✓";
  return (
    <span
      className="inline-flex items-center justify-center"
      style={{
        width: "1.45rem",
        height: "1.45rem",
        borderRadius: "9999px",
        background: isCheck ? accent : "rgba(255,255,255,0.16)",
        color: isCheck ? "#10141a" : "#fff",
        fontSize: "0.7rem",
        fontWeight: 700,
      }}
      title={label}
    >
      {isCheck ? <CheckmarkIcon aria-hidden fontSize="0.95rem" /> : label}
    </span>
  );
}

function MicroChip({ label, mono }: { label: string; mono?: boolean }) {
  return (
    <span
      className={mono ? "font-mono" : ""}
      style={{
        background: CHIP_BG,
        color: CHIP_TEXT,
        fontSize: "0.62rem",
        lineHeight: 1.1,
        padding: "0.16rem 0.36rem",
        borderRadius: "0.3rem",
        whiteSpace: "nowrap",
      }}
      title={label}
    >
      {label}
    </span>
  );
}

function RowIcon({ icon: Icon, accent }: { icon: typeof SparklesIcon; accent: string }) {
  return <Icon aria-hidden fontSize="0.85rem" style={{ color: accent, flexShrink: 0 }} />;
}

// chip: a labelled group of tokens. Wrapping is bounded (max 4 visible, then +N)
// so a row never overflows the 240px viewport or clips text mid-word.
function ChipRow({ overlay, accent }: { overlay: OverlayComponent; accent: string }) {
  const max = 4;
  const visible = overlay.labels.slice(0, max);
  const hidden = overlay.labels.length - visible.length;
  const Icon = iconForLabels(overlay.labels, overlay.monospace);
  return (
    <HStack gap="space-4" align="center" wrap={false}>
      <RowIcon icon={Icon} accent={accent} />
      <HStack gap="space-4" align="center" wrap>
        {visible.map((label, i) => (
          <MicroChip key={i} label={label} mono={overlay.monospace} />
        ))}
        {hidden > 0 && <MicroChip label={`+${hidden}`} />}
      </HStack>
    </HStack>
  );
}

// ladder: an ordered sequence of modes/steps with one highlighted. Rendered as a
// connected flow (step › step › step) so the progression reads visually instead
// of as a comma-joined string.
function LadderRow({ overlay, accent }: { overlay: OverlayComponent; accent: string }) {
  const highlight = overlay.highlightIndex ?? -1;
  return (
    <HStack gap="space-2" align="center" wrap>
      {overlay.labels.map((label, i) => {
        const active = i === highlight;
        return (
          <HStack as="span" key={i} gap="space-2" align="center" wrap={false}>
            {i > 0 && <ChevronRightIcon aria-hidden fontSize="0.7rem" style={{ color: "rgba(255,255,255,0.5)" }} />}
            <span
              className="font-mono"
              style={{
                background: active ? accent : CHIP_BG,
                color: active ? "#10141a" : CHIP_TEXT,
                fontWeight: active ? 700 : 400,
                fontSize: "0.62rem",
                lineHeight: 1.1,
                padding: "0.16rem 0.36rem",
                borderRadius: "0.3rem",
                whiteSpace: "nowrap",
              }}
            >
              {label}
            </span>
          </HStack>
        );
      })}
    </HStack>
  );
}

// counter: a before → after transition (e.g. "3 → 1"). The arrow is emphasised
// so the reduction reads as the story of the clip.
function CounterRow({ overlay, accent }: { overlay: OverlayComponent; accent: string }) {
  const raw = overlay.labels[0] ?? "";
  const parts = raw.split(/→|->/).map((p) => p.trim());
  if (parts.length === 2) {
    return (
      <HStack gap="space-4" align="center" wrap={false}>
        <span className="font-mono" style={{ color: CHIP_TEXT, fontSize: "0.78rem", fontWeight: 700 }}>
          {parts[0]}
        </span>
        <ArrowRightIcon aria-hidden fontSize="0.85rem" style={{ color: accent }} />
        <span className="font-mono" style={{ color: accent, fontSize: "0.78rem", fontWeight: 700 }}>
          {parts[1]}
        </span>
      </HStack>
    );
  }
  return (
    <span className="font-mono" style={{ color: CHIP_TEXT, fontSize: "0.72rem", fontWeight: 700 }}>
      {raw}
    </span>
  );
}

// badge (long form, e.g. "patch + 2 linjer"): an outcome statement. Accent-tinted
// with a check so it reads as a result.
function ResultRow({ overlay, accent }: { overlay: OverlayComponent; accent: string }) {
  const label = overlay.labels[0] ?? "";
  return (
    <HStack gap="space-4" align="center" wrap={false}>
      <CheckmarkIcon aria-hidden fontSize="0.85rem" style={{ color: accent, flexShrink: 0 }} />
      <Detail textColor="subtle" style={{ color: CHIP_TEXT, whiteSpace: "nowrap" }} title={label}>
        {label}
      </Detail>
    </HStack>
  );
}

// rule-pill: the single "money" takeaway of the clip. Rendered as the panel
// header with divider lines so it reads as a headline.
function RuleHeader({ overlay, accent }: { overlay: OverlayComponent; accent: string }) {
  const label = overlay.labels[0] ?? "";
  return (
    <HStack gap="space-8" align="center" wrap={false}>
      <span style={{ flex: 1, height: 1, background: "rgba(255,255,255,0.22)" }} />
      <span
        style={{
          color: accent,
          fontSize: "0.66rem",
          fontWeight: 700,
          letterSpacing: "0.01em",
          whiteSpace: "nowrap",
        }}
        title={label}
      >
        {label}
      </span>
      <span style={{ flex: 1, height: 1, background: "rgba(255,255,255,0.22)" }} />
    </HStack>
  );
}

// Content rows are ordered by narrative importance, independent of the source
// `anchor` (which previously caused collisions). rule-pill leads as the headline.
const CONTENT_ORDER: Record<string, number> = {
  "rule-pill": 0,
  ladder: 1,
  counter: 2,
  chip: 3,
  badge: 4,
};

function contentOrder(kind: string): number {
  return CONTENT_ORDER[kind] ?? 99;
}

export function VideoOverlayRenderer({ overlays }: VideoOverlayRendererProps) {
  if (!overlays || overlays.length === 0) {
    return null;
  }

  const episode = overlays.find((o) => o.kind === "episode-number")?.labels[0];
  const accent = accentForEpisode(episode);

  const glyphBadges = overlays.filter((o) => o.kind === "badge" && isGlyph(o.labels));
  const contentOverlays = overlays
    .filter((o) => o.kind !== "episode-number" && !(o.kind === "badge" && isGlyph(o.labels)))
    .sort((a, b) => contentOrder(a.kind) - contentOrder(b.kind))
    .slice(0, MAX_CONTENT_ROWS);

  return (
    <div className="absolute inset-0 pointer-events-none" style={{ zIndex: 10 }} aria-hidden="true">
      {/* Top rail: episode number + status glyphs. Left-aligned to clear the
          host duration badge in the top-right corner. */}
      <HStack gap="space-4" align="center" wrap={false} style={{ position: "absolute", top: "0.5rem", left: "0.5rem" }}>
        {episode && <EpisodePill label={episode} accent={accent} />}
        {glyphBadges.map((b, i) => (
          <GlyphBadge key={i} label={b.labels[0] ?? ""} accent={accent} />
        ))}
      </HStack>

      {/* Content panel: a single scrim-backed stack above the title. Vertical
          flow removes all horizontal overflow and play-button collisions. */}
      {contentOverlays.length > 0 && (
        <div
          style={{
            position: "absolute",
            left: "0.5rem",
            right: "0.5rem",
            // Reserve extra space for 2-line titles + subtitle in card footer.
            bottom: "5.25rem",
            background: SCRIM,
            backdropFilter: "blur(6px)",
            WebkitBackdropFilter: "blur(6px)",
            borderRadius: "0.6rem",
            padding: "0.5rem 0.6rem",
          }}
        >
          <VStack gap="space-8">
            {contentOverlays.map((overlay, i) => {
              switch (overlay.kind) {
                case "rule-pill":
                  return <RuleHeader key={i} overlay={overlay} accent={accent} />;
                case "ladder":
                  return <LadderRow key={i} overlay={overlay} accent={accent} />;
                case "counter":
                  return <CounterRow key={i} overlay={overlay} accent={accent} />;
                case "badge":
                  return <ResultRow key={i} overlay={overlay} accent={accent} />;
                case "chip":
                default:
                  return <ChipRow key={i} overlay={overlay} accent={accent} />;
              }
            })}
          </VStack>
        </div>
      )}
    </div>
  );
}
