"use client";

/**
 * The hero on /mcp: tool calls from many clients merge into one stream through fleet-gateway, pass four gates and
 * fan out to the MCP servers. A port of the approved mockup. The calls follow a fixed, seeded plan, so the picture is
 * the same every visit. Server names and tools are real (pilot-routes.prod.yaml); the calls are examples.
 *
 * Every position is a pure function of time, so each animation frame just re-renders. Under prefers-reduced-motion
 * the diagram shows one fixed frame with numbered steps; the pause button does the same on request (WCAG 2.2.2).
 */

import { CodeIcon, LaptopIcon, RobotIcon, SparklesIcon } from "@navikt/aksel-icons";
import { BodyShort, Button, HStack, Switch, Tag, VStack } from "@navikt/ds-react";
import { SiGithubcopilot } from "@icons-pack/react-simple-icons";
import { useEffect, useRef, useState, type ComponentType } from "react";

type Icon = ComponentType<{ width?: number; height?: number; "aria-hidden"?: boolean }>;
type Pt = { x: number; y: number };

const C_COPILOT = "var(--ax-text-accent)";
const C_AGENT = "var(--ax-text-warning)";
const C_RED = "var(--ax-bg-danger-strong)";
const C_FOCUS = "var(--ax-border-accent-strong)";

type ClientType = { k: string; full: string; short: string; ic: Icon; c: string; agent?: boolean; mix: number[] };
const TYPES: ClientType[] = [
  {
    k: "copilot-vscode",
    full: "GitHub Copilot i VS Code",
    short: "VS Code",
    ic: SiGithubcopilot,
    c: C_COPILOT,
    mix: [2, 2, 1, 1],
  },
  { k: "copilot-cli", full: "GitHub Copilot CLI", short: "CLI", ic: SiGithubcopilot, c: C_COPILOT, mix: [1, 2, 1, 0] },
  {
    k: "copilot-agent",
    full: "GitHub Copilot coding agent",
    short: "Agent",
    ic: SiGithubcopilot,
    c: C_COPILOT,
    mix: [1, 1, 0, 0],
  },
  { k: "opencode", full: "OpenCode", short: "OpenCode", ic: CodeIcon, c: "var(--ax-text-neutral)", mix: [1, 1, 1, 0] },
  {
    k: "copilot-intellij",
    full: "GitHub Copilot i IntelliJ",
    short: "IntelliJ",
    ic: SiGithubcopilot,
    c: C_COPILOT,
    mix: [1, 1, 1, 0],
  },
  {
    k: "nav-pilot",
    full: "nav-pilot",
    short: "nav-pilot",
    ic: SparklesIcon,
    c: "var(--ax-text-meta-purple)",
    mix: [2, 2, 1, 0],
  },
  {
    k: "mcp-klient",
    full: "Annen MCP-klient",
    short: "Andre",
    ic: LaptopIcon,
    c: "var(--ax-text-neutral-subtle)",
    mix: [0, 1, 0, 0],
  },
  // Agent profiles call only Aksel designsystem and Felles kodeverk.
  {
    k: "aksel-studio",
    full: "aksel-studio: lager kodeprototyper fra Aksel-komponenter (agentprofil)",
    short: "studio",
    ic: RobotIcon,
    c: C_AGENT,
    agent: true,
    mix: [2, 0, 0, 0],
  },
  {
    k: "sikkerhet-agent",
    full: "Sikkerhetsagent: sjekker avhengigheter og teamets egen kode for sårbarheter (agentprofil)",
    short: "sikkerhet",
    ic: RobotIcon,
    c: C_AGENT,
    agent: true,
    mix: [0, 2, 0, 0],
  },
  {
    k: "oppgrader-agent",
    full: "Oppgraderingsagent: oppdaterer avhengigheter og lager PR-er (agentprofil)",
    short: "oppgrader",
    ic: RobotIcon,
    c: C_AGENT,
    agent: true,
    mix: [1, 1, 0, 0],
  },
  {
    k: "docs-agent",
    full: "Dokumentasjonsagent: holder README og dokumentasjon oppdatert fra koden (agentprofil)",
    short: "docs",
    ic: RobotIcon,
    c: C_AGENT,
    agent: true,
    mix: [2, 1, 0, 0],
  },
  {
    k: "compliance-agent",
    full: "Compliance-agent: sjekker at teamets MCP-server følger retningslinjene og består samsvarstesten (agentprofil)",
    short: "compliance",
    ic: RobotIcon,
    c: C_AGENT,
    agent: true,
    mix: [1, 1, 0, 0],
  },
];
// 16 tiles: the clients repeat, many users. Studio (slot 6) and sikkerhet (slot 10) never swap out.
const TILES = [0, 5, 1, 3, 0, 4, 7, 0, 1, 5, 8, 6, 9, 0, 5, 1];
const AGENT_SLOT = 12,
  NON = 7,
  AG = 5;
