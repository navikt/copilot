"use client";

import { useEffect, useRef } from "react";
import * as THREE from "three";
import { cssToken, hasWebGL2, pointSpriteCanvas } from "@/lib/webgl";
import type { TermEdge } from "../ordliste/term-graph";
import type { Term } from "../ordliste/terms";

interface Props {
  terms: Term[];
  edges: TermEdge[];
  selected: number | null;
  onSelect: (i: number | null) => void;
  /** Set by the canvas: zooms by a factor (below 1 zooms in). */
  zoomRef: { current: ((factor: number) => void) | null };
}

function color(name: string, fallback: string): THREE.Color {
  try {
    return new THREE.Color(cssToken(name, fallback));
  } catch {
    return new THREE.Color(fallback);
  }
}

// Deterministic 3D force layout: start on a Fibonacci sphere, then springs on edges,
// repulsion between all pairs and a pull to the centre. ponytail: O(n²) per step, fine for ~50 terms.
function layout(n: number, edges: TermEdge[]): THREE.Vector3[] {
  const p = Array.from({ length: n }, (_, i) => {
    const y = 1 - (2 * (i + 0.5)) / n;
    const r = Math.sqrt(1 - y * y);
    const a = i * 2.39996;
    return new THREE.Vector3(Math.cos(a) * r, y, Math.sin(a) * r).multiplyScalar(3);
  });
  const f = p.map(() => new THREE.Vector3());
  const d = new THREE.Vector3();
  for (let step = 0; step < 300; step++) {
    f.forEach((v) => v.set(0, 0, 0));
    for (let i = 0; i < n; i++)
      for (let j = i + 1; j < n; j++) {
        d.subVectors(p[i], p[j]);
        const len2 = Math.max(d.lengthSq(), 0.01);
        d.multiplyScalar(0.6 / len2);
        f[i].add(d);
        f[j].sub(d);
      }
    for (const e of edges) {
      d.subVectors(p[e.to], p[e.from]);
      d.multiplyScalar((d.length() - 1.2) * 0.05);
      f[e.from].add(d);
      f[e.to].sub(d);
    }
    p.forEach((v, i) => v.add(f[i].addScaledVector(v, -0.02).clampLength(0, 0.3)));
  }
  // Centre, scale so most nodes fill a radius of 3, and pull loose outliers in.
  const mid = p.reduce((a, v) => a.add(v), new THREE.Vector3()).divideScalar(n);
  p.forEach((v) => v.sub(mid));
  const lens = p.map((v) => v.length()).sort((a, b) => a - b);
  const scale = 3 / (lens[Math.floor(n * 0.85)] || 1);
  return p.map((v) => v.multiplyScalar(scale).clampLength(0, 3.6));
}

function labelSprite(text: string, fill: string): THREE.Sprite {
  const c = document.createElement("canvas");
  const ctx = c.getContext("2d")!;
  const font = "600 40px system-ui, sans-serif";
  ctx.font = font;
  c.width = Math.ceil(ctx.measureText(text).width) + 8;
  c.height = 52;
  ctx.font = font;
  ctx.fillStyle = fill;
  ctx.textBaseline = "middle";
  ctx.fillText(text, 4, 26);
  const map = new THREE.CanvasTexture(c);
  map.colorSpace = THREE.SRGBColorSpace;
  const s = new THREE.Sprite(new THREE.SpriteMaterial({ map, transparent: true, depthWrite: false }));
  s.scale.set((c.width / c.height) * 0.32, 0.32, 1);
  s.center.set(0, 0.5);
  return s;
}

