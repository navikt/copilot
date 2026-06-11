"use client";

import { LinkIcon } from "@navikt/aksel-icons";
import { HStack } from "@navikt/ds-react";
import type { OverlayComponent } from "@/lib/public-videos";
import { isBodyContentVisible, type PlaybackState } from "@/lib/video-playback-machine";
import { ContentPanel, EpisodePill, GlyphBadge, isTopRailGlyph } from "./video-overlay-components";
import { HeaderLinkToken, HeaderToken, PlaybackControls } from "./video-card-chrome";

export function UnifiedVideoHUD({
  overlays,
  episodeLabel,
  accent,
  durationLabel,
  shareHref,
  shareTitle,
  playing,
  isActive,
  completed,
  showHud,
  playbackState,
  onTogglePlayback,
  onSeekBackward,
  onSeekForward,
  title,
}: {
  overlays?: OverlayComponent[];
  episodeLabel?: string;
  accent: string;
  durationLabel: string;
  shareHref: string;
  shareTitle: string;
  playing: boolean;
  isActive: boolean;
  completed: boolean;
  showHud: boolean;
  playbackState: PlaybackState;
  onTogglePlayback: () => void;
  onSeekBackward: () => void;
  onSeekForward: () => void;
  title: string;
}) {
  // Generate screen reader announcement based on playback state
  const getLiveRegionMessage = (): string => {
    if (playbackState === "playing") {
      return `Video spilles av: ${title}`;
    } else if (playbackState === "paused") {
      return `Video pauset: ${title}`;
    } else if (playbackState === "completed") {
      return `Video ferdig. Trykk replay for å se igjen.`;
    }
    return "";
  };

  // Extract glyph badges for top rail
  const glyphBadges = overlays?.filter(isTopRailGlyph) ?? [];

  // Determine if we should show playback controls
  const showPlaybackControls = !completed;

  return (
    <>
      {/* Screen reader announcements for video state changes (a11y) */}
      <div aria-live="polite" aria-atomic="true" className="sr-only" role="status">
        {getLiveRegionMessage()}
      </div>

      {/* Header layer: [Episode] [Duration] [Link] */}
      <div
        style={{
          position: "absolute",
          left: "8px",
          right: "8px",
          top: "8px",
          zIndex: 20,
        }}
        className={`transition-opacity duration-200 pointer-events-none ${showHud ? "opacity-100" : "opacity-0"}`}
      >
        <HStack justify="space-between" gap="space-2" align="center">
          <HStack gap="space-2" align="center" className="pointer-events-auto">
            {episodeLabel && <EpisodePill label={episodeLabel} accent={accent} />}
            <HeaderToken className="bg-black/70 text-white">{durationLabel}</HeaderToken>
            <HeaderLinkToken href={shareHref} ariaLabel={`Link video: ${shareTitle}`}>
              <LinkIcon aria-hidden fontSize="0.8rem" />
              <span className="text-[11px] leading-none">Link</span>
            </HeaderLinkToken>
          </HStack>
          <HStack gap="space-2" align="center">
            {glyphBadges.map((badge, i) => (
              <GlyphBadge key={i} label={badge.labels?.[0] ?? ""} accent={accent} />
            ))}
          </HStack>
        </HStack>
      </div>

      {/* Lower metadata layer: content pane above title area */}
      {(!isActive || isBodyContentVisible(playbackState)) && overlays && overlays.length > 0 && (
        <div
          style={{
            position: "absolute",
            left: "8px",
            right: "8px",
            bottom: "64px",
            zIndex: 20,
          }}
          className={`transition-opacity duration-200 pointer-events-none ${showHud ? "opacity-100" : "opacity-0"}`}
        >
          <ContentPanel overlays={overlays} accent={accent} />
        </div>
      )}

      {/* Playback layer: play/pause + skip buttons (center screen) */}
      {showPlaybackControls && (
        <div
          style={{
            position: "absolute",
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            zIndex: 30,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
          className={`pointer-events-none transition-opacity duration-200 ${
            showHud ? "opacity-100" : "opacity-0 pointer-events-none"
          }`}
        >
          <PlaybackControls
            ariaLabel={playing ? `Sett på pause: ${title}` : `Spill av video: ${title}`}
            playing={playing}
            showSkip={isActive && !completed}
            onToggle={onTogglePlayback}
            onSeekBackward={onSeekBackward}
            onSeekForward={onSeekForward}
            title={title}
          />
        </div>
      )}
    </>
  );
}