const TW = TILES.map((t) => (TYPES[t].agent ? 1 : 2));

type Server = { route: string; title: string; ro: boolean; n: number; ex: [string, number, string?][]; lc?: string };
const SERVERS: Server[] = [
  {
    route: "/mcp/designsystem/aksel",
    title: "Aksel designsystem",
    ro: true,
    n: 5,
    ex: [
      ["aksel_get_component_info", 2],
      ["aksel_find_icons", 1],
    ],
    lc: "var(--ax-border-info)",
  },
  {
    route: "/mcp/copilot/kodeverk",
    title: "Felles kodeverk",
    ro: true,
    n: 6,
    ex: [
      ["search_kodeverk", 5],
      ["get_kodeverk_koder", 1],
    ],
  },
  {
    route: "/mcp/copilot/team-catalog",
    title: "Teamkatalogen",
    ro: true,
    n: 10,
    ex: [
      ["get_team", 2],
      ["search_teams", 1],
    ],
  },
  {
    route: "/mcp/copilot/dolly",
    title: "Dolly",
    ro: false,
    n: 7,
    ex: [
      ["dolly_search", 3],
      ["create_order", 6, "w"],
    ],
  },
];
const toolName = (s: number, t?: number) => (SERVERS[s].ex.find((x) => x[1] === t) || ["?"])[0];
const GATES = [
  { label: "Tilgang", why: "" },
  { label: "Godkjente verktøy", why: "verktøyet er ikke godkjent" },
  { label: "Ratelimit", why: "ratelimit nådd" },
  { label: "Nødstopp", why: "" },
];
const OUTSIDE = "<verktøy utenfor listen>";
type Call = { c: number; s: number; t?: number; rej?: number };
// The followed calls (tile index); rej is the gate that stops it.
const FOCUS: Call[] = [
  { c: 1, s: 1, t: 5 },
  { c: 6, s: 0, t: 2 },
  { c: 0, s: 3, t: 3 },
  { c: 2, s: 3, rej: 1 },
  { c: 9, s: 0, t: 2 },
  { c: 14, s: 3, t: 6 },
  { c: 5, s: 2, t: 1 },
  { c: 14, s: 3, t: 6, rej: 2 },
  { c: 10, s: 1, t: 5 },
  { c: 3, s: 1, t: 1 },
];
const D = 6200,
  FP = 7000,
  PER = 60000,
  POOL = 40;
const T_STILL = 4 * FP + 0.99 * D;

const frac = (x: number) => x - Math.floor(x);
const rnd = (n: number, salt: number) => frac(Math.sin(n * 12.9898 + salt * 78.233) * 43758.5453);
const pickW = (ws: number[], r: number) => {
  let x = r * ws.reduce((a, b) => a + b, 0);
  for (let i = 0; i < ws.length; i++) {
    x -= ws[i];
    if (x < 0) return i;
  }
  return ws.length - 1;
};

// Organic grid: seeded fade swaps on slots that never carry a followed call.
const AVAIL = [4, 7, 8, 11, 12, 13, 15],
  SWP = 4200,
  FADE = 600;
type Swap = { slot: number; S: number; E: number; old: number; ty: number };
const SWS: Swap[] = [];
const LASTT: Record<number, number> = {};
function sw(n: number): Swap {
  for (let m = SWS.length; m <= n; m++) {
    const st = Math.floor(rnd(m, 13) * 7);
    let slot = AVAIL[st];
    for (let j = 0; j < 7; j++) {
      const c = AVAIL[(st + j) % 7];
      if ((m < 1 || SWS[m - 1].slot !== c) && (m < 2 || SWS[m - 2].slot !== c) && (m < 3 || SWS[m - 3].slot !== c)) {
        slot = c;
        break;
      }
    }
    const old = LASTT[slot] ?? TILES[slot];
    const ty =
      slot === AGENT_SLOT
        ? NON + ((old - NON + 1 + Math.floor(rnd(m, 14) * (AG - 1))) % AG)
        : (old + 1 + Math.floor(rnd(m, 14) * (NON - 1))) % NON;
    SWS.push({ slot, S: m * SWP + Math.floor(rnd(m, 11) * 1800), E: 6500 + Math.floor(rnd(m, 12) * 2500), old, ty });
    LASTT[slot] = ty;
  }
  return SWS[n];
}
type Phase = "on" | "out" | "empty" | "in";
function view(i: number, T: number): { ty: number; ph: Phase } {
  if (AVAIL.indexOf(i) < 0) return { ty: TILES[i], ph: "on" };
  for (let n = Math.floor(T / SWP) + 1; n >= 0; n--) {
    const w = sw(n);
    if (w.slot !== i || T < w.S) continue;
    const t = T - w.S;
    if (t < FADE) return { ty: w.old, ph: "out" };
    if (t < FADE + w.E) return { ty: w.ty, ph: "empty" };
    return { ty: w.ty, ph: t < 2 * FADE + w.E ? "in" : "on" };
  }
  return { ty: TILES[i], ph: "on" };
}
const present = (c: number, t: number) => view(c, t).ph === "on";

