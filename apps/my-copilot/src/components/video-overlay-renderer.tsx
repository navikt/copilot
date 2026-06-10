"use client";

import { HStack } from "@navikt/ds-react";
import type { OverlayComponent } from "@/lib/public-videos";
import { accentForEpisode, ContentPanel, EpisodePill, GlyphBadge } from "./video-overlay-components";

export { accentForEpisode };

type VideoOverlayRendererProps = {
  overlays?: OverlayComponent[];
};

// A "marker" badge is a short glyph (✓, !, ★ …) that belongs in the top rail
// next to the episode number rather than in the content panel.
function isGlyph(labels: string[]): boolean {
  return (labels[0] ?? "").length <= 2;
}

export function VideoOverlayRenderer({ overlays }: VideoOverlayRendererProps) {
  if (!overlays || overlays.length === 0) {
    return null;
  }

  const episode = overlays.find((o) => o.kind === "episode-number")?.labels[0];
  const accent = accentForEpisode(episode);
  const glyphBadges = overlays.filter((o) => o.kind === "badge" && isGlyph(o.labels));

  return (
    <div className="absolute inset-0 pointer-events-none" style={{ zIndex: 10 }} aria-hidden="true">
      {/* Top rail: episode number + status glyphs */}
      <HStack gap="space-4" align="center" wrap={false} style={{ position: "absolute", top: "0.5rem", left: "0.5rem" }}>
        {episode && <EpisodePill label={episode} accent={accent} />}
        {glyphBadges.map((b, i) => (
          <GlyphBadge key={i} label={b.labels[0] ?? ""} accent={accent} />
        ))}
      </HStack>

      {/* Content panel: scrim-backed stack above the title */}
      <div
        style={{
          position: "absolute",
          left: "0.5rem",
          right: "0.5rem",
          // Reserve extra space for 2-line titles + subtitle in card footer.
          bottom: "5.25rem",
        }}
      >
        <ContentPanel overlays={overlays} accent={accent} />
      </div>
    </div>
  );
}
