import type React from "react";
import {
  findAlternateAnchor,
  getProtectedZones,
  estimateOverlaySize,
  isInProtectedZone,
  type OverlayBounds,
} from "./overlay-collision";

export function getAnchorStyles(
  anchor: string,
  options?: {
    kind?: string;
    labels?: string[];
    containerWidth?: number;
    containerHeight?: number;
  }
): React.CSSProperties | null {
  const baseStyles: React.CSSProperties = {
    position: "absolute",
  };

  const padding = "0.5rem";
  let resolvedAnchor = anchor;

  // Check for collisions if we have container dimensions
  if (options?.containerWidth && options?.containerHeight) {
    const isMobile = options.containerWidth < 400;
    const size = estimateOverlaySize(options.kind || "chip", options.labels || [], isMobile);
    const protectedZones = getProtectedZones(options.containerWidth, options.containerHeight);

    // Calculate overlay position based on anchor
    let overlayX = 0,
      overlayY = 0;

    switch (anchor) {
      case "top-left":
        overlayX = parseInt(padding) || 8;
        overlayY = parseInt(padding) || 8;
        break;
      case "top-right":
        overlayX = options.containerWidth - size.width - (parseInt(padding) || 8);
        overlayY = parseInt(padding) || 8;
        break;
      case "center-left":
        overlayX = parseInt(padding) || 8;
        overlayY = (options.containerHeight - size.height) / 2;
        break;
      case "center-right":
        overlayX = options.containerWidth - size.width - (parseInt(padding) || 8);
        overlayY = (options.containerHeight - size.height) / 2;
        break;
      case "center":
      case "center-left":
      case "center-right":
        // Avoid center anchors - redirect to corners
        resolvedAnchor = "top-left";
        overlayX = parseInt(padding) || 8;
        overlayY = parseInt(padding) || 8;
        break;
      case "bottom-left":
        overlayX = parseInt(padding) || 8;
        overlayY = options.containerHeight - size.height - (parseInt(padding) || 8);
        break;
      case "bottom-right":
        overlayX = options.containerWidth - size.width - (parseInt(padding) || 8);
        overlayY = options.containerHeight - size.height - (parseInt(padding) || 8);
        break;
      case "bottom-full":
        // Keep bottom-full as is (spans full width)
        break;
      default:
        overlayX = parseInt(padding) || 8;
        overlayY = parseInt(padding) || 8;
    }

    // Check for collision with protected zones
    if (anchor !== "bottom-full") {
      const overlayBounds: OverlayBounds = {
        x: overlayX,
        y: overlayY,
        width: size.width,
        height: size.height,
      };

      if (isInProtectedZone(overlayBounds, protectedZones)) {
        // Try to find alternate anchor
        const alternate = findAlternateAnchor(
          anchor,
          size.width,
          size.height,
          options.containerWidth,
          options.containerHeight,
          protectedZones
        );

        if (alternate) {
          resolvedAnchor = alternate;
        } else {
          // Can't find safe position - skip rendering
          return null;
        }
      }
    }
  }

  // Return styles based on resolved anchor
  switch (resolvedAnchor) {
    case "top-left":
      return { ...baseStyles, top: padding, left: padding };
    case "top-right":
      return { ...baseStyles, top: padding, right: padding };
    case "center-left":
      return {
        ...baseStyles,
        top: "50%",
        left: padding,
        transform: "translateY(-50%)",
      };
    case "center-right":
      return {
        ...baseStyles,
        top: "50%",
        right: padding,
        transform: "translateY(-50%)",
      };
    case "center":
      // Redirect center to top-left to avoid play button
      return { ...baseStyles, top: padding, left: padding };
    case "bottom-left":
      return { ...baseStyles, bottom: padding, left: padding };
    case "bottom-right":
      return { ...baseStyles, bottom: padding, right: padding };
    case "bottom-full":
      return { ...baseStyles, bottom: padding, left: padding, right: padding };
    default:
      return { ...baseStyles, top: padding, left: padding };
  }
}