// Seeded, wave-shaped arrivals with bursts from the same tile.
type Evt = Call & { at: number };
const EVT: Evt[] = (() => {
  const out: Evt[] = [];
  for (let t = 0; t < PER; t += 50) {
    const lam =
      Math.max(0, 0.45 + 0.9 * Math.sin((2 * Math.PI * t) / 13000) + 0.35 * Math.sin((2 * Math.PI * t) / 4700 + 1)) *
      2.6;
    if (rnd(t, 7) >= lam * 0.05) continue;
    const c = pickW(TW, rnd(t, 1)),
      k = rnd(t, 4) < 0.3 ? 1 + Math.floor(rnd(t, 5) * 3) : 0;
    for (let j = 0; j <= k; j++) {
      const s = pickW(TYPES[TILES[c]].mix, rnd(t + j, 2)),
        r = rnd(t + j, 3);
      out.push({ at: t + j * (90 + Math.floor(rnd(t + j, 6) * 110)), c, s, rej: r < 0.05 ? 1 : r < 0.08 ? 2 : 0 });
    }
  }
  return out.sort((a, b) => a.at - b.at);
})();

type Box = { x: number; y: number; w: number; h: number };
type Layout = {
  W: number;
  H: number;
  v: boolean;
  tBox: (i: number) => Box;
  tAnc: (i: number) => Pt;
  gw: Box & { ty: number; tx: number; tw: number };
  entry: (k: number) => Pt;
  exit: (k: number) => Pt;
  gatePos: number[];
  gateSpan: number[];
  gateLabel: (p: number) => { x: number; y: number; w: number; align: string };
  sBox: (k: number) => Box;
  sAnc: (k: number) => Pt;
};
function layout(v: boolean): Layout {
  if (!v) {
    // Wide: the band sits on the horizon (cy); streams and cards are symmetric about it.
    const cy = 147,
      lane = (k: number) => cy + (k - 1.5) * 8,
      cardY = [cy - 142, cy - 72, cy + 10, cy + 80];
    return {
      W: 1064,
      H: 300,
      v,
      tBox: (i) => ({ x: 14 + (i % 4) * 70, y: 14 + Math.floor(i / 4) * 68, w: 66, h: 62 }),
      tAnc: (i) => ({ x: 290, y: 45 + Math.floor(i / 4) * 68 }),
      gw: { x: 360, y: cy - 24, w: 408, h: 48, ty: cy + 28, tx: 360, tw: 408 },
      entry: (k) => ({ x: 360, y: lane(k) }),
      exit: (k) => ({ x: 768, y: lane(k) }),
      gatePos: [405, 515, 625, 735],
      gateSpan: [cy - 19, cy + 19],
      gateLabel: (p) => ({ x: p - 63, y: cy - 46, w: 110, align: "center" }),
      sBox: (k) => ({ x: 808, y: cardY[k], w: 246, h: 62 }),
      sAnc: (k) => ({ x: 808, y: cardY[k] + 31 }),
    };
  }
  const cx = 175,
    lane = (k: number) => cx + (k - 1.5) * 8;
  return {
    W: 350,
    H: 920,
    v,
    tBox: (i) => ({ x: 2 + (i % 4) * 87, y: 8 + Math.floor(i / 4) * 60, w: 84, h: 54 }),
    tAnc: (i) => ({ x: 44 + (i % 4) * 87, y: 62 + Math.floor(i / 4) * 60 }),
    gw: { x: cx - 24, y: 320, w: 48, h: 280, ty: 294, tx: cx - 100, tw: 200 },
    entry: (k) => ({ x: lane(k), y: 320 }),
    exit: (k) => ({ x: lane(k), y: 600 }),
    gatePos: [375, 440, 505, 570],
    gateSpan: [cx - 19, cx + 19],
    gateLabel: (p) => ({ x: 4, y: p - 9, w: 142, align: "flex-end" }),
    sBox: (k) => ({ x: 46, y: 650 + k * 66, w: 300, h: 58 }),
    sAnc: (k) => ({ x: 46, y: 679 + k * 66 }),
  };
}

