// Plain DOM helpers for the three.js canvases. No three import here, so callers can probe
// before they load three.

export function hasWebGL2(): boolean {
  try {
    // three r186+ needs WebGL 2.
    return !!document.createElement("canvas").getContext("webgl2");
  } catch {
    return false;
  }
}

/** Reads an Aksel token, e.g. "--ax-bg-accent-strong", as a CSS colour string. Pass `el` to read it
 * inside a scoped theme, such as a section wrapped in <Theme theme="dark">. */
export function cssToken(name: string, fallback = "#7fb2ff", el: Element = document.documentElement): string {
  return getComputedStyle(el).getPropertyValue(name).trim() || fallback;
}

/** A crisp 128 px disc (fill) or ring (stroke) with a thin antialiased edge. Wrap it in a CanvasTexture. */
export function discSpriteCanvas(ring = false): HTMLCanvasElement {
  const c = document.createElement("canvas");
  c.width = c.height = 128;
  const ctx = c.getContext("2d");
  if (ctx) {
    ctx.beginPath();
    ctx.arc(64, 64, ring ? 56 : 62, 0, Math.PI * 2);
    if (ring) {
      ctx.lineWidth = 10;
      ctx.strokeStyle = "#fff";
      ctx.stroke();
    } else {
      ctx.fillStyle = "#fff";
      ctx.fill();
    }
  }
  return c;
}

/** A 64 px radial-gradient sprite for round, soft-edged points. Wrap it in a CanvasTexture. */
export function pointSpriteCanvas(): HTMLCanvasElement {
  const c = document.createElement("canvas");
  c.width = c.height = 64;
  const ctx = c.getContext("2d");
  if (ctx) {
    const g = ctx.createRadialGradient(32, 32, 0, 32, 32, 32);
    g.addColorStop(0, "#fff");
    g.addColorStop(0.5, "rgba(255,255,255,0.85)");
    g.addColorStop(1, "rgba(255,255,255,0)");
    ctx.fillStyle = g;
    ctx.fillRect(0, 0, 64, 64);
  }
  return c;
}
