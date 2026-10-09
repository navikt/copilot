"use client";

import { useEffect, useRef } from "react";
import * as THREE from "three";
import { cssToken, discSpriteCanvas, hasWebGL2 } from "@/lib/webgl";
import type { TermEdge } from "../ordliste/term-graph";
import { categories, type Category, type Term } from "../ordliste/terms";

interface Props {
  terms: Term[];
  edges: TermEdge[];
  selected: number | null;
  onSelect: (i: number | null) => void;
  /** Categories switched off in the legend. */
  hidden: Category[];
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

function crispTexture(c: HTMLCanvasElement): THREE.CanvasTexture {
  const t = new THREE.CanvasTexture(c);
  t.generateMipmaps = true;
  t.minFilter = THREE.LinearMipmapLinearFilter;
  t.magFilter = THREE.LinearFilter;
  t.colorSpace = THREE.SRGBColorSpace;
  return t;
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

const LABEL_PX = 20; // label height on screen, constant at every depth and zoom

// Text is drawn at devicePixelRatio (and for up to ~2.5x zoom), with a halo in the page background.
function labelSprite(text: string, fill: string, halo: string): { sprite: THREE.Sprite; aspect: number } {
  const px = Math.round(LABEL_PX * 0.75 * Math.min(window.devicePixelRatio || 1, 3));
  const c = document.createElement("canvas");
  const ctx = c.getContext("2d")!;
  const font = `600 ${px}px system-ui, sans-serif`;
  ctx.font = font;
  const pad = Math.ceil(px * 0.2);
  c.width = Math.ceil(ctx.measureText(text).width) + pad * 2;
  c.height = Math.ceil(px * 1.3);
  ctx.font = font;
  ctx.textBaseline = "middle";
  ctx.lineJoin = "round";
  ctx.lineWidth = px * 0.18;
  ctx.strokeStyle = halo;
  ctx.strokeText(text, pad, c.height / 2);
  ctx.fillStyle = fill;
  ctx.fillText(text, pad, c.height / 2);
  const s = new THREE.Sprite(
    new THREE.SpriteMaterial({ map: crispTexture(c), transparent: true, depthWrite: false, depthTest: false })
  );
  const aspect = c.width / c.height;
  s.center.set(0, 0.5);
  s.renderOrder = 2;
  return { sprite: s, aspect };
}

export function TermNetworkCanvas({ terms, edges, selected, onSelect, hidden, zoomRef }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const update = useRef<(selected: number | null, hidden: Category[]) => void>(() => {});
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
    const byDegree = terms.map((_, i) => i).sort((a, b) => neighbours[b].size - neighbours[a].size);

    const disc = crispTexture(discSpriteCanvas());
    const ringTex = crispTexture(discSpriteCanvas(true));
    const baseSize = terms.map((_, i) => 0.18 + 0.09 * Math.sqrt(neighbours[i].size));
    const nodes = pos.map((p, i) => {
      const s = new THREE.Sprite(new THREE.SpriteMaterial({ map: disc, transparent: true, depthWrite: false }));
      s.scale.setScalar(baseSize[i]);
      s.position.copy(p);
      group.add(s);
      return s;
    });
    // A ring in the text colour marks the selected node on top of its category colour.
    const ring = new THREE.Sprite(new THREE.SpriteMaterial({ map: ringTex, transparent: true, depthWrite: false }));
    ring.visible = false;
    ring.renderOrder = 1;
    group.add(ring);

    const linePos = new Float32Array(edges.length * 6);
    const lineGeo = new THREE.BufferGeometry();
    lineGeo.setAttribute("position", new THREE.BufferAttribute(linePos, 3));
    const lineColors = new THREE.Float32BufferAttribute(new Float32Array(edges.length * 6), 3);
    lineGeo.setAttribute("color", lineColors);
    const lineMat = new THREE.LineBasicMaterial({ vertexColors: true, transparent: true, opacity: 0.7 });
    group.add(new THREE.LineSegments(lineGeo, lineMat));

    let labels: ({ sprite: THREE.Sprite; aspect: number } | undefined)[] = [];
    const clearLabels = () => {
      labels.forEach((l) => {
        l?.sprite.material.map?.dispose();
        l?.sprite.material.dispose();
        l?.sprite.removeFromParent();
      });
      labels = [];
    };
    let current: number | null = null;
    let hovered: number | null = null;
    let off = new Set<Category>();
    const visible = (i: number) => !off.has(terms[i].category);
    let dirty = true;

    const paint = () => {
      const catColor = Object.fromEntries(categories.map((c) => [c.id, color(c.token, "#2a6ebb")]));
      const dim = color("--ax-border-neutral-subtle", "#c0c4cc");
      const line = color("--ax-border-neutral", "#8a8f98");
      const bg = color("--ax-bg-default", "#ffffff");
      const near = current === null ? null : neighbours[current];
      nodes.forEach((s, i) => {
        s.visible = visible(i);
        const lit = current === null || i === current || near!.has(i);
        // Unrelated nodes keep their category colour, faded toward the background.
        s.material.color.copy(catColor[terms[i].category]);
        if (!lit) s.material.color.lerp(bg, 0.7);
        s.scale.setScalar(baseSize[i] * (i === current ? 1.4 : 1));
      });
      if (current !== null && visible(current)) {
        ring.visible = true;
        ring.position.copy(pos[current]);
        ring.scale.setScalar(baseSize[current] * 2.1);
        ring.material.color.copy(color("--ax-text-neutral", "#202733"));
      } else ring.visible = false;
      edges.forEach((e, k) => {
        const show = visible(e.from) && visible(e.to);
        // A hidden edge collapses to a point.
        pos[e.from].toArray(linePos, k * 6);
        (show ? pos[e.to] : pos[e.from]).toArray(linePos, k * 6 + 3);
        const on = current !== null && (e.from === current || e.to === current);
        const c = current === null ? line : on ? color("--ax-border-strong", "#3b414b") : dim;
        c.toArray(lineColors.array, k * 6);
        c.toArray(lineColors.array, k * 6 + 3);
      });
      lineGeo.attributes.position.needsUpdate = true;
      lineColors.needsUpdate = true;
      dirty = true;
    };

    // Label candidates in priority order: selected, its neighbours, hovered and its neighbours,
    // then by degree. Labels stay inside the canvas. The selected node and its neighbours always get
    // a label, nudged to a free spot when they can; any other label that would overlap is skipped.
    const tmp = new THREE.Vector3();
    const placeLabels = () => {
      const order: number[] = [];
      const push = (i: number) => !order.includes(i) && visible(i) && order.push(i);
      const must = new Set<number>();
      if (current !== null) {
        // A hub can have 20+ neighbours: label the 8 best connected, plus the one under the pointer.
        const near = [...neighbours[current]].sort((a, b) => neighbours[b].size - neighbours[a].size);
        const labelled = near.slice(0, 8);
        if (hovered !== null && neighbours[current].has(hovered)) labelled.push(hovered);
        [current, ...labelled].forEach((i) => (push(i), must.add(i)));
      }
      if (hovered !== null && current === null) [hovered, ...neighbours[hovered]].forEach(push);
      byDegree.slice(0, 14).forEach(push);
      const { clientWidth: w, clientHeight: h } = host;
      const tanHalf = Math.tan(THREE.MathUtils.degToRad(camera.fov / 2));
      const placed: [number, number, number, number][] = [];
      const free = (r: [number, number, number, number]) =>
        !placed.some((p) => r[0] < p[2] && r[2] > p[0] && r[1] < p[3] && r[3] > p[1]);
      const show = new Set<number>();
      const lh = LABEL_PX;
      for (const i of order) {
        tmp.copy(pos[i]).applyMatrix4(group.matrixWorld);
        const depth = camera.position.z - tmp.z;
        if (depth <= 0.1) continue;
        const ppw = h / (2 * depth * tanHalf); // pixels per world unit at this depth
        tmp.project(camera);
        const nx = ((tmp.x + 1) / 2) * w;
        const ny = ((1 - tmp.y) / 2) * h;
        if (!labels[i]) {
          const halo = cssToken("--ax-bg-default", "#fff");
          labels[i] = labelSprite(terms[i].term, cssToken("--ax-text-neutral", "#202733"), halo);
          group.add(labels[i]!.sprite);
        }
        const lw = labels[i]!.aspect * lh;
        const gap = (baseSize[i] * 0.5 + 0.04) * ppw;
        // Offsets of the label's left edge and middle from the node, in screen pixels.
        const spots: [number, number][] = must.has(i)
          ? [0, -1, 1, -2, 2, -3, 3, -4, 4]
              .map((k) => k * lh)
              .flatMap((dy) => [[gap, dy] as [number, number], [-gap - lw, dy] as [number, number]])
          : [
              [gap, 0],
              [-gap - lw, 0],
            ];
        let chosen: [number, number, number, number] | null = null;
        let off: [number, number] = spots[0];
        for (const [dx, dy] of spots) {
          const x = THREE.MathUtils.clamp(nx + dx, 4, Math.max(4, w - lw - 14)); // ponytail: 14 px slack for a right-edge drift seen in swiftshader;
          const y = THREE.MathUtils.clamp(ny + dy, lh / 2, h - lh / 2);
          const r: [number, number, number, number] = [x, y - lh / 2, x + lw, y + lh / 2];
          if (free(r)) {
            chosen = r;
            off = [x - nx, y - ny];
            break;
          }
          if (!chosen && must.has(i)) {
            // Fallback for a must-show label: the first in-bounds spot, even if it overlaps.
            chosen = r;
            off = [x - nx, y - ny];
          }
        }
        if (!chosen) continue;
        placed.push(chosen);
        show.add(i);
        const wh = lh / ppw; // world height that gives LABEL_PX on screen
        const sprite = labels[i]!.sprite;
        sprite.position.copy(pos[i]);
        sprite.scale.set(labels[i]!.aspect * wh, wh, 1);
        sprite.center.set(-off[0] / lw, 0.5 + off[1] / lh);
      }
      labels.forEach((l, i) => l && (l.sprite.visible = show.has(i)));
    };

    update.current = (sel, hid) => {
      current = sel;
      off = new Set(hid);
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

    // Interaction: drag rotates, pinch, ctrl/cmd + wheel or the buttons zoom, a tap picks the nearest node.
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
    const pick = (cx: number, cy: number, radius: number): number | null => {
      const rect = canvas.getBoundingClientRect();
      let best: number | null = null;
      let bestDist = radius;
      nodes.forEach((s, i) => {
        if (!s.visible) return;
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
            dirty = true;
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
        group.updateMatrixWorld();
        placeLabels();
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
      update.current = () => {};
      zoomRef.current = null;
      clearLabels();
      nodes.forEach((s) => s.material.dispose());
      [disc, ringTex, ring.material, lineGeo, lineMat].forEach((d) => d.dispose());
      renderer.dispose();
      canvas.remove();
    };
  }, [terms, edges, zoomRef]);

  useEffect(() => update.current(selected, hidden), [selected, hidden]);

  // The glossary list below is the accessible version of this network.
  return (
    <div
      ref={ref}
      aria-hidden="true"
      className="h-[360px] md:h-[480px] w-full cursor-grab active:cursor-grabbing [&>canvas]:w-full [&>canvas]:h-full [&>canvas]:block"
    />
  );
}
