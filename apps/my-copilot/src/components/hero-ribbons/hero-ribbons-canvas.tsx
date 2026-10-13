"use client";

import { useEffect, useRef } from "react";
import * as THREE from "three";
import { cssToken, hasWebGL2, pointSpriteCanvas, runCanvasLoop } from "@/lib/webgl";

// Decorative only: the curves are made up and show no data.
const TOKENS = [
  "--ax-bg-accent-strong",
  "--ax-bg-meta-purple-strong",
  "--ax-bg-success-strong",
  "--ax-bg-warning-strong",
];
const SEGMENTS = 96;
const ROWS = 3; // edge, centre, edge: the edges fade to nothing, so each ribbon glows.

export function HeroRibbonsCanvas() {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const host = ref.current;
    if (!host || !hasWebGL2()) return;

    const small = window.innerWidth < 640;
    let renderer: THREE.WebGLRenderer;
    try {
      renderer = new THREE.WebGLRenderer({ alpha: true, antialias: !small });
    } catch {
      return;
    }
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    host.appendChild(renderer.domElement);

    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(45, 1, 0.1, 100);
    camera.position.z = 8;
    let halfW = 4;

    const count = small ? 3 : 4;
    const ribbons = Array.from({ length: count }, (_, i) => {
      const pos = new Float32Array(SEGMENTS * ROWS * 3);
      const col = new Float32Array(SEGMENTS * ROWS * 4);
      const idx: number[] = [];
      for (let s = 0; s < SEGMENTS - 1; s++)
        for (let r = 0; r < ROWS - 1; r++) {
          const a = s * ROWS + r;
          idx.push(a, a + ROWS, a + 1, a + 1, a + ROWS, a + ROWS + 1);
        }
      const geo = new THREE.BufferGeometry();
      geo.setAttribute("position", new THREE.BufferAttribute(pos, 3));
      geo.setAttribute("color", new THREE.BufferAttribute(col, 4));
      geo.setIndex(idx);
      const mat = new THREE.MeshBasicMaterial({
        vertexColors: true,
        transparent: true,
        depthWrite: false,
        side: THREE.DoubleSide,
        blending: THREE.AdditiveBlending,
      });
      scene.add(new THREE.Mesh(geo, mat));
      return {
        geo,
        mat,
        pos,
        col,
        base: (i / (count - 1 || 1) - 0.5) * (small ? 2.4 : 2.2),
        slope: 0.15 + Math.random() * 0.15, // gently rising: «trends over time»
        phase: Math.random() * Math.PI * 2,
        speed: 0.08 + Math.random() * 0.06,
        width: 0.3 + Math.random() * 0.2,
        centre: new Float32Array(SEGMENTS * 3),
      };
    });

    const applyColors = () => {
      const c = new THREE.Color();
      ribbons.forEach((rb, i) => {
        c.set(cssToken(TOKENS[i % TOKENS.length]));
        for (let s = 0; s < SEGMENTS; s++) {
          // Fade in and out at the ends of each ribbon.
          const along = Math.sin((s / (SEGMENTS - 1)) * Math.PI);
          for (let r = 0; r < ROWS; r++) {
            const a = (r === 1 ? 0.9 : 0) * along;
            c.toArray(rb.col, (s * ROWS + r) * 4);
            rb.col[(s * ROWS + r) * 4 + 3] = a;
          }
        }
        rb.geo.attributes.color.needsUpdate = true;
      });
    };
    applyColors();

    // Pulses: soft lights that travel along the ribbons.
    const sprite = new THREE.CanvasTexture(pointSpriteCanvas());
    const pulses = Array.from({ length: small ? 4 : 8 }, (_, i) => ({
      ribbon: i % count,
      t: Math.random(),
      speed: 0.05 + Math.random() * 0.05,
    }));
    const pulsePos = new Float32Array(pulses.length * 3);
    const pulseGeo = new THREE.BufferGeometry();
    pulseGeo.setAttribute("position", new THREE.BufferAttribute(pulsePos, 3));
    const pulseMat = new THREE.PointsMaterial({
      size: small ? 0.45 : 0.35,
      map: sprite,
      transparent: true,
      opacity: 0.8,
      depthWrite: false,
      blending: THREE.AdditiveBlending,
    });
    scene.add(new THREE.Points(pulseGeo, pulseMat));

    let elapsed = 0;
    const draw = (dt: number) => {
      elapsed += dt;
      for (const rb of ribbons) {
        const t = elapsed * rb.speed + rb.phase;
        for (let s = 0; s < SEGMENTS; s++) {
          const u = s / (SEGMENTS - 1);
          const x = (u - 0.5) * halfW * 2.4;
          const y = rb.base + rb.slope * (u - 0.5) * 4 + Math.sin(u * 4 + t) * 0.5 + Math.sin(u * 9 - t * 1.3) * 0.12;
          const z = Math.sin(u * 3 + t * 0.7) * 0.8;
          const twist = Math.sin(u * 5 + t) * rb.width;
          rb.centre[s * 3] = x;
          rb.centre[s * 3 + 1] = y;
          rb.centre[s * 3 + 2] = z;
          for (let r = 0; r < ROWS; r++) {
            const o = (r - 1) * rb.width;
            const k = (s * ROWS + r) * 3;
            rb.pos[k] = x;
            rb.pos[k + 1] = y + o;
            rb.pos[k + 2] = z + (r - 1) * twist;
          }
        }
        rb.geo.attributes.position.needsUpdate = true;
      }
      pulses.forEach((p, i) => {
        p.t = (p.t + p.speed * dt) % 1;
        const s = Math.round(p.t * (SEGMENTS - 1)) * 3;
        const c = ribbons[p.ribbon].centre;
        pulsePos[i * 3] = c[s];
        pulsePos[i * 3 + 1] = c[s + 1];
        pulsePos[i * 3 + 2] = c[s + 2];
      });
      pulseGeo.attributes.position.needsUpdate = true;
      renderer.render(scene, camera);
    };

    const stop = runCanvasLoop(host, {
      draw,
      resize: (w, h) => {
        renderer.setSize(w, h, false);
        camera.aspect = w / h;
        camera.updateProjectionMatrix();
        halfW = Math.tan(THREE.MathUtils.degToRad(camera.fov / 2)) * camera.position.z * camera.aspect;
      },
      onTheme: applyColors,
    });

    return () => {
      stop();
      ribbons.forEach((rb) => {
        rb.geo.dispose();
        rb.mat.dispose();
      });
      [pulseGeo, pulseMat, sprite].forEach((d) => d.dispose());
      renderer.dispose();
      renderer.domElement.remove();
    };
  }, []);

  // Decorative: hidden from assistive tech, never takes pointer events. The mask fades the
  // ribbons out behind the text, so the hero text keeps its contrast.
  return (
    <div
      ref={ref}
      aria-hidden="true"
      className="absolute inset-0 pointer-events-none [&>canvas]:w-full [&>canvas]:h-full [mask-image:linear-gradient(90deg,rgb(0_0_0/0.25)_20%,black_75%)] max-md:[mask-image:linear-gradient(180deg,rgb(0_0_0/0.3)_30%,black_90%)] opacity-80"
    />
  );
}
