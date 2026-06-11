"use client";

/**
 * Polished video overlay components for episode pills, badges, chips, and tags.
 *
 * Shared primitives for the unified video HUD.
 *
 * Design philosophy:
 * - Accent colors give each episode visual identity
 * - Semantic spacing with Nav tokens (where practical with inline styles)
 * - Proper visual hierarchy (pills → badges → chips → rules)
 * - Accessibility first (semantic HTML, contrast, aria-labels)
 * - Performance (pure components, no side effects)
 */

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
import { accentForEpisode } from "./video-accent";

// ============================================================================
// Accent Colors: Each episode gets a unique accent for visual identity
// ============================================================================

export { accentForEpisode };

// ============================================================================
// Visual Constants: Color tokens for overlay rendering
// ============================================================================

/** Scrim background (dark overlay with slight transparency) */
const SCRIM = "rgba(12, 14, 18, 0.72)";
/** Chip background (glassmorphism effect with white tint) */
const CHIP_BG = "rgba(255, 255, 255, 0.14)";
/** Chip text (high contrast white with transparency) */
const CHIP_TEXT = "rgba(255, 255, 255, 0.92)";
const MAX_CONTENT_ROWS = 4;

// ============================================================================
// Utility Functions: Classifying and styling overlay content
// ============================================================================

/**
 * Determine if a label is a single glyph (✓, !, etc).
 * Glyphs render differently—as small badges in the top rail instead of full rows.
 */
function isGlyph(labels: string[]): boolean {
  const label = labels[0] ?? "";
  return label.length <= 2;
}

export function isTopRailGlyph(overlay: OverlayComponent): boolean {
  return (overlay.kind === "badge" || overlay.kind === "glyph") && isGlyph(overlay.labels);
}

/**
 * Pick a leading icon based on the shape of the first label.
 * This provides visual scanning cues without spelling out full text.
 */
function iconForLabels(labels: string[], monospace?: boolean) {
  const first = labels[0] ?? "";
  if (first.startsWith("@")) return PersonIcon;
  if (first.startsWith("/")) return TerminalIcon;
  if (monospace && first.includes(".")) return FileCodeIcon;
  if (monospace) return TerminalIcon;
  return SparklesIcon;
}

// ============================================================================
// Episode Pill: Small badge showing episode number at top-left
// ============================================================================

/**
 * Episode pill: Small, polished badge for episode identity.
 *
 * - Monospace font for numeric consistency
 * - Accent background for visual pop
 * - Tight padding and small radius for compact feel
 * - High contrast dark text on bright background
 * - Accessibility: aria-label for screen readers
 */
export function EpisodePill({ label, accent, className = "" }: { label: string; accent: string; className?: string }) {
  return (
    <span
      className={`inline-flex h-7 items-center justify-center rounded-[0.4rem] text-[11px] font-medium shadow-sm backdrop-blur-sm ${className}`.trim()}
      style={{
        paddingInline: "var(--ax-space-8)",
        background: accent,
        color: "#10141a",
      }}
      title={`Episode ${label}`}
      aria-label={`Episode ${label}`}
    >
      {label}
    </span>
  );
}

// ============================================================================
// Glyph Badge: Small circular badges for status (✓, !, ★)
// ============================================================================

/**
 * Glyph badge: Renders single-character status indicators.
 *
 * - Circular container for compact visual impact
 * - Checkmark badges get accent background (highlight success/completion)
 * - Other glyphs get neutral background with high contrast
 * - Small icon or text rendering for visual clarity
 * - Accessibility: aria-label describes the status meaning
 */
export function GlyphBadge({ label, accent }: { label: string; accent: string }) {
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
      aria-label={`Status: ${label}`}
    >
      {isCheck ? <CheckmarkIcon aria-hidden fontSize="0.95rem" /> : label}
    </span>
  );
}

// ============================================================================
// Micro Chip: Inline content tag for labels/tokens
// ============================================================================

/**
 * Micro chip: Inline tag for content labels (rules, commands, agents).
 *
 * - Tiny font and tight padding for density in small viewports
 * - Glassmorphism background for layered appearance
 * - Optional monospace for code/technical content
 * - Nowrap to prevent mid-word breaks
 */
