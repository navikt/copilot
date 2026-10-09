"use client";

import { useRef } from "react";
import * as THREE from "three";
import { discTexture, disposeTree, tokenColor, useThreeScene } from "./use-three-scene";

/** /reisen hero: a gentle path with one glowing point per phase. Scroll moves the camera along it. */
export function JourneyCanvas({ phases }: { phases: number }) {
  const ref = useRef<HTMLDivElement>(null);
  useThreeScene(ref, ({ scene, camera, small }) => {
    const root = new THREE.Group();
    scene.add(root);
    // The path runs left to right and winds in depth, one milestone per phase, longer at both ends.
    const stops = Array.from({ length: phases + 2 }, (_, i) => {
      const k = i - 1;
      return new THREE.Vector3(k * 2.4, Math.sin(k * 1.7) * 0.45, Math.cos(k * 1.1) * 1.4 - 1);
    });
    const curve = new THREE.CatmullRomCurve3(stops);
    const pathGeo = new THREE.BufferGeometry().setFromPoints(curve.getPoints(200));
    const pathMat = new THREE.LineBasicMaterial({ transparent: true, opacity: 0.55 });
    root.add(new THREE.Line(pathGeo, pathMat));

    const disc = discTexture();
    const ring = discTexture(true);
    const milestones = stops.slice(1, -1).map((p) => {
      const dot = new THREE.Sprite(new THREE.SpriteMaterial({ map: disc, transparent: true, depthWrite: false }));
      dot.position.copy(p);
      dot.scale.setScalar(0.2);
      const halo = new THREE.Sprite(
        new THREE.SpriteMaterial({ map: ring, transparent: true, depthWrite: false, blending: THREE.AdditiveBlending })
      );
      halo.position.copy(p);
      root.add(dot, halo);
      return { dot, halo };
    });

    // Read passively: the page scrolls as before, the camera only follows.
    let scroll = 0;
    let target = 0;
    const onScroll = () => {
      const max = document.documentElement.scrollHeight - window.innerHeight;
      target = max > 0 ? Math.min(window.scrollY / max, 1) : 0;
    };
    window.addEventListener("scroll", onScroll, { passive: true });
    onScroll();

    let elapsed = 0;
    const eye = new THREE.Vector3();
    // The camera trails behind the point it follows, so the path ahead fills the right-hand side
    // of the hero, away from the text.
    const back = small ? 2 : 4;
    return {
      applyColors() {
        const accent = tokenColor("--ax-bg-accent-strong", undefined, "#3b82f6");
        const glow = tokenColor("--ax-bg-warning-strong", undefined, "#f59e0b");
        pathMat.color.copy(accent);
        milestones.forEach((m) => {
          m.dot.material.color.copy(glow);
          m.halo.material.color.copy(glow);
        });
      },
      update(dt) {
        elapsed += dt;
        scroll = dt === 0 ? target : scroll + (target - scroll) * Math.min(dt * 3, 1);
        // Slow drift there and back over the first part, plus up to half the path from scroll.
        const u = 0.02 + 0.12 * (0.5 - 0.5 * Math.cos(elapsed * 0.08)) + scroll * 0.5;
        curve.getPointAt(Math.min(u, 1), eye);
        // The hero is a wide strip: fix the horizontal field of view (~70°) instead of the vertical one.
        const fov = THREE.MathUtils.radToDeg(2 * Math.atan(Math.tan(THREE.MathUtils.degToRad(35)) / camera.aspect));
        if (Math.abs(camera.fov - fov) > 0.01) {
          camera.fov = fov;
          camera.updateProjectionMatrix();
        }
        camera.position.set(eye.x - back, 1.6, 6);
        camera.lookAt(eye.x, 0.7, -1);
        milestones.forEach((m, i) => {
          const beat = 0.5 + 0.5 * Math.sin(elapsed * 1.4 - i * 0.8);
          m.halo.scale.setScalar(0.34 + beat * 0.16);
          m.halo.material.opacity = 0.35 + beat * 0.4;
        });
      },
      dispose() {
        window.removeEventListener("scroll", onScroll);
        disposeTree(root);
      },
    };
  });

  // Decorative, as on the front page: the mask fades it out behind the text on the left (and the
  // top on phones), so the hero text keeps its contrast.
  return (
    <div
      ref={ref}
      aria-hidden="true"
      className="absolute inset-0 pointer-events-none [&>canvas]:w-full [&>canvas]:h-full [mask-image:linear-gradient(90deg,transparent_20%,black_70%)] opacity-80 max-md:opacity-50 max-md:[mask-image:linear-gradient(90deg,transparent_45%,black_90%)]"
    />
  );
}