// Orthogonal routes with rounded corners, like a metro map.
const R = 10;
function route(L: Layout, c: number, k: number): Pt[] {
  const a = L.tAnc(c),
    en = L.entry(k),
    ex = L.exit(k),
    b = L.sAnc(k);
  if (!L.v) {
    const bx = 320,
      tx = 788;
    return [a, { x: bx, y: a.y }, { x: bx, y: en.y }, en, ex, { x: tx, y: ex.y }, { x: tx, y: b.y }, b];
  }
  const by = 252 + c * 3,
    tx = 20 + k * 6,
    ty = 612 + k * 5;
  return [a, { x: a.x, y: by }, { x: en.x, y: by }, en, ex, { x: ex.x, y: ty }, { x: tx, y: ty }, { x: tx, y: b.y }, b];
}
function clean(p: Pt[]) {
  const o = [p[0]];
  for (const q of p.slice(1)) {
    const l = o[o.length - 1];
    if (Math.abs(q.x - l.x) + Math.abs(q.y - l.y) > 0.5) o.push(q);
  }
  for (let i = o.length - 2; i > 0; i--) {
    const A = o[i - 1],
      B = o[i],
      C = o[i + 1];
    if (Math.abs((B.x - A.x) * (C.y - B.y) - (B.y - A.y) * (C.x - B.x)) < 0.01) o.splice(i, 1);
  }
  return o;
}
const hyp = (a: Pt, b: Pt) => Math.hypot(b.x - a.x, b.y - a.y);
type Seg = { t: "L" | "Q"; a: Pt; b: Pt; c?: Pt; len: number; at: number };
type Route = { segs: Seg[]; len: number; dE: number; dX: number; dG: number[] };
function segs(p0: Pt[]) {
  const p = clean(p0),
    out: Seg[] = [];
  let cur = p[0];
  for (let i = 1; i < p.length; i++) {
    const B = p[i];
    if (i === p.length - 1) {
      out.push({ t: "L", a: cur, b: B, len: 0, at: 0 });
      break;
    }
    const C = p[i + 1],
      d1 = hyp(p[i - 1], B),
      d2 = hyp(B, C),
      r = Math.min(R, d1 / 2, d2 / 2);
    const s = { x: B.x - ((B.x - p[i - 1].x) / d1) * r, y: B.y - ((B.y - p[i - 1].y) / d1) * r };
    const e = { x: B.x + ((C.x - B.x) / d2) * r, y: B.y + ((C.y - B.y) / d2) * r };
    out.push({ t: "L", a: cur, b: s, len: 0, at: 0 }, { t: "Q", a: s, c: B, b: e, len: 0, at: 0 });
    cur = e;
  }
  let acc = 0;
  out.forEach((g) => {
    g.len = g.t === "L" ? hyp(g.a, g.b) : (hyp(g.a, g.c!) + hyp(g.c!, g.b) + hyp(g.a, g.b)) / 2;
    g.at = acc;
    acc += g.len;
  });
  return { segs: out.filter((g) => g.len > 0), len: acc };
}
const pathD = (r: { segs: Seg[] }) =>
  "M" +
  r.segs[0].a.x +
  " " +
  r.segs[0].a.y +
  r.segs
    .map((g) => (g.t === "L" ? "L" + g.b.x + " " + g.b.y : "Q" + g.c!.x + " " + g.c!.y + " " + g.b.x + " " + g.b.y))
    .join("");
