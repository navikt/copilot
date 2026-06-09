"use client";

import type { HomepageVideo } from "@/lib/public-videos";

interface ResponsiveVideoPlayerProps {
  video: HomepageVideo;
  autoplay?: boolean;
}

/**
 * Converts API aspect ratio string to CSS aspect-ratio value.
 *   "9:16" → "9 / 16"
 *   "16 / 9" → "16 / 9"  (already correct, from fallback in public-videos.ts)
 */
function toCssAspectRatio(ar: string): string {
  return ar.includes(":") ? ar.replace(":", " / ") : ar;
}

/**
 * Returns true when height > width (portrait / vertical video).
 */
function isVertical(ar: string): boolean {
  const css = toCssAspectRatio(ar);
  const [w, h] = css.split("/").map((s) => parseFloat(s.trim()));
  return Number.isFinite(w) && Number.isFinite(h) && h > w;
}

/**
 * Responsive video player for the detail page.
 *
 * Renders a native <video> element sized to the video's true aspect ratio:
 * - 9:16 (vertical): capped at 360 px wide, centered in black column → tall portrait player
 * - 16:9 (landscape): full column width → standard widescreen player
 *
 * Uses native browser controls for maximum accessibility on all devices.
 * The outer div sets the exact aspect-ratio frame; the <video> fills it with
 * object-contain so content is never clipped or distorted.
 */
export function ResponsiveVideoPlayer({ video, autoplay = false }: ResponsiveVideoPlayerProps) {
  const cssAR = toCssAspectRatio(video.aspectRatio || "16:9");
  const vertical = isVertical(video.aspectRatio || "16:9");

  return (
    <div
      className="bg-black overflow-hidden"
      style={{
        aspectRatio: cssAR,
        width: "100%",
        maxWidth: vertical ? "360px" : undefined,
      }}
    >
      {}
      <video
        className="w-full h-full object-contain"
        controls
        playsInline
        poster={video.posterUrl}
        preload="metadata"
        muted={autoplay}
        autoPlay={autoplay}
        crossOrigin="anonymous"
      >
        <source src={video.playUrl} type="application/x-mpegURL" />
        {video.mp4Url && <source src={video.mp4Url} type="video/mp4" />}
        {video.captionsUrl && (
          <track kind="captions" src={video.captionsUrl} srcLang={video.language || "nb"} label="Teksting" />
        )}
        Din nettleser støtter ikke videoavspilling.
      </video>
    </div>
  );
}
