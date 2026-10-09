"use client";

import { useRef } from "react";
import * as THREE from "three";
import { HeroBand } from "./hero-band";
import { discTexture, disposeTree, fitCamera, textSprite, tokenColor, useThreeScene } from "./use-three-scene";

// Real cplt defaults (navikt/cplt README): the proxy lets the agent reach GitHub, npm and PyPI;
// the kernel blocks reads of ~/.ssh, ~/.aws and ~/.gnupg.
const TARGETS = [
  { label: "api.github.com", allowed: true },
  { label: "~/.ssh", allowed: false },
  { label: "registry.npmjs.org", allowed: true },
  { label: "~/.aws", allowed: false },
  { label: "pypi.org", allowed: true },
  { label: "~/.gnupg", allowed: false },
];

const PROBE = 4; // seconds per probe; two probes make the ~8 s loop
const TRAIL = 28;

const smooth = (a: number, b: number, t: number) => {
  const x = Math.min(Math.max((t - a) / (b - a), 0), 1);
  return x * x * (3 - 2 * x);
};

/** cplt hero: an agent wanders inside a sandbox and probes the targets outside it. */
export function SandboxCanvas() {
  const ref = useRef<HTMLDivElement>(null);
  useThreeScene(ref, ({ scene, camera, small, host }) => {
    // Allowed targets to the right of the box, blocked to the left. On phones the labels sit
    // under the dots, so the scene is narrower and can be drawn larger.
    const half = new THREE.Vector3(1.4, 1, 1);
    const side = small ? 2.5 : 3;
    const root = new THREE.Group();
    scene.add(root);

    const boxGeo = new THREE.BoxGeometry(half.x * 2, half.y * 2, half.z * 2);
    const edgeMat = new THREE.LineBasicMaterial({ transparent: true, opacity: 0.55 });
    root.add(new THREE.LineSegments(new THREE.EdgesGeometry(boxGeo), edgeMat));
    const fillMat = new THREE.MeshBasicMaterial({ transparent: true, opacity: 0.06, depthWrite: false });
    root.add(new THREE.Mesh(boxGeo, fillMat));

    const disc = discTexture();
    const targets = TARGETS.map((t, i) => {
      const k = Math.floor(i / 2) - 1; // -1, 0, 1 along the side
      const pos = new THREE.Vector3(t.allowed ? side : -side, k * 1.1, k * 0.3);
      const dotMat = new THREE.SpriteMaterial({ map: disc, transparent: true, depthWrite: false });
      const dot = new THREE.Sprite(dotMat);
      dot.scale.setScalar(0.16);
      dot.position.copy(pos);
      const label = textSprite(t.label, small ? 0.26 : 0.2);
      const lw = label.scale.x / 2 + 0.18;
      label.position.copy(pos);
      if (small) label.position.y -= 0.26;
      else label.position.x += t.allowed ? lw : -lw;
      root.add(dot, label);
      // Where a probe towards this target meets the wall: scale the ray until it hits the box.
      const wall = pos
        .clone()
        .divideScalar(Math.max(Math.abs(pos.x) / half.x, Math.abs(pos.y) / half.y, Math.abs(pos.z) / half.z));
      return { ...t, pos, wall, dotMat, labelMat: label.material };
    });

    // The agent: a bright point with a fading trail (additive, so darker vertices vanish).
    const agentMat = new THREE.SpriteMaterial({
      map: disc,
      transparent: true,
      depthWrite: false,
      blending: THREE.AdditiveBlending,
    });
    const agent = new THREE.Sprite(agentMat);
    agent.scale.setScalar(small ? 0.26 : 0.22);
    root.add(agent);
    const trailPos = new Float32Array(TRAIL * 3);
    const trailCol = new Float32Array(TRAIL * 3);
    const trailGeo = new THREE.BufferGeometry();
    trailGeo.setAttribute("position", new THREE.BufferAttribute(trailPos, 3));
    trailGeo.setAttribute("color", new THREE.BufferAttribute(trailCol, 3));
    const trailMat = new THREE.LineBasicMaterial({
      vertexColors: true,
      transparent: true,
      blending: THREE.AdditiveBlending,
    });
    root.add(new THREE.Line(trailGeo, trailMat));

    // Allowed: a glowing line through the wall. Blocked: a ring flash where the probe hits it.
    const beamGeo = new THREE.BufferGeometry().setFromPoints([new THREE.Vector3(), new THREE.Vector3()]);
    const beamMat = new THREE.LineBasicMaterial({ transparent: true, opacity: 0, blending: THREE.AdditiveBlending });
    root.add(new THREE.Line(beamGeo, beamMat));
    const flashMat = new THREE.SpriteMaterial({
      map: discTexture(true),
      transparent: true,
      opacity: 0,
      depthWrite: false,
    });
    const flash = new THREE.Sprite(flashMat);
    root.add(flash);

    const colors = { agent: new THREE.Color(), ok: new THREE.Color(), no: new THREE.Color() };
    let elapsed = 0;
    const wander = (t: number, out: THREE.Vector3) =>
      out.set(
        Math.sin(t * 0.7) * half.x * 0.6,
        Math.sin(t * 1.1 + 1) * half.y * 0.55,
        Math.cos(t * 0.5) * half.z * 0.5
      );
    const free = new THREE.Vector3();
    const tmp = new THREE.Color();

    return {
      applyColors() {
        const edge = tokenColor("--ax-border-neutral", host, "#8a94a6");
        edgeMat.color.copy(edge);
        fillMat.color.copy(edge);
        colors.agent.copy(tokenColor("--ax-text-neutral", host, "#ffffff"));
        colors.ok.copy(tokenColor("--cplt-accent", host, "#10b981"));
        colors.no.copy(tokenColor("--ax-text-danger-decoration", host, "#ff6b6b"));
        agentMat.color.copy(colors.agent);
        beamMat.color.copy(colors.ok);
        flashMat.color.copy(colors.no);
        targets.forEach((t) => {
          t.dotMat.color.copy(t.allowed ? colors.ok : colors.no);
          t.labelMat.color.copy(tokenColor("--ax-text-neutral-subtle", host, "#c0c6d0"));
        });
      },
      update(dt) {
        fitCamera(camera, small ? 4 : 5.4, 1.9);
        // A still frame (reduced motion) shows an allowed probe with its line lit.
        elapsed = dt === 0 && elapsed === 0 ? 2.8 : elapsed + dt;
        const n = Math.floor(elapsed / PROBE);
        const p = elapsed % PROBE;
        const target = targets[n % targets.length];
        // w: how far the agent has left its wander path towards the wall.
        const reach = smooth(1.3, 2.2, p);
        const back = target.allowed ? smooth(3.2, 4, p) : smooth(2.25, 2.9, p);
        const w = reach * (1 - back);
        wander(elapsed, free);
        agent.position.lerpVectors(free, target.wall, w * 0.97);

        beamMat.opacity = target.allowed ? smooth(2.0, 2.3, p) * (1 - smooth(3.0, 3.8, p)) * 0.9 : 0;
        beamGeo.attributes.position.setXYZ(0, agent.position.x, agent.position.y, agent.position.z);
        beamGeo.attributes.position.setXYZ(1, target.pos.x, target.pos.y, target.pos.z);
        beamGeo.attributes.position.needsUpdate = true;

        const hit = target.allowed ? 0 : smooth(2.1, 2.25, p) * (1 - smooth(2.3, 3.1, p));
        flashMat.opacity = hit;
        flash.position.copy(target.wall);
        flash.scale.setScalar(0.25 + (1 - hit) * 0.35 * (p > 2.3 ? 1 : 0));
        edgeMat.opacity = 0.55 + hit * 0.35;

        // Trail: shift back one slot, newest at index 0, fading towards the tail.
        trailPos.copyWithin(3, 0, (TRAIL - 1) * 3);
        agent.position.toArray(trailPos, 0);
        if (dt === 0) for (let i = 1; i < TRAIL; i++) agent.position.toArray(trailPos, i * 3);
        for (let i = 0; i < TRAIL; i++)
          tmp
            .copy(colors.agent)
            .multiplyScalar(0.6 * (1 - i / TRAIL))
            .toArray(trailCol, i * 3);
        trailGeo.attributes.position.needsUpdate = true;
        trailGeo.attributes.color.needsUpdate = true;

        root.rotation.y = Math.sin(elapsed * 0.08) * 0.25;
        root.rotation.x = 0.12 + Math.sin(elapsed * 0.05) * 0.05;
      },
      dispose: () => disposeTree(root),
    };
  });

  return <HeroBand ref={ref} />;
}