function ptAt(r: { segs: Seg[] }, d: number): Pt {
  for (const g of r.segs) {
    if (d <= g.at + g.len) {
      const w = g.len ? (d - g.at) / g.len : 0;
      if (g.t === "L") return { x: g.a.x + (g.b.x - g.a.x) * w, y: g.a.y + (g.b.y - g.a.y) * w };
      const m = 1 - w;
      return {
        x: m * m * g.a.x + 2 * m * w * g.c!.x + w * w * g.b.x,
        y: m * m * g.a.y + 2 * m * w * g.c!.y + w * w * g.b.y,
      };
    }
  }
  const l = r.segs[r.segs.length - 1];
  return { x: l.b.x, y: l.b.y };
}
// Distance along the route where it first crosses a coordinate inside the band.
function distAt(r: { segs: Seg[] }, val: number, vert: boolean) {
  for (const g of r.segs) {
    if (g.t !== "L") continue;
    const a = vert ? g.a.y : g.a.x,
      b = vert ? g.b.y : g.b.x;
    if (
      (a - val) * (b - val) <= 0 &&
      Math.abs(b - a) > 0.1 &&
      Math.abs((vert ? g.a.x : g.a.y) - (vert ? g.b.x : g.b.y)) < 0.1
    )
      return g.at + (g.len * (val - a)) / (b - a);
  }
  return 0;
}
const CACHE: Record<string, Route> = {};
function R2(L: Layout, c: number, k: number): Route {
  const key = (L.v ? "v" : "h") + c + ":" + k;
  if (!CACHE[key]) {
    const r = segs(route(L, c, k));
    CACHE[key] = {
      ...r,
      dE: distAt(r, L.v ? L.entry(k).y : L.entry(k).x, L.v),
      dX: distAt(r, L.v ? L.exit(k).y : L.exit(k).x, L.v),
      dG: L.gatePos.map((p) => distAt(r, p, L.v)),
    };
  }
  return CACHE[key];
}
const ease = (u: number) => u - (Math.sin(2 * Math.PI * u) / (2 * Math.PI)) * 0.3;
function posAt(L: Layout, e: Call, u0: number) {
  const u = ease(Math.max(0, Math.min(1, u0))),
    r = R2(L, e.c, e.s),
    d = u * r.len;
  if (e.rej && d >= r.dG[e.rej]) {
    // Rejected: stops at its gate and fades out in red.
    const q = ptAt(r, r.dG[e.rej]),
      k = (d - r.dG[e.rej]) / (0.12 * r.len);
    return { x: q.x, y: q.y, o: Math.max(0, 1 - k), red: true };
  }
  const q = ptAt(r, d);
  return { x: q.x, y: q.y, o: u < 0.05 ? u / 0.05 : u > 0.96 ? Math.max(0, 1 - (u - 0.96) / 0.04) : 1, red: false };
}
function stageOf(L: Layout, e: Call, u0: number) {
  const u = ease(Math.max(0, Math.min(1, u0))),
    r = R2(L, e.c, e.s),
    d = u * r.len;
  if (d < r.dE) return 0;
  if (e.rej && d >= r.dG[e.rej]) return -1;
  if (d < r.dX) {
    let g = 0;
    for (let j = 0; j < 4; j++) if (d >= r.dG[j]) g = j + 1;
    return g;
  }
  return u >= 0.96 ? 6 : 5;
}
type Dot = { x: number; y: number; o: number; f: string; r: number };
function frame(L: Layout, T: number) {
  const act: [Call, number, number][] = [];
  const cyc = Math.floor(T / PER);
  for (let c = cyc - 1; c <= cyc; c++)
    for (const e of EVT) {
      const at = e.at + c * PER,
        u = (T - at) / D;
      if (u >= 0 && u <= 1 && present(e.c, at) && present(e.c, at + 0.45 * D)) {
        const mix = TYPES[view(e.c, at).ty].mix;
        act.push([{ ...e, s: mix[e.s] ? e.s : mix.findIndex((w) => w > 0) }, u, T - at]);
      }
    }
  const pool: Dot[] = [],
    glow: Record<number, boolean> = {};
  for (let i = 0; i < POOL; i++) {
    const a = act[i];
    if (!a) {
      pool.push({ x: -20, y: -20, o: 0, f: C_COPILOT, r: 3 });
      continue;
    }
    const p = posAt(L, a[0], a[1]);
    if (a[2] < 700) glow[a[0].c] = true;
    pool.push({
      x: p.x,
      y: p.y,
      o: p.o * (p.red ? 0.95 : 0.7),
      f: p.red ? C_RED : TYPES[TILES[a[0].c]].c,
      r: p.red ? 4.5 : 3,
    });
  }
  const fi = Math.floor(T / FP),
    fe = { ...FOCUS[fi % FOCUS.length], fi },
    fu = (T - fi * FP) / D;
  const p = posAt(L, fe, fu);
  if (fu < 0.1) glow[fe.c] = true;
  const fp = fu > 1 ? { x: -20, y: -20, o: 0, f: C_FOCUS } : { x: p.x, y: p.y, o: p.o, f: p.red ? C_RED : C_FOCUS };
  return { pool, fp, fe, stage: stageOf(L, fe, fu), glow };
}

const SUMMARY =
  "Illustrasjon: kall fra mange klienter, som GitHub Copilot, OpenCode, nav-pilot og agentprofiler, samles i én smal strøm gjennom fleet-gateway. " +
  "Der passerer de fire sjekker: tilgang, godkjente verktøy, ratelimit og nødstopp. " +
  "Så fordeles de til MCP-serverne Aksel designsystem, Felles kodeverk, Teamkatalogen og Dolly. " +
  "Kall som stoppes, blir røde ved sjekken som stopper dem.";
const PER_MIN = EVT.length + Math.round(PER / FP);
const REJECTED = EVT.filter((x) => x.rej).length + 1;

