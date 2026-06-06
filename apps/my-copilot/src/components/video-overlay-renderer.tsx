"use client";

import { Tag, BodyShort } from "@navikt/ds-react";
import type { OverlayComponent } from "@/lib/public-videos";
import { getAnchorStyles } from "@/lib/overlay-positioning";

type VideoOverlayRendererProps = {
  overlays?: OverlayComponent[];
};

function EpisodeNumber({ labels, monospace }: { labels: string[]; monospace?: boolean }) {
  const label = labels[0] ?? "";
  return (
    <Tag
      variant="neutral"
      size="medium"
      className={`${monospace ? "font-mono" : ""}`}
      style={{ padding: "0.5rem 0.75rem" }}
    >
      {label}
    </Tag>
  );
}

function Badge({ labels }: { labels: string[] }) {
  const label = labels[0] ?? "";
  return (
    <Tag variant="info" size="small">
      {label}
    </Tag>
  );
}

function Chip({ labels, monospace }: { labels: string[]; monospace?: boolean }) {
  const label = labels[0] ?? "";
  return (
    <Tag variant="neutral" size="small" className={`${monospace ? "font-mono" : ""}`}>
      {label}
    </Tag>
  );
}

function Counter({ labels }: { labels: string[] }) {
  const label = labels[0] ?? "";
  return (
    <div className="bg-black/50 text-white px-2 py-1 rounded text-xs">
      <BodyShort size="small" className="text-white">
        {label}
      </BodyShort>
    </div>
  );
}

function RulePill({ labels }: { labels: string[] }) {
  const label = labels[0] ?? "";
  return (
    <div className="flex items-center gap-2 w-full">
      <div className="flex-1 h-px bg-black/30" />
      <BodyShort size="small" className="text-black/60 px-2 whitespace-nowrap">
        {label}
      </BodyShort>
      <div className="flex-1 h-px bg-black/30" />
    </div>
  );
}

function GenericOverlay({ labels }: { labels: string[] }) {
  return (
    <div className="bg-black/50 text-white px-2 py-1 rounded">
      <BodyShort size="small" className="text-white">
        {labels.join(", ")}
      </BodyShort>
    </div>
  );
}

export function VideoOverlayRenderer({ overlays }: VideoOverlayRendererProps) {
  if (!overlays || overlays.length === 0) {
    return null;
  }

  return (
    <>
      {overlays.map((overlay, index) => {
        const styles = getAnchorStyles(overlay.anchor);
        let content: React.ReactNode;

        switch (overlay.kind) {
          case "episode-number":
            content = <EpisodeNumber labels={overlay.labels} monospace={overlay.monospace} />;
            break;
          case "badge":
            content = <Badge labels={overlay.labels} />;
            break;
          case "chip":
            content = <Chip labels={overlay.labels} monospace={overlay.monospace} />;
            break;
          case "counter":
            content = <Counter labels={overlay.labels} />;
            break;
          case "rule-pill":
            content = <RulePill labels={overlay.labels} />;
            break;
          default:
            content = <GenericOverlay labels={overlay.labels} />;
        }

        return (
          <div key={`overlay-${index}`} style={styles} className="z-10">
            {content}
          </div>
        );
      })}
    </>
  );
}
