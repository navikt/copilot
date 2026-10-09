"use client";

import { useEffect, type RefObject } from "react";
import * as THREE from "three";
import { cssToken, discSpriteCanvas, hasWebGL2 } from "@/lib/webgl";

export interface SceneContext {
  renderer: THREE.WebGLRenderer;
  scene: THREE.Scene;
  camera: THREE.PerspectiveCamera;
  /** The element the canvas sits in. Read tokens from it so a scoped theme applies. */
  host: HTMLDivElement;
  /** Phone-sized viewport: fewer objects, larger sprites. */
  small: boolean;
}

export interface SceneHooks {
  /** Advance by dt seconds, then the hook renders. dt is 0 for a still frame. */
  update: (dt: number) => void;
  /** Re-read the Aksel tokens. Called once at start and on every theme change. */
  applyColors: () => void;
  /** Free what setup created. The renderer is disposed by the hook. */
  dispose: () => void;
}

/** An Aksel token as a THREE.Color, read now, so call it again on theme change. */
export function tokenColor(name: string, el?: Element, fallback = "#7fb2ff"): THREE.Color {
  try {
    return new THREE.Color(cssToken(name, fallback, el));
  } catch {
    return new THREE.Color(fallback);
  }
}

/** A crisp, round sprite texture (disc or ring) with mipmaps. */
export function discTexture(ring = false): THREE.CanvasTexture {
  const t = new THREE.CanvasTexture(discSpriteCanvas(ring));
  t.minFilter = THREE.LinearMipmapLinearFilter;
  t.colorSpace = THREE.SRGBColorSpace;
  return t;
}

/**
 * Boots a decorative three.js scene in `ref`: WebGL 2 only, DPR capped at 2, paused offscreen
 * and in hidden tabs, a still frame under reduced motion, colours that follow the theme.
 * Renders nothing when WebGL 2 or the renderer is unavailable.
 */
export function useThreeScene(ref: RefObject<HTMLDivElement | null>, setup: (ctx: SceneContext) => SceneHooks) {
  useEffect(() => {
    const host = ref.current;
    if (!host || !hasWebGL2()) return;

    const small = window.innerWidth < 640;
    const motionMq = window.matchMedia("(prefers-reduced-motion: reduce)");
    let reduced = motionMq.matches;
    let renderer: THREE.WebGLRenderer;
    try {
      renderer = new THREE.WebGLRenderer({ alpha: true, antialias: true });
    } catch {
      return;
    }
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    host.appendChild(renderer.domElement);

    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(45, 1, 0.1, 100);
    const hooks = setup({ renderer, scene, camera, small, host });
    hooks.applyColors();

    const draw = (dt: number) => {
      hooks.update(dt);
      renderer.render(scene, camera);
    };

    const resize = () => {
      const { clientWidth: w, clientHeight: h } = host;
      if (!w || !h) return;
      renderer.setSize(w, h, false);
      camera.aspect = w / h;
      camera.updateProjectionMatrix();
      if (reduced) draw(0);
    };
    const ro = new ResizeObserver(resize);
    ro.observe(host);
    resize();

    let raf = 0;
    let last = 0;
    let onScreen = true;
    const loop = (now: number) => {
      // rAF time can trail the performance.now() taken in sync, so clamp at 0 too.
      draw(Math.min(Math.max((now - last) / 1000, 0), 0.1));
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
    const onTheme = () => {
      hooks.applyColors();
      if (reduced) draw(0);
    };
    themeMq.addEventListener("change", onTheme);
    const mo = new MutationObserver(onTheme);
    mo.observe(document.documentElement, { attributes: true, attributeFilter: ["class", "data-theme"] });

    const onMotion = () => {
      reduced = motionMq.matches;
      sync();
      if (reduced) draw(0);
    };
    motionMq.addEventListener("change", onMotion);

    if (reduced) draw(0);
    else sync();

    return () => {
      cancelAnimationFrame(raf);
      io.disconnect();
      mo.disconnect();
      ro.disconnect();
      motionMq.removeEventListener("change", onMotion);
      themeMq.removeEventListener("change", onTheme);
      document.removeEventListener("visibilitychange", sync);
      hooks.dispose();
      renderer.dispose();
      renderer.domElement.remove();
    };
    // setup is defined inline by each canvas and must only run once per mount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
}

/** Moves the camera back along z until a box of halfW × halfH around the origin fits the view. */
export function fitCamera(camera: THREE.PerspectiveCamera, halfW: number, halfH: number) {
  const t = Math.tan(THREE.MathUtils.degToRad(camera.fov / 2));
  camera.position.z = Math.max(halfW / (t * camera.aspect), halfH / t);
}

/** Disposes every geometry, material and texture under `root`. */
export function disposeTree(root: THREE.Object3D) {
  root.traverse((o) => {
    const m = o as THREE.Mesh;
    m.geometry?.dispose();
    const mats = Array.isArray(m.material) ? m.material : m.material ? [m.material] : [];
    mats.forEach((mat) => {
      (mat as THREE.PointsMaterial).map?.dispose();
      mat.dispose();
    });
  });
}

/** A text label as a sprite, `height` world units tall. Drawn white; tint it with material.color. */
export function textSprite(text: string, height: number): THREE.Sprite {
  const px = 48;
  const c = document.createElement("canvas");
  const ctx = c.getContext("2d")!;
  const font = `600 ${px}px ui-monospace, SFMono-Regular, Menlo, monospace`;
  ctx.font = font;
  c.width = Math.ceil(ctx.measureText(text).width) + 16;
  c.height = Math.round(px * 1.4);
  ctx.font = font;
  ctx.fillStyle = "#fff";
  ctx.textBaseline = "middle";
  ctx.fillText(text, 8, c.height / 2);
  const tex = new THREE.CanvasTexture(c);
  tex.colorSpace = THREE.SRGBColorSpace;
  tex.minFilter = THREE.LinearMipmapLinearFilter;
  const sprite = new THREE.Sprite(new THREE.SpriteMaterial({ map: tex, transparent: true, depthWrite: false }));
  sprite.scale.set((height * c.width) / c.height, height, 1);
  return sprite;
}