export function McpHeroDiagram() {
  const [rm, setRm] = useState(false);
  const [paused, setPaused] = useState(false);
  const [live, setLive] = useState(false);
  const [compact, setCompact] = useState(false);
  const [scale, setScale] = useState(1);
  // Diagram time in ms. The server and the reduced-motion frame show T_STILL: a call passing the last gates.
  const [t, setT] = useState(T_STILL);
  const wrap = useRef<HTMLDivElement>(null);
  const started = useRef(false);
  const still = rm || paused;

  useEffect(() => {
    const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
    const on = () => setRm(mq.matches);
    on();
    mq.addEventListener("change", on);
    return () => mq.removeEventListener("change", on);
  }, []);

  // The wide drawing scales down to fit; below 640 px the tall one takes over, so the text stays readable.
  useEffect(() => {
    const el = wrap.current;
    if (!el) return;
    const ro = new ResizeObserver(([e]) => {
      const w = e.contentRect.width;
      setCompact(w < 640);
      setScale(Math.min(1, w / (w < 640 ? 350 : 1064)));
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  useEffect(() => {
    if (still) return;
    let raf = 0;
    let from = 0;
    const loop = (ts: number) => {
      setT((prev) => {
        // The first start begins a little before the second followed call; a resume picks up where it stopped.
        if (!from) from = ts - (started.current ? prev : 2 * FP - 300);
        started.current = true;
        return ts - from;
      });
      raf = requestAnimationFrame(loop);
    };
    raf = requestAnimationFrame(loop);
    return () => cancelAnimationFrame(raf);
  }, [still]);

  const L = layout(compact),
    T = rm ? T_STILL : t,
    f = frame(L, T),
    e = f.fe,
    st = f.stage;
  const V = TILES.map((_, i) => view(i, T));

  const streams: { d: string; c: string; o: number }[] = [];
  V.forEach((vw, i) =>
    TYPES[vw.ty].mix.forEach((w, k) => {
      if (w)
        streams.push({ d: pathD(R2(L, i, k)), c: TYPES[vw.ty].c, o: vw.ph === "out" || vw.ph === "empty" ? 0 : 0.28 });
    })
  );
  const vol = [0, 0, 0, 0];
  TILES.forEach((t) => TYPES[t].mix.forEach((w, k) => (vol[k] += w)));
  const fr = R2(L, e.c, e.s);
  let focusD = pathD(fr);
  if (e.rej) {
    const cut = fr.dG[e.rej];
    focusD = pathD({
      segs: fr.segs
        .filter((g) => g.at < cut)
        .map((g) => (g.at + g.len > cut && g.t === "L" ? { ...g, b: ptAt(fr, cut) } : g)),
    });
  }
  const gState = (j: number) =>
    st === -1 && j === e.rej
      ? "no"
      : (st === -1 ? j < (e.rej ?? 0) : j < st || st >= 5)
        ? "ok"
        : st >= 1 && st <= 4 && j === st
          ? "run"
          : "";
  const log: { client: string; route: string; tool: string; rej: number; why: string; fresh: boolean; id: number }[] =
    [];
  for (let i = e.fi; i >= 0 && log.length < (compact ? 4 : 5); i--) {
    const g = FOCUS[i % FOCUS.length];
    if (i === e.fi && !(st === 6 || st === -1)) continue;
    log.push({
      client: TYPES[TILES[g.c]].k,
      route: SERVERS[g.s].route,
      tool: g.rej === 1 ? OUTSIDE : toolName(g.s, g.t),
      rej: g.rej ?? 0,
      why: g.rej ? GATES[g.rej].why : "tillatt",
      fresh: i === e.fi && !still,
      id: i,
    });
  }
  const nums = still
    ? (() => {
        const c = L.tBox(e.c),
          g = (j: number) => L.gateLabel(L.gatePos[j]),
          s = L.sBox(e.s);
        return [
          { n: 1, x: c.x - 8, y: c.y - 10 },
          { n: 2, x: g(0).x - 10, y: g(0).y - 14 },
          { n: 3, x: g(1).x - 10, y: g(1).y - 14 },
          { n: 4, x: g(2).x - 10, y: g(2).y - 14 },
          { n: 5, x: s.x - 10, y: s.y - 10 },
        ];
      })()
    : [];
  const box = (b: Box) => ({ left: b.x, top: b.y, width: b.w, height: b.h });

  return (
    <VStack gap="space-12" className="mh">
      <HStack justify="space-between" align="center" gap="space-8" wrap>
        <HStack gap="space-16" className="mh-mono" aria-hidden>
          <span>
            <b>{V.filter((x) => x.ph !== "empty").length}</b> klienter
          </span>
          <span>
            <b>{PER_MIN}</b> kall/min
          </span>
          <span className="mh-no">
            <b>{REJECTED}</b> avvist
          </span>
        </HStack>
        <HStack gap="space-8" align="center">
          <Tag variant="warning" size="small">
            Illustrasjon. Kallene er eksempler.
          </Tag>
          {!rm && (
            <Button size="small" variant="secondary" aria-pressed={paused} onClick={() => setPaused((p) => !p)}>
              {paused ? "Spill av" : "Pause"}
            </Button>
          )}
        </HStack>
      </HStack>

      <div ref={wrap}>
        <div className="mh-fit" style={{ height: L.H * scale }}>
          <div
            className="mh-stage"
            role="img"
            aria-label={SUMMARY}
            style={{ width: L.W, height: L.H, transform: `scale(${scale})` }}
          >
            <div className="mh-abs mh-gw" style={box(L.gw)} />
            <div className="mh-abs mh-gw-name" style={{ left: L.gw.tx, top: L.gw.ty, width: L.gw.tw }}>
              fleet-gateway
            </div>
            {SERVERS.map((s, k) => {
              const hit = st === 6 && e.s === k && !e.rej;
              return (
                <div key={s.route} className={"mh-abs mh-srv" + (hit ? " mh-srv-hit" : "")} style={box(L.sBox(k))}>
                  <div className="mh-srv-head">
                    <strong>{s.title}</strong>
                    <Tag variant={s.ro ? "info" : "warning"} size="xsmall">
                      {s.ro ? "Leser" : "Skriver"}
                    </Tag>
                  </div>
                  <div className="mh-tools">
                    {s.ex.map((x) => (
                      <span
                        key={x[0]}
                        className={"mh-tl" + (x[2] ? " mh-tl-w" : "") + (hit && e.t === x[1] ? " mh-tl-hit" : "")}
                      >
                        {x[0]}
                      </span>
                    ))}
                    <span className="mh-more">+{s.n - s.ex.length} verktøy</span>
                  </div>
                </div>
              );
            })}

            <svg
              className="mh-abs"
              style={{ left: 0, top: 0 }}
              width={L.W}
              height={L.H}
              viewBox={`0 0 ${L.W} ${L.H}`}
              aria-hidden
            >
              <defs>
                <filter id="mh-soft" x="-2" y="-2" width="5" height="5">
                  <feGaussianBlur stdDeviation="1.6" />
                </filter>
              </defs>
              {streams.map((p, i) => (
                <path key={i} d={p.d} fill="none" className="mh-stream" style={{ stroke: p.c }} strokeOpacity={p.o} />
              ))}
              {SERVERS.map((s, k) => {
                const a = L.entry(k),
                  b = L.exit(k);
                return (
                  <path
                    key={k}
                    d={`M${a.x} ${a.y}L${b.x} ${b.y}`}
                    fill="none"
                    style={{ stroke: s.lc ?? C_COPILOT }}
                    strokeWidth={1 + vol[k] / 8}
                    strokeOpacity="0.35"
                  />
                );
              })}
              <path
                d={focusD}
                fill="none"
                style={{ stroke: C_FOCUS }}
                strokeWidth="1.5"
                strokeOpacity="0.45"
                strokeDasharray="3 3"
              />
              {GATES.map((_, j) => {
                const p = L.gatePos[j],
                  c = gState(j) === "no" ? C_RED : "var(--ax-border-neutral)",
                  [a, b] = L.gateSpan;
                return L.v ? (
                  <line key={j} x1={a} y1={p} x2={b} y2={p} style={{ stroke: c }} strokeWidth="1.5" />
                ) : (
                  <line key={j} x1={p} y1={a} x2={p} y2={b} style={{ stroke: c }} strokeWidth="1.5" />
                );
              })}
              <g filter="url(#mh-soft)">
                {f.pool.map((q, i) => (
                  <circle key={i} cx={q.x} cy={q.y} r={q.r} style={{ fill: q.f }} fillOpacity={q.o} />
                ))}
              </g>
              {f.pool.map((q, i) => (
                <circle
                  key={i}
                  cx={q.x}
                  cy={q.y}
                  r="1.3"
                  style={{ fill: "var(--ax-bg-default)" }}
                  fillOpacity={q.o * 0.8}
                />
              ))}
              <circle
                cx={f.fp.x}
                cy={f.fp.y}
                r="5"
                style={{ fill: f.fp.f, stroke: "var(--ax-bg-default)" }}
                fillOpacity={f.fp.o}
                strokeWidth="1.5"
              />
            </svg>

            {TILES.map((_, i) => (
              <span key={i} className="mh-abs mh-ph" style={box(L.tBox(i))} />
            ))}
            {V.map((vw, i) => {
              const T0 = TYPES[vw.ty],
                Ic = T0.ic,
                gone = vw.ph === "out" || vw.ph === "empty";
              return (
                <span
                  key={i}
                  className={
                    "mh-abs mh-tile" +
                    (gone ? " mh-gone" : "") +
                    (T0.agent ? " mh-agent" : "") +
                    (f.glow[i] ? " mh-on" : "") +
                    (i === e.c ? " mh-focus" : "")
                  }
                  style={{ ...box(L.tBox(i)), color: T0.c, ["--mh-c" as string]: T0.c }}
                  title={T0.full}
                >
                  <Ic width={compact ? 18 : 20} height={compact ? 18 : 20} aria-hidden />
                  <span className="mh-lb" style={{ fontSize: compact ? 14 : 11 }}>
                    {T0.short}
                  </span>
                </span>
              );
            })}
            {GATES.map((g, j) => {
              const s = gState(j),
                pos = L.gateLabel(L.gatePos[j]);
              return (
                <div
                  key={j}
                  className={"mh-abs mh-gl" + (s ? " mh-gl-" + s : "")}
                  style={{ left: pos.x, top: pos.y, width: pos.w, justifyContent: pos.align }}
                >
                  <span className="mh-sy">{s === "ok" ? "✓" : s === "no" ? "✕" : s === "run" ? "•" : ""}</span>
                  <span>{g.label}</span>
                </div>
              );
            })}
            {nums.map((n) => (
              <span key={n.n} className="mh-abs mh-num" style={{ left: n.x, top: n.y }}>
                {n.n}
              </span>
            ))}
          </div>
        </div>
      </div>

      {still && (
        <ol className={"mh-steps" + (compact ? "" : " mh-steps-2")}>
          <li>En klient, her nav-pilot, kaller et verktøy på Aksel designsystem.</li>
          <li>
            Tilgang: gatewayen sjekker hvem du er og gir et eget token til akkurat denne serveren. Ditt eget token
            sendes aldri videre.
          </li>
          <li>Godkjente verktøy: bare verktøy eierteamet har godkjent, slipper gjennom.</li>
          <li>Ratelimit og nødstopp: for mange samtidige kall stoppes, og en server kan stenges med en gang.</li>
          <li>Serveren kjører verktøyet, og kallet havner i revisjonsloggen.</li>
        </ol>
      )}

      <HStack gap="space-16" wrap className="mh-legend" aria-hidden>
        <span>
          <svg width="14" height="14">
            <circle cx="7" cy="7" r="5" style={{ fill: C_FOCUS, stroke: "var(--ax-bg-default)" }} strokeWidth="1.5" />
          </svg>
          Kallet du følger
        </span>
        <span>
          <svg width="12" height="12">
            <circle cx="6" cy="6" r="3" style={{ fill: C_COPILOT }} fillOpacity=".6" />
          </svg>
          Andre kall
        </span>
        <span>
          <b className="mh-ok">✓</b>Sjekk bestått: tilgang, godkjente verktøy, ratelimit, nødstopp
        </span>
        <span>
          <svg width="12" height="12">
            <circle cx="6" cy="6" r="4" style={{ fill: C_RED }} />
          </svg>
          <b className="mh-no">✕</b>Avvist i gatewayen
        </span>
        <span>
          <span className="mh-tl mh-tl-hit">tool</span>Verktøyet som kalles
        </span>
        <span>
          <span className="mh-key-agent" />
          Agentprofil
        </span>
      </HStack>

      <div className="mh-log">
        <HStack justify="space-between" align="center" gap="space-8" wrap className="mh-log-head">
          <BodyShort weight="semibold">Revisjonslogg</BodyShort>
          <Switch size="small" checked={live} onChange={(ev) => setLive(ev.target.checked)}>
            Les opp nye linjer
          </Switch>
        </HStack>
        {!compact && (
          <div className="mh-row mh-row-head" aria-hidden>
            <span>client</span>
            <span>route</span>
            <span>tool</span>
            <span>outcome</span>
            <span>decision</span>
          </div>
        )}
        <div aria-live={live ? "polite" : "off"} className="mh-mono">
          {log.map((l) => (
            <div key={l.id} className={"mh-row" + (compact ? " mh-row-c" : "") + (l.fresh ? " mh-row-new" : "")}>
              {compact ? (
                <span>
                  {l.client} · {l.tool}
                </span>
              ) : (
                <>
                  <span>{l.client}</span>
                  <span>{l.route}</span>
                  <span>{l.tool}</span>
                </>
              )}
              <span className={l.rej ? "mh-no" : "mh-ok"}>{l.rej ? "✕ avvist" : "✓ ok"}</span>
              {!compact && <span>{l.why}</span>}
            </div>
          ))}
        </div>
      </div>
    </VStack>
  );
}