export function MicroChip({ label, mono }: { label: string; mono?: boolean }) {
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

// ============================================================================
// Row Icons: Leading icons for content rows with accent color
// ============================================================================

function RowIcon({ icon: Icon, accent }: { icon: typeof SparklesIcon; accent: string }) {
  return <Icon aria-hidden fontSize="0.85rem" style={{ color: accent, flexShrink: 0 }} />;
}

// ============================================================================
// Chip Row: Labeled group of tokens (e.g., "rules: A B C")
// ============================================================================

/**
 * Chip row: Groups related tags under an icon.
 *
 * - Leading icon provides visual category at a glance
 * - Wrapping bounded to 4 visible items max (then "+N" overflow indicator)
 * - Prevents layout overflow in 240px viewport
 * - Flexible gap management with HStack
 */
export function ChipRow({ overlay, accent }: { overlay: OverlayComponent; accent: string }) {
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

// ============================================================================
// Ladder Row: Ordered sequence with one step highlighted
// ============================================================================

/**
 * Ladder row: Renders step sequences (e.g., "setup › build › deploy › done").
 *
 * - Connected flow with chevrons between steps
 * - Active step highlighted with accent and bold
 * - Inactive steps use neutral background
 * - Semantic separator (chevron) shows progression
 */
export function LadderRow({ overlay, accent }: { overlay: OverlayComponent; accent: string }) {
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
              title={`Step: ${label}${active ? " (current)" : ""}`}
            >
              {label}
            </span>
          </HStack>
        );
      })}
    </HStack>
  );
}

// ============================================================================
// Counter Row: Before → After transition
// ============================================================================

/**
 * Counter row: Shows reduction/transformation (e.g., "3 → 1").
 *
 * - Arrow emphasizes the transition
 * - "Before" value in neutral text
 * - "After" value highlighted in accent
 * - Monospace for numeric alignment
 */
export function CounterRow({ overlay, accent }: { overlay: OverlayComponent; accent: string }) {
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

// ============================================================================
// Result Row: Outcome statement with checkmark
// ============================================================================

/**
 * Result row: Displays outcome/result with accent checkmark.
 *
 * - Checkmark icon signals success/completion
 * - Accent color on icon for visual emphasis
 * - Compact Detail component for proper typography
 * - Nowrap to keep result statement on one line
 */
export function ResultRow({ overlay, accent }: { overlay: OverlayComponent; accent: string }) {
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

// ============================================================================
// Rule Header: Headline with divider lines
// ============================================================================

/**
 * Rule header: Main takeaway with visual emphasis.
 *
 * - Divider lines on both sides make rule stand out as headline
 * - Accent-colored text (no background) for prominence
 * - Centered, high contrast
 * - Nowrap to keep rule on one line
 */
export function RuleHeader({ overlay, accent }: { overlay: OverlayComponent; accent: string }) {
  const label = overlay.labels[0] ?? "";
  return (
    <HStack gap="space-8" align="center" wrap={false}>
      <span style={{ flex: 1, height: 1, background: "rgba(255,255,255,0.22)" }} aria-hidden />
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
      <span style={{ flex: 1, height: 1, background: "rgba(255,255,255,0.22)" }} aria-hidden />
    </HStack>
  );
}

// ============================================================================
// Content Panel: Container with scrim background for all content rows
// ============================================================================

/** Ordering priority for content rows (determines what renders first) */
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

/**
 * Content panel: Scrim-backed container for overlay content.
 *
 * - Dark background with blur for legibility over video
 * - Vertical stack maintains responsive flow
 * - Max 4 rows to prevent overwhelming the player
 * - Positioned above title area to avoid collisions
 * - Uses HStack/VStack for semantic Aksel layout
 */
export function ContentPanel({ overlays, accent }: { overlays: OverlayComponent[]; accent: string }) {
  // Filter content overlays (excluding episode numbers and glyph badges rendered separately)
  const contentOverlays = overlays
    .filter((o) => o.kind !== "episode-number" && !isTopRailGlyph(o))
    .sort((a, b) => contentOrder(a.kind) - contentOrder(b.kind))
    .slice(0, MAX_CONTENT_ROWS);

  if (contentOverlays.length === 0) {
    return null;
  }

  return (
    <div
      style={{
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
            case "glyph":
              return <ResultRow key={i} overlay={overlay} accent={accent} />;
            case "chip":
            default:
              return <ChipRow key={i} overlay={overlay} accent={accent} />;
          }
        })}
      </VStack>
    </div>
  );
}
