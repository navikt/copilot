"use client";

import { useEffect, useRef } from "react";
import * as THREE from "three";

export interface HeroCluster {
  name: string;
  count: number;
}

// One Aksel token per cluster, read at runtime so the theme decides the colour.
const TOKENS = [
  "--ax-bg-accent-strong",
  "--ax-bg-success-strong",
  "--ax-bg-warning-strong",
  "--ax-bg-meta-purple-strong",
];

function hasWebGL(): boolean {
  try {
    const c = document.createElement("canvas");
    return !!(c.getContext("webgl2") ?? c.getContext("webgl"));
  } catch {
    return false;
  }
}

function tokenColor(name: string): THREE.Color {
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  try {
    return new THREE.Color(v || "#7fb2ff");
  } catch {
    return new THREE.Color("#7fb2ff");
  }
}

export function HeroNetworkCanvas({ clusters }: { clusters: HeroCluster[] }) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const host = ref.current;
    if (!host || !hasWebGL()) return;

    const small = window.innerWidth < 640;
    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const renderer = new THREE.WebGLRenderer({ alpha: true, antialias: !small });
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    host.appendChild(renderer.domElement);

    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(45, 1, 0.1, 100);
    camera.position.z = 8;
    const group = new THREE.Group();
    scene.add(group);

    // Nodes: one cluster per catalogue type, sized by its real count (thinned on phones).
    const nodes: THREE.Vector3[] = [];
    const colors: number[] = [];
    const edges: [number, number][] = [];
    const hubs: number[] = [];
    clusters.forEach((cl, ci) => {
      const angle = (ci / clusters.length) * Math.PI * 2;
      const center = new THREE.Vector3(
        (small ? -2.7 : 0.5) + ci * (small ? 1.8 : 3.4),
        small ? (ci % 2 ? 0.1 : -2) : Math.sin(angle) * 0.8,
        (ci % 2) - 0.5
      );
      const n = Math.max(3, Math.round(small ? cl.count / 2 : cl.count));
      const first = nodes.length;
      hubs.push(first);
      for (let i = 0; i < n; i++) {
        const r = i === 0 ? 0 : (0.4 + Math.random() * 1.3) * (small ? 0.9 : 1);
        const dir = new THREE.Vector3().randomDirection().multiplyScalar(r);
        nodes.push(center.clone().add(dir));
        colors.push(ci, 0, 0);
        if (i > 0) edges.push([first + Math.floor(Math.random() * i), first + i]);
      }
    });
    hubs.forEach((h, i) => edges.push([h, hubs[(i + 1) % hubs.length]]));

    const nodeGeo = new THREE.BufferGeometry().setFromPoints(nodes);
    const colorAttr = new THREE.Float32BufferAttribute(colors, 3);
    nodeGeo.setAttribute("color", colorAttr);
    // colors holds the cluster index in r until the tokens are read; re-read on theme change.
    const clusterOf = colors.filter((_, i) => i % 3 === 0);
    const applyColors = () => {
      const palette = TOKENS.map(tokenColor);
      clusterOf.forEach((ci, i) => palette[ci % palette.length].toArray(colorAttr.array, i * 3));
      colorAttr.needsUpdate = true;
      lineMat.color.copy(palette[0]);
      pulseMat.color.copy(palette[0]).lerp(palette[2], 0.3);
    };

    // Round, soft-edged points: a tiny radial-gradient sprite.
    const spriteCanvas = document.createElement("canvas");
    spriteCanvas.width = spriteCanvas.height = 64;
    const ctx = spriteCanvas.getContext("2d");
    if (ctx) {
      const g = ctx.createRadialGradient(32, 32, 0, 32, 32, 32);
      g.addColorStop(0, "#fff");
      g.addColorStop(0.5, "rgba(255,255,255,0.85)");
      g.addColorStop(1, "rgba(255,255,255,0)");
      ctx.fillStyle = g;
      ctx.fillRect(0, 0, 64, 64);
    }
    const sprite = new THREE.CanvasTexture(spriteCanvas);
    const nodeMat = new THREE.PointsMaterial({
      size: small ? 0.5 : 0.36,
      vertexColors: true,
      map: sprite,
      transparent: true,
      depthWrite: false,
    });
    group.add(new THREE.Points(nodeGeo, nodeMat));

    const lineGeo = new THREE.BufferGeometry().setFromPoints(edges.flatMap(([a, b]) => [nodes[a], nodes[b]]));
    const lineMat = new THREE.LineBasicMaterial({ transparent: true, opacity: small ? 0.5 : 0.3 });
    group.add(new THREE.LineSegments(lineGeo, lineMat));

    // Pulses: points that travel along random edges.
    const pulseCount = small ? 8 : 18;
    const pulses = Array.from({ length: pulseCount }, () => ({
      edge: Math.floor(Math.random() * edges.length),
      t: Math.random(),
      speed: 0.15 + Math.random() * 0.25,
    }));
    const pulsePos = new Float32Array(pulseCount * 3);
    const pulseGeo = new THREE.BufferGeometry();
    pulseGeo.setAttribute("position", new THREE.BufferAttribute(pulsePos, 3));
    const pulseMat = new THREE.PointsMaterial({
      size: small ? 0.8 : 0.55,
      map: sprite,
      transparent: true,
      depthWrite: false,
      blending: THREE.AdditiveBlending,
    });
    group.add(new THREE.Points(pulseGeo, pulseMat));
    applyColors();

    const tmp = new THREE.Vector3();
    const pointer = { x: 0, y: 0 };
    let last = performance.now();
    let elapsed = 0;

    const draw = (dt: number) => {
      elapsed += dt;
      pulses.forEach((p, i) => {
        p.t += p.speed * dt;
        if (p.t >= 1) {
          p.t = 0;
          p.edge = Math.floor(Math.random() * edges.length);
        }
        const [a, b] = edges[p.edge];
        tmp.lerpVectors(nodes[a], nodes[b], p.t).toArray(pulsePos, i * 3);
      });
      pulseGeo.attributes.position.needsUpdate = true;
      group.rotation.y = Math.sin(elapsed * 0.05) * 0.2 + pointer.x * 0.15;
      group.rotation.x = Math.sin(elapsed * 0.03) * 0.15 + pointer.y * 0.1;
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
    let onScreen = true;
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

    // Gentle parallax on desktop only; passive, so touch scrolling is never blocked.
    const fine = window.matchMedia("(pointer: fine)").matches;
    const onPointer = (e: PointerEvent) => {
      pointer.x = e.clientX / window.innerWidth - 0.5;
      pointer.y = e.clientY / window.innerHeight - 0.5;
    };
    if (fine && !reduced) window.addEventListener("pointermove", onPointer, { passive: true });

    const themeMq = window.matchMedia("(prefers-color-scheme: dark)");
    const onTheme = () => {
      applyColors();
      if (reduced) draw(0);
    };
    themeMq.addEventListener("change", onTheme);
    const mo = new MutationObserver(onTheme);
    mo.observe(document.documentElement, { attributes: true, attributeFilter: ["class", "data-theme"] });

    if (reduced) draw(0);
    else sync();

    return () => {
      cancelAnimationFrame(raf);
      io.disconnect();
      mo.disconnect();
      themeMq.removeEventListener("change", onTheme);
      ro.disconnect();
      document.removeEventListener("visibilitychange", sync);
      window.removeEventListener("pointermove", onPointer);
      [nodeGeo, lineGeo, pulseGeo, nodeMat, lineMat, pulseMat, sprite].forEach((d) => d.dispose());
      renderer.dispose();
      renderer.domElement.remove();
    };
  }, [clusters]);

  // Decorative: hidden from assistive tech, never takes pointer events. The mask fades it out
  // behind the text on the left, so the hero text keeps its contrast.
  return (
    <div
      ref={ref}
      aria-hidden="true"
      className="absolute inset-0 pointer-events-none [&>canvas]:w-full [&>canvas]:h-full [mask-image:linear-gradient(90deg,transparent_15%,black_70%)] max-md:[mask-image:linear-gradient(180deg,rgb(0_0_0/0.2)_25%,black_60%)] opacity-70 max-md:opacity-100"
    />
  );
}
