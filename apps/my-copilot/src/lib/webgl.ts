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

/** Reads an Aksel token, e.g. "--ax-bg-accent-strong", as a CSS colour string. */
export function cssToken(name: string, fallback = "#7fb2ff"): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback;
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

/**
 * Drives a hero canvas: runs the loop only while the host is on screen and the tab is visible,
 * draws one still frame under reduced motion, follows reduced-motion and theme changes, and
 * resizes with the host. Returns the cleanup.
 */
export function runCanvasLoop(
  host: HTMLElement,
  { draw, resize, onTheme }: { draw: (dt: number) => void; resize: (w: number, h: number) => void; onTheme: () => void }
): () => void {
  const motionMq = window.matchMedia("(prefers-reduced-motion: reduce)");
  let reduced = motionMq.matches;
  let raf = 0;
  let onScreen = true;
  let last = 0;
  const still = () => {
    if (reduced) draw(0);
  };

  const ro = new ResizeObserver(() => {
    const { clientWidth: w, clientHeight: h } = host;
    if (!w || !h) return;
    resize(w, h);
    still();
  });
  ro.observe(host);

  const loop = (now: number) => {
    draw(Math.min((now - last) / 1000, 0.1));
    last = now;
    raf = requestAnimationFrame(loop);
  };
  const sync = () => {
    const run = onScreen && !document.hidden && !reduced;
    if (run && !raf) {
      last = performance.now();
      raf = requestAnimationFrame(loop);
    } else if (!run && raf) {
      cancelAnimationFrame(raf);
      raf = 0;
    }
  };
  const io = new IntersectionObserver(([e]) => {
    onScreen = e.isIntersecting;
    sync();
  });
  io.observe(host);
  document.addEventListener("visibilitychange", sync);

  const themeMq = window.matchMedia("(prefers-color-scheme: dark)");
  const theme = () => {
    onTheme();
    still();
  };
  themeMq.addEventListener("change", theme);
  const mo = new MutationObserver(theme);
  mo.observe(document.documentElement, { attributes: true, attributeFilter: ["class", "data-theme"] });

  const onMotion = () => {
    reduced = motionMq.matches;
    sync();
    still();
  };
  motionMq.addEventListener("change", onMotion);
  sync();

  return () => {
    cancelAnimationFrame(raf);
    io.disconnect();
    mo.disconnect();
    ro.disconnect();
    motionMq.removeEventListener("change", onMotion);
    themeMq.removeEventListener("change", theme);
    document.removeEventListener("visibilitychange", sync);
  };
}