export function TermNetworkCanvas({ terms, edges, selected, onSelect, zoomRef }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const highlight = useRef<(i: number | null) => void>(() => {});
  const onSelectRef = useRef(onSelect);
  useEffect(() => {
    onSelectRef.current = onSelect;
  }, [onSelect]);

  useEffect(() => {
    const host = ref.current;
    if (!host || !hasWebGL2()) return;

    const motionMq = window.matchMedia("(prefers-reduced-motion: reduce)");
    let reduced = motionMq.matches;
    let renderer: THREE.WebGLRenderer;
    try {
      renderer = new THREE.WebGLRenderer({ alpha: true, antialias: true });
    } catch {
      return;
    }
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    const canvas = renderer.domElement;
    // pan-y: the browser keeps vertical scrolling; horizontal drags and pinches reach us.
    canvas.style.touchAction = "pan-y";
    host.appendChild(canvas);

    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(45, 1, 0.1, 100);
    camera.position.z = 9;
    const group = new THREE.Group();
    group.rotation.x = 0.3;
    scene.add(group);

    const pos = layout(terms.length, edges);
    const neighbours = terms.map(() => new Set<number>());
    edges.forEach((e) => {
      neighbours[e.from].add(e.to);
      neighbours[e.to].add(e.from);
    });

    const sprite = new THREE.CanvasTexture(pointSpriteCanvas());
    const nodes = pos.map((p, i) => {
      const s = new THREE.Sprite(new THREE.SpriteMaterial({ map: sprite, transparent: true, depthWrite: false }));
      const size = 0.2 + 0.1 * Math.sqrt(neighbours[i].size);
      s.scale.set(size, size, 1);
      s.position.copy(p);
      group.add(s);
      return s;
    });

    const lineGeo = new THREE.BufferGeometry().setFromPoints(edges.flatMap((e) => [pos[e.from], pos[e.to]]));
    const lineColors = new THREE.Float32BufferAttribute(new Float32Array(edges.length * 6), 3);
    lineGeo.setAttribute("color", lineColors);
    const lineMat = new THREE.LineBasicMaterial({ vertexColors: true, transparent: true, opacity: 0.7 });
    group.add(new THREE.LineSegments(lineGeo, lineMat));

    // Labels: always for the 8 best-connected terms, plus the selected or hovered term and its neighbours.
    const top = new Set(
      terms
        .map((_, i) => i)
        .sort((a, b) => neighbours[b].size - neighbours[a].size)
        .slice(0, 8)
    );
    let labels: (THREE.Sprite | undefined)[] = [];
    const clearLabels = () => {
      labels.forEach((l) => {
        l?.material.map?.dispose();
        l?.material.dispose();
        l?.removeFromParent();
      });
      labels = [];
    };
    let current: number | null = null;
    let hovered: number | null = null;
    let dirty = true;
    const paint = () => {
      const accent = color("--ax-bg-accent-strong", "#2a6ebb");
      const hot = color("--ax-bg-warning-strong", "#e8a33d");
      const dim = color("--ax-border-neutral-subtle", "#c0c4cc");
      const line = color("--ax-border-neutral", "#8a8f98");
      const text = cssToken("--ax-text-neutral", "#202733");
      const near = current === null ? null : neighbours[current];
      nodes.forEach((s, i) => {
        s.material.color.copy(current === null ? accent : i === current ? hot : near!.has(i) ? accent : dim);
      });
      edges.forEach((e, k) => {
        const on = current !== null && (e.from === current || e.to === current);
        const c = current === null ? line : on ? accent : dim;
        c.toArray(lineColors.array, k * 6);
        c.toArray(lineColors.array, k * 6 + 3);
      });
      lineColors.needsUpdate = true;

      const show = new Set(top);
      for (const f of [current, hovered]) if (f !== null) [f, ...neighbours[f]].forEach((i) => show.add(i));
      terms.forEach((t, i) => {
        if (show.has(i) && !labels[i]) {
          const l = labelSprite(t.term, text);
          // Start the label just outside the visible edge of the soft point.
          l.position.copy(pos[i]).add(new THREE.Vector3(nodes[i].scale.x * 0.45 + 0.05, 0, 0));
          group.add(l);
          labels[i] = l;
        }
        if (labels[i]) labels[i]!.visible = show.has(i);
      });
      dirty = true;
    };
    highlight.current = (i) => {
      current = i;
      paint();
    };
    paint();

    const resize = () => {
      const { clientWidth: w, clientHeight: h } = host;
      if (!w || !h) return;
      renderer.setSize(w, h, false);
      camera.aspect = w / h;
      camera.updateProjectionMatrix();
      dirty = true;
    };
    const ro = new ResizeObserver(resize);
    ro.observe(host);
    resize();

    // Interaction: drag rotates, wheel or pinch zooms, a tap picks the nearest node.
    let interacted = false;
    const pointers = new Map<number, { x: number; y: number }>();
    let start = { x: 0, y: 0 };
    let moved = 0;
    let pinch = 0;
    const zoom = (factor: number) => {
      camera.position.z = THREE.MathUtils.clamp(camera.position.z * factor, 4, 18);
      interacted = true;
      dirty = true;
    };
    zoomRef.current = zoom;
    const tmp = new THREE.Vector3();
    const pick = (cx: number, cy: number, radius: number): number | null => {
      const rect = canvas.getBoundingClientRect();
      let best: number | null = null;
      let bestDist = radius;
      nodes.forEach((s, i) => {
        tmp.copy(s.position).applyMatrix4(group.matrixWorld).project(camera);
        const x = ((tmp.x + 1) / 2) * rect.width + rect.left;
        const y = ((1 - tmp.y) / 2) * rect.height + rect.top;
        const dist = Math.hypot(x - cx, y - cy);
        if (dist < bestDist) {
          bestDist = dist;
          best = i;
        }
      });
      return best;
    };
    const onDown = (e: PointerEvent) => {
      pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
      if (pointers.size === 1) {
        start = { x: e.clientX, y: e.clientY };
        moved = 0;
      }
      if (pointers.size === 2) {
        const [a, b] = [...pointers.values()];
        pinch = Math.hypot(a.x - b.x, a.y - b.y);
        moved = Infinity;
      }
    };
    const onMove = (e: PointerEvent) => {
      const prev = pointers.get(e.pointerId);
      if (!prev) {
        if (e.pointerType === "mouse") {
          const h = pick(e.clientX, e.clientY, 20);
          if (h !== hovered) {
            hovered = h;
            paint();
          }
        }
        return;
      }
      const dx = e.clientX - prev.x;
      const dy = e.clientY - prev.y;
      pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
      interacted = true;
      if (pointers.size === 2) {
        const [a, b] = [...pointers.values()];
        const dist = Math.hypot(a.x - b.x, a.y - b.y);
        if (pinch > 0 && dist > 0) zoom(pinch / dist);
        pinch = dist;
        return;
      }
      moved = Math.max(moved, Math.hypot(e.clientX - start.x, e.clientY - start.y));
      group.rotation.y += dx * 0.008;
      // Touch only rotates sideways; vertical touch movement belongs to page scroll.
      if (e.pointerType === "mouse") group.rotation.x += dy * 0.008;
      dirty = true;
    };
    const onUp = (e: PointerEvent) => {
      const wasTap = pointers.size === 1 && moved < 8 && e.type === "pointerup";
      pointers.delete(e.pointerId);
      if (pointers.size < 2) pinch = 0;
      if (!wasTap) return;
      interacted = true;
      onSelectRef.current(pick(e.clientX, e.clientY, e.pointerType === "mouse" ? 20 : 32));
    };
    // A plain wheel scrolls the page. Ctrl/cmd + wheel zooms; trackpad pinch arrives as ctrl + wheel.
    const onWheel = (e: WheelEvent) => {
      if (!e.ctrlKey && !e.metaKey) return;
      e.preventDefault();
      zoom(1 + Math.sign(e.deltaY) * Math.min(Math.abs(e.deltaY), 100) * 0.002);
    };
    canvas.addEventListener("pointerdown", onDown);
    canvas.addEventListener("pointermove", onMove);
    canvas.addEventListener("pointerup", onUp);
    canvas.addEventListener("pointercancel", onUp);
    canvas.addEventListener("pointerleave", onUp);
    canvas.addEventListener("wheel", onWheel, { passive: false });

    let raf = 0;
    let onScreen = true;
    let last = performance.now();
    const loop = (now: number) => {
      const dt = Math.min((now - last) / 1000, 0.1);
      last = now;
      if (!reduced && !interacted) {
        group.rotation.y += dt * 0.12;
        dirty = true;
      }
      if (dirty) {
        renderer.render(scene, camera);
        dirty = false;
      }
      raf = requestAnimationFrame(loop);
    };
    const sync = () => {
      const run = onScreen && !document.hidden;
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
      clearLabels();
      paint();
    };
    themeMq.addEventListener("change", onTheme);
    const mo = new MutationObserver(onTheme);
    mo.observe(document.documentElement, { attributes: true, attributeFilter: ["class", "data-theme"] });
    const onMotion = () => {
      reduced = motionMq.matches;
    };
    motionMq.addEventListener("change", onMotion);
    sync();

    return () => {
      cancelAnimationFrame(raf);
      io.disconnect();
      mo.disconnect();
      ro.disconnect();
      themeMq.removeEventListener("change", onTheme);
      motionMq.removeEventListener("change", onMotion);
      document.removeEventListener("visibilitychange", sync);
      highlight.current = () => {};
      zoomRef.current = null;
      clearLabels();
      nodes.forEach((s) => s.material.dispose());
      [sprite, lineGeo, lineMat].forEach((d) => d.dispose());
      renderer.dispose();
      canvas.remove();
    };
  }, [terms, edges, zoomRef]);

  useEffect(() => highlight.current(selected), [selected]);

  // The glossary list below is the accessible version of this network.
  return (
    <div
      ref={ref}
      aria-hidden="true"
      className="h-[360px] md:h-[480px] w-full cursor-grab active:cursor-grabbing [&>canvas]:w-full [&>canvas]:h-full [&>canvas]:block"
    />
  );
}
