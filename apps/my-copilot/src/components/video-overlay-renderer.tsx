"use client";

import { Tag, BodyShort } from "@navikt/ds-react";
import { useRef, useEffect, useState } from "react";
import type { OverlayComponent } from "@/lib/public-videos";
import { getAnchorStyles } from "@/lib/overlay-positioning";
import { HeaderOverlayGroup } from "./header-overlay-group";

type VideoOverlayRendererProps = {
  overlays?: OverlayComponent[];
};

// Priority order for rendering (higher = more important, rendered first to appear on top)
const OVERLAY_PRIORITY: Record<string, number> = {
  "episode-number": 100,
  badge: 80,
  chip: 60,
  counter: 50,
  "rule-pill": 40,
};

function getOverlayPriority(kind: string): number {
  return OVERLAY_PRIORITY[kind] ?? 0;
}

function Badge({ labels, isMobile }: { labels: string[]; isMobile: boolean }) {
  const label = labels[0] ?? "";
  return (
    <Tag variant="info" size={isMobile ? "small" : "medium"} title={label} className="truncate">
      {label}
    </Tag>
  );
}

function Chip({ labels, monospace, isMobile }: { labels: string[]; monospace?: boolean; isMobile: boolean }) {
  const label = labels[0] ?? "";
  return (
    <Tag
      variant="neutral"
      size="small"
      className={`${monospace ? "font-mono" : ""} truncate`}
      style={{ fontSize: isMobile ? "0.7rem" : undefined }}
      title={label}
    >
      {label}
    </Tag>
  );
}

function Counter({ labels, isMobile }: { labels: string[]; isMobile: boolean }) {
  const label = labels[0] ?? "";
  return (
    <div
      className="bg-black/65 text-white rounded text-xs"
      style={{ padding: isMobile ? "0.4rem 0.5rem" : "0.5rem 0.5rem" }}
    >
      <BodyShort size="small" className="text-white">
        {label}
      </BodyShort>
    </div>
  );
}

function RulePill({ labels }: { labels: string[] }) {
  const label = labels[0] ?? "";
  return (
    <div className="flex items-center gap-2 w-full max-w-[120px]">
      <div className="flex-1 h-px bg-black/30" />
      <BodyShort size="small" className="text-black/60 px-1 whitespace-nowrap text-xs" title={label}>
        {label}
      </BodyShort>
      <div className="flex-1 h-px bg-black/30" />
    </div>
  );
}

function GenericOverlay({ labels, isMobile }: { labels: string[]; isMobile: boolean }) {
  return (
    <div className="bg-black/65 text-white rounded" style={{ padding: isMobile ? "0.4rem 0.5rem" : "0.5rem 0.5rem" }}>
      <BodyShort size="small" className="text-white text-xs">
        {labels.join(", ")}
      </BodyShort>
    </div>
  );
}

export function VideoOverlayRenderer({ overlays }: VideoOverlayRendererProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [containerSize, setContainerSize] = useState<{ width: number; height: number } | null>(null);
  const isMobile = containerSize ? containerSize.width < 400 : false;

  useEffect(() => {
    const handleResize = () => {
      if (containerRef.current) {
        setContainerSize({
          width: containerRef.current.offsetWidth,
          height: containerRef.current.offsetHeight,
        });
      }
    };

    handleResize();
    const observer = new ResizeObserver(handleResize);
    if (containerRef.current) {
      observer.observe(containerRef.current);
    }

    return () => observer.disconnect();
  }, []);

  if (!overlays || overlays.length === 0) {
    return null;
  }

  // Separate header overlays (top-left anchored) from other overlays
  const headerOverlays = overlays.filter((o) => o.anchor === "top-left");
  const otherOverlays = overlays.filter((o) => o.anchor !== "top-left");

  // Sort other overlays by priority (highest first, so they render on top)
  const sortedOtherOverlays = [...otherOverlays].sort(
    (a, b) => getOverlayPriority(b.kind) - getOverlayPriority(a.kind)
  );

  return (
    <div ref={containerRef} className="absolute inset-0 pointer-events-none" style={{ zIndex: 10 }} aria-hidden="true">
      {/* Header overlay group */}
      {headerOverlays.length > 0 && <HeaderOverlayGroup overlays={headerOverlays} isMobile={isMobile} />}

      {/* Other overlays */}
      {sortedOtherOverlays.map((overlay, index) => {
        const styles = getAnchorStyles(overlay.anchor, {
          kind: overlay.kind,
          labels: overlay.labels,
          containerWidth: containerSize?.width,
          containerHeight: containerSize?.height,
        });

        // Skip rendering if collision detection returns null
        if (!styles) {
          return null;
        }

        let content: React.ReactNode;

        switch (overlay.kind) {
          case "badge":
            content = <Badge labels={overlay.labels} isMobile={isMobile} />;
            break;
          case "chip":
            content = <Chip labels={overlay.labels} monospace={overlay.monospace} isMobile={isMobile} />;
            break;
          case "counter":
            content = <Counter labels={overlay.labels} isMobile={isMobile} />;
            break;
          case "rule-pill":
            content = <RulePill labels={overlay.labels} />;
            break;
          default:
            content = <GenericOverlay labels={overlay.labels} isMobile={isMobile} />;
        }

        return (
          <div key={`overlay-${index}`} style={styles} className="z-10">
            {content}
          </div>
        );
      })}
    </div>
  );
}
