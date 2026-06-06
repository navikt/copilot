"use client";

import { Tag, BodyShort } from "@navikt/ds-react";
import type { OverlayComponent } from "@/lib/public-videos";

type HeaderOverlayGroupProps = {
  overlays: OverlayComponent[];
  isMobile: boolean;
};

function truncateText(text: string, maxChars: number): string {
  if (text.length > maxChars) {
    return text.slice(0, maxChars - 1) + "…";
  }
  return text;
}

function HeaderOverlayItem({ overlay, isMobile }: { overlay: OverlayComponent; isMobile: boolean }) {
  const label = overlay.labels[0] ?? "";
  const truncated = truncateText(label, 15);

  switch (overlay.kind) {
    case "episode-number":
      return (
        <Tag
          variant="neutral"
          size={isMobile ? "small" : "medium"}
          className={`${overlay.monospace ? "font-mono" : ""} truncate flex-shrink-0`}
          title={label}
        >
          {truncated}
        </Tag>
      );
    case "chip":
      return (
        <Tag
          variant="neutral"
          size="small"
          className={`${overlay.monospace ? "font-mono" : ""} truncate flex-shrink-0`}
          style={{ fontSize: isMobile ? "0.7rem" : undefined }}
          title={label}
        >
          {truncated}
        </Tag>
      );
    case "badge":
      return (
        <Tag variant="info" size={isMobile ? "small" : "medium"} title={label} className="truncate flex-shrink-0">
          {truncated}
        </Tag>
      );
    case "counter":
      return (
        <div
          className="bg-black/65 text-white rounded text-xs flex-shrink-0"
          style={{ padding: isMobile ? "0.4rem 0.5rem" : "0.5rem 0.5rem" }}
          title={label}
        >
          <BodyShort size="small" className="text-white">
            {truncated}
          </BodyShort>
        </div>
      );
    default:
      return (
        <div className="bg-black/65 text-white rounded text-xs flex-shrink-0" title={label}>
          <BodyShort size="small" className="text-white">
            {truncated}
          </BodyShort>
        </div>
      );
  }
}

export function HeaderOverlayGroup({ overlays, isMobile }: HeaderOverlayGroupProps) {
  if (!overlays.length) return null;

  // Separate episode number from other items
  const episodeOverlay = overlays.find((o) => o.kind === "episode-number");
  const otherOverlays = overlays.filter((o) => o.kind !== "episode-number");

  return (
    <div
      className="absolute top-0 left-0 pointer-events-none z-20"
      style={{
        padding: isMobile ? "0.5rem 0.5rem" : "0.75rem 1rem",
        background: "rgba(0, 0, 0, 0.65)",
        borderRadius: "0.375rem",
        maxWidth: isMobile ? "calc(100% - 1rem)" : "auto",
        margin: isMobile ? "0.5rem" : "0.75rem",
      }}
    >
      <div className="flex flex-col gap-1">
        {/* First line: Episode number + first row of items */}
        <div className="flex items-center gap-2 flex-wrap">
          {episodeOverlay && (
            <>
              <HeaderOverlayItem overlay={episodeOverlay} isMobile={isMobile} />
              {otherOverlays.length > 0 && <span className="text-white/40 text-xs">·</span>}
            </>
          )}
          {otherOverlays.slice(0, 2).map((overlay, idx) => (
            <div key={idx} className="flex items-center gap-2">
              <HeaderOverlayItem overlay={overlay} isMobile={isMobile} />
              {idx < Math.min(1, otherOverlays.length - 1) && <span className="text-white/40 text-xs">·</span>}
            </div>
          ))}
        </div>

        {/* Second line if there are more items */}
        {otherOverlays.length > 2 && (
          <div className="flex items-center gap-2 flex-wrap">
            {otherOverlays.slice(2).map((overlay, idx) => (
              <div key={idx} className="flex items-center gap-2">
                <HeaderOverlayItem overlay={overlay} isMobile={isMobile} />
                {idx < otherOverlays.length - 3 && <span className="text-white/40 text-xs">·</span>}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
