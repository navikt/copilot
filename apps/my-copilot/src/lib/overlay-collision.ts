// Collision detection and protected zone management for video overlays

export type ProtectedZone = {
  x: number;
  y: number;
  width: number;
  height: number;
  name: string;
};

export type OverlayBounds = {
  x: number;
  y: number;
  width: number;
  height: number;
};

/**
 * Defines protected zones that should never contain overlays.
 * Coordinates are relative to container (0-100 for positioning, pixels for sizes).
 */
export function getProtectedZones(containerWidth: number, containerHeight: number): ProtectedZone[] {
  // Play button: ~100px diameter circle at center
  const playButtonRadius = 50;
  const centerX = containerWidth / 2;
  const centerY = containerHeight / 2;

  return [
    {
      x: centerX - playButtonRadius,
      y: centerY - playButtonRadius,
      width: playButtonRadius * 2,
      height: playButtonRadius * 2,
      name: "play-button",
    },
    {
      x: 0,
      y: containerHeight - 60,
      width: containerWidth,
      height: 60,
      name: "title-area",
    },
  ];
}

/**
 * Check if an overlay would collide with any protected zone.
 */
export function isInProtectedZone(overlay: OverlayBounds, protectedZones: ProtectedZone[]): boolean {
  return protectedZones.some((zone) => {
    // Check for AABB (axis-aligned bounding box) collision
    return !(
      overlay.x + overlay.width <= zone.x ||
      overlay.x >= zone.x + zone.width ||
      overlay.y + overlay.height <= zone.y ||
      overlay.y >= zone.y + zone.height
    );
  });
}

/**
 * Find an alternative anchor position that avoids collisions.
 * Priority order: top-left, top-right, bottom-left, bottom-right.
 */
export function findAlternateAnchor(
  anchor: string,
  overlayWidth: number,
  overlayHeight: number,
  containerWidth: number,
  containerHeight: number,
  protectedZones: ProtectedZone[]
): string | null {
  const padding = 8;

  // Define candidate positions in priority order
  const candidates = [
    { anchor: "top-left", x: padding, y: padding },
    { anchor: "top-right", x: containerWidth - overlayWidth - padding, y: padding },
    { anchor: "bottom-left", x: padding, y: containerHeight - overlayHeight - padding },
    {
      anchor: "bottom-right",
      x: containerWidth - overlayWidth - padding,
      y: containerHeight - overlayHeight - padding,
    },
  ];

  for (const candidate of candidates) {
    const bounds: OverlayBounds = {
      x: candidate.x,
      y: candidate.y,
      width: overlayWidth,
      height: overlayHeight,
    };

    if (!isInProtectedZone(bounds, protectedZones)) {
      return candidate.anchor;
    }
  }

  return null;
}

/**
 * Estimate overlay dimensions based on kind and label length.
 * Used for collision detection.
 */
export function estimateOverlaySize(
  kind: string,
  labels: string[],
  isMobile: boolean
): { width: number; height: number } {
  const label = labels[0] ?? "";

  // Mobile uses smaller sizes
  const baseHeight = isMobile ? 24 : 32;
  const charWidth = isMobile ? 6 : 8;
  const padding = isMobile ? 8 : 12;

  switch (kind) {
    case "episode-number":
      return { width: Math.min(80, label.length * charWidth + padding * 2), height: baseHeight };
    case "badge":
      return { width: Math.min(70, label.length * charWidth + padding * 2), height: isMobile ? 24 : 28 };
    case "chip":
      return { width: Math.min(80, label.length * charWidth + padding * 2), height: baseHeight };
    case "counter":
      return { width: Math.min(60, label.length * charWidth + padding * 2), height: baseHeight };
    case "rule-pill":
      return { width: Math.min(120, label.length * charWidth + padding * 3), height: 24 };
    default:
      return { width: Math.min(100, label.length * charWidth + padding * 2), height: baseHeight };
  }
}
