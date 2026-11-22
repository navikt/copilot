/**
 * The hero diagram on /mcp: client → fleet-gateway → servers. Two drawings, wide and tall, so the text stays
 * readable on a phone. CSS shows one. Colours come from Aksel tokens, so the diagram follows the theme.
 * The dashes only move when the reader has not asked for reduced motion.
 */

const STYLE = `
  .mh-box { fill: var(--ax-bg-default); stroke: var(--ax-border-neutral); stroke-width: 2; }
  .mh-gw { fill: var(--ax-bg-accent-strong); }
  .mh-srv { fill: var(--ax-bg-accent-soft); stroke: var(--ax-border-accent); stroke-width: 1.5; }
  .mh-title { fill: var(--ax-text-neutral); font-size: 20px; font-weight: 600; }
  .mh-sub { fill: var(--ax-text-neutral-subtle); font-size: 16px; }
  .mh-gw-title { fill: var(--ax-text-accent-contrast); font-size: 22px; font-weight: 700; }
  .mh-gw-sub { fill: var(--ax-text-accent-contrast); font-size: 16px; }
  .mh-invite { fill: var(--ax-bg-neutral-soft); stroke: var(--ax-border-neutral); stroke-width: 1.5; stroke-dasharray: 6 5; }
  .mh-invite-link:focus-visible { outline: 3px solid var(--ax-border-focus); outline-offset: 2px; }
  .mh-invite-link:hover .mh-invite { stroke: var(--ax-border-accent); }
  .mh-flow-invite { opacity: 0.45; stroke-dasharray: 3 8; }
  .mh-flow { stroke: var(--ax-border-accent); stroke-width: 3; fill: none; stroke-dasharray: 8 10; }
  @media (prefers-reduced-motion: no-preference) {
    .mh-flow { animation: mh-dash 1.6s linear infinite; }
  }
  @keyframes mh-dash { to { stroke-dashoffset: -36; } }
`;

const DESC =
  "En MCP-klient, som Copilot CLI eller OpenCode, kobler seg til fleet-gateway. Du logger inn én gang. " +
  "fleet-gateway sjekker innloggingen, logger kallet og sender det videre til riktig MCP-server med et token på vegne av deg. " +
  "Den siste boksen, «Ditt verktøy?», lenker til veiledningen for team som vil tilby en server.";

function Client({ x, y, w }: { x: number; y: number; w: number }) {
  return (
    <g>
      <rect x={x} y={y} width={w} height={90} rx="10" className="mh-box" />
      <text x={x + w / 2} y={y + 40} textAnchor="middle" className="mh-title">
        MCP-klient
      </text>
      <text x={x + w / 2} y={y + 66} textAnchor="middle" className="mh-sub">
        Copilot CLI, OpenCode
      </text>
    </g>
  );
}

function Gateway({ x, y, w }: { x: number; y: number; w: number }) {
  return (
    <g>
      <rect x={x} y={y} width={w} height={110} rx="12" className="mh-gw" />
      <text x={x + w / 2} y={y + 50} textAnchor="middle" className="mh-gw-title">
        fleet-gateway
      </text>
      <text x={x + w / 2} y={y + 78} textAnchor="middle" className="mh-gw-sub">
        innlogging · logg · OBO
      </text>
    </g>
  );
}

function Server({ x, y, w, title, invite }: { x: number; y: number; w: number; title: string; invite?: boolean }) {
  const box = (
    <g>
      <rect x={x} y={y} width={w} height={52} rx="8" className={invite ? "mh-invite" : "mh-srv"} />
      <text x={x + w / 2} y={y + 32} textAnchor="middle" className="mh-title">
        {title}
      </text>
    </g>
  );
  return invite ? (
    <a
      href="/mcp/tilby-server"
      className="mh-invite-link"
      aria-label="Ditt verktøy? Les hvordan teamet ditt tilbyr en MCP-server"
    >
      {box}
    </a>
  ) : (
    box
  );
}

const INVITE = "Ditt verktøy?";

export function McpHeroDiagram({ servers: pilot }: { servers: string[] }) {
  // The last box invites teams to offer their own server.
  const servers = [...pilot, INVITE];
  const n = servers.length;
  const flow = (s: string) => (s === INVITE ? "mh-flow mh-flow-invite" : "mh-flow");
  // Wide: 960 × 340. Servers in a column on the right, centred on the gateway.
  const wideTop = 170 - (n * 52 + (n - 1) * 20) / 2;
  // Tall: 360 × (300 + n·72). Servers stacked under the gateway, fed from a trunk on the left.
  const tallH = 300 + n * 72;
  return (
    <div className="mcp-hero-diagram">
      <style>{STYLE}</style>
      <svg
        className="mcp-hero-wide"
        viewBox="0 0 960 340"
        role="group"
        aria-labelledby="mh-wt"
        aria-describedby="mh-wd"
      >
        <title id="mh-wt">Slik går et kall gjennom fleet-gateway, og plass til ditt verktøy</title>
        <desc id="mh-wd">{DESC}</desc>
        <path className="mh-flow" d="M240 170 H380" />
        {servers.map((s, i) => {
          const cy = wideTop + i * 72 + 26;
          return <path key={s} className={flow(s)} d={`M580 170 C650 170 670 ${cy} 740 ${cy}`} />;
        })}
        <Client x={20} y={125} w={220} />
        <Gateway x={380} y={115} w={200} />
        {servers.map((s, i) => (
          <Server key={s} x={740} y={wideTop + i * 72} w={200} title={s} invite={s === INVITE} />
        ))}
      </svg>
      <svg
        className="mcp-hero-tall"
        viewBox={`0 0 360 ${tallH}`}
        role="group"
        aria-labelledby="mh-tt"
        aria-describedby="mh-td"
      >
        <title id="mh-tt">Slik går et kall gjennom fleet-gateway, og plass til ditt verktøy</title>
        <desc id="mh-td">{DESC}</desc>
        <path className="mh-flow" d="M180 100 V150" />
        {servers.map((s, i) => {
          const cy = 300 + i * 72 + 26;
          return <path key={s} className={flow(s)} d={`M120 260 C120 280 30 280 30 300 V${cy} H70`} />;
        })}
        <Client x={50} y={10} w={260} />
        <Gateway x={60} y={150} w={240} />
        {servers.map((s, i) => (
          <Server key={s} x={70} y={300 + i * 72} w={270} title={s} invite={s === INVITE} />
        ))}
      </svg>
    </div>
  );
}
