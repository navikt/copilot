"use client";

import { useRef } from "react";
import * as THREE from "three";
import { HeroBand } from "./hero-band";
import { discTexture, disposeTree, fitCamera, textSprite, tokenColor, useThreeScene } from "./use-three-scene";

const WORKERS = 4;
const LOOP = 8; // seconds; each worker starts a quarter loop after the previous one
const CUBES = 12;

const smooth = (a: number, b: number, t: number) => {
  const x = Math.min(Math.max((t - a) / (b - a), 0), 1);
  return x * x * (3 - 2 * x);
};

/** nav-pilot hero: @nav-pilot hands tasks to @worker subagents and gets results back. */
export function CoordinatorCanvas() {
  const ref = useRef<HTMLDivElement>(null);
  useThreeScene(ref, ({ scene, camera, small, host }) => {
    const root = new THREE.Group();
    scene.add(root);
    const disc = discTexture();
    const ring = discTexture(true);
    const sprite = (size: number, tex = disc, additive = false) => {
      const s = new THREE.Sprite(
        new THREE.SpriteMaterial({
          map: tex,
          transparent: true,
          depthWrite: false,
          blending: additive ? THREE.AdditiveBlending : THREE.NormalBlending,
        })
      );
      s.scale.setScalar(size);
      root.add(s);
      return s;
    };

    const hub = sprite(0.55);
    const hubRing = sprite(0.9, ring);
    const hubLabel = textSprite("@nav-pilot", small ? 0.3 : 0.26);
    hubLabel.position.set(0, -0.62, 0);
    root.add(hubLabel);

    const rx = small ? 2 : 3.2;
    const workers = Array.from({ length: WORKERS }, (_, i) => {
      const a = Math.PI / 4 + (i * Math.PI) / 2;
      const pos = new THREE.Vector3(Math.cos(a) * rx, Math.sin(a) * 1.1, Math.sin(a * 2) * 0.4);
      const node = sprite(0.3);
      node.position.copy(pos);
      const glow = sprite(0.6, ring);
      glow.position.copy(pos);
      const label = textSprite("@worker", small ? 0.22 : 0.18);
      label.position.copy(pos).y -= 0.36;
      root.add(label);
      return { pos, node, glow, label, task: sprite(0.18, disc, true), result: sprite(0.2, disc, true) };
    });

    const linkGeo = new THREE.BufferGeometry().setFromPoints(workers.flatMap((w) => [new THREE.Vector3(), w.pos]));
    const linkMat = new THREE.LineBasicMaterial({ transparent: true, opacity: 0.25 });
    root.add(new THREE.LineSegments(linkGeo, linkMat));

    // «agentpakke»: a slow ring of small cubes around the coordinator.
    const cubeGroup = new THREE.Group();
    const cubeGeo = new THREE.BoxGeometry(0.11, 0.11, 0.11);
    const cubeMat = new THREE.MeshBasicMaterial({ transparent: true, opacity: 0.55 });
    for (let i = 0; i < CUBES; i++) {
      const c = new THREE.Mesh(cubeGeo, cubeMat);
      const a = (i / CUBES) * Math.PI * 2;
      c.position.set(Math.cos(a) * 1.05, 0, Math.sin(a) * 1.05);
      c.rotation.set(a, a * 2, 0);
      cubeGroup.add(c);
    }
    cubeGroup.rotation.x = 0.35;
    root.add(cubeGroup);

    let elapsed = 0;
    const centre = new THREE.Vector3();
    return {
      applyColors() {
        const accent = tokenColor("--ax-bg-accent-strong", host, "#3b82f6");
        const success = tokenColor("--ax-bg-success-strong", host, "#10b981");
        const text = tokenColor("--ax-text-neutral-subtle", host, "#c0c6d0");
        const purple = tokenColor("--ax-bg-meta-purple-strong", host, "#a78bfa");
        hub.material.color.copy(accent);
        hubRing.material.color.copy(accent);
        hubLabel.material.color.copy(text);
        linkMat.color.copy(text);
        cubeMat.color.copy(purple);
        workers.forEach((w) => {
          w.node.material.color.copy(purple);
          w.glow.material.color.copy(purple);
          w.label.material.color.copy(text);
          w.task.material.color.copy(accent);
          w.result.material.color.copy(success);
        });
      },
      update(dt) {
        fitCamera(camera, rx + 0.7, 1.7);
        // A still frame (reduced motion) shows packets on their way.
        elapsed = dt === 0 && elapsed === 0 ? 1.0 : elapsed + dt;
        let arrived = 0;
        workers.forEach((w, i) => {
          const p = (elapsed + LOOP - (i * LOOP) / WORKERS) % LOOP;
          const out = smooth(0, 1.3, p);
          w.task.position.lerpVectors(centre, w.pos, out);
          w.task.material.opacity = p < 1.4 ? 1 : 0;
          const working = p > 1.3 && p < 4.2 ? 1 : 0;
          const beat = working * (0.5 + 0.5 * Math.sin((p - 1.3) * 6));
          w.glow.scale.setScalar(0.45 + beat * 0.35);
          w.glow.material.opacity = working * (0.3 + beat * 0.5);
          const back = smooth(4.2, 5.6, p);
          w.result.position.lerpVectors(w.pos, centre, back);
          w.result.material.opacity = p > 4.2 && p < 5.7 ? 1 : 0;
          arrived = Math.max(arrived, 1 - Math.min(Math.abs(p - 5.7) / 0.4, 1));
        });
        hubRing.scale.setScalar(0.85 + arrived * 0.3);
        hubRing.material.opacity = 0.35 + arrived * 0.5;
        cubeGroup.rotation.y = elapsed * 0.15;
        root.rotation.y = Math.sin(elapsed * 0.07) * 0.15;
      },
      dispose: () => disposeTree(root),
    };
  });
  return <HeroBand ref={ref} />;
}
