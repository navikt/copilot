/**
 * Static diagrams for the fleet-gateway pilot page. The text lists on the page are the text alternative;
 * colours come from Aksel tokens so the diagrams follow the active theme.
 */

const STYLE = `
  .fg-box { fill: var(--ax-bg-default); stroke: var(--ax-border-neutral-subtle); stroke-width: 1.5; }
  .fg-fleet { fill: var(--ax-bg-accent-soft); stroke: var(--ax-border-accent-subtle); stroke-width: 1.5; }
  .fg-host { fill: var(--ax-bg-neutral-soft); stroke: var(--ax-border-neutral-subtle); stroke-width: 1.5; stroke-dasharray: 6 4; }
  .fg-title { fill: var(--ax-text-neutral); font-size: 15px; font-weight: 700; }
  .fg-sub { fill: var(--ax-text-neutral-subtle); font-size: 12px; }
  .fg-group { fill: var(--ax-text-accent); font-size: 13px; font-weight: 700; }
  .fg-line { stroke: var(--ax-text-neutral-subtle); stroke-width: 1.5; fill: none; }
  .fg-dash { stroke-dasharray: 4 4; }
  .fg-head { fill: var(--ax-text-neutral-subtle); }
`;

function Defs({ id }: { id: string }) {
  return (
    <defs>
      <style>{STYLE}</style>
      <marker id={id} markerWidth="10" markerHeight="8" refX="9" refY="4" orient="auto-start-reverse">
        <path d="M0 0 L10 4 L0 8 z" className="fg-head" />
      </marker>
    </defs>
  );
}

function Box({ x, y, w, h, title, sub }: { x: number; y: number; w: number; h: number; title: string; sub: string[] }) {
  const cx = x + w / 2;
  return (
    <g>
      <rect x={x} y={y} width={w} height={h} rx="10" className="fg-box" />
      <text x={cx} y={y + 26} textAnchor="middle" className="fg-title">
        {title}
      </text>
      {sub.map((line, i) => (
        <text key={line} x={cx} y={y + 46 + i * 16} textAnchor="middle" className="fg-sub">
          {line}
        </text>
      ))}
    </g>
  );
}

function Figure({
  titleId,
  title,
  viewBox,
  children,
}: {
  titleId: string;
  title: string;
  viewBox: string;
  children: React.ReactNode;
}) {
  return (
    <div className="w-full overflow-x-auto">
      <svg
        viewBox={viewBox}
        className="w-full min-w-[640px]"
        style={{ display: "block" }}
        role="img"
        aria-labelledby={titleId}
      >
        <title id={titleId}>{title}</title>
        {children}
      </svg>
    </div>
  );
}

export function FleetGatewayAccessDiagram() {
  const m = "fg-arrow-access";
  const arrow = `url(#${m})`;
  return (
    <Figure
      titleId="fg-access-title"
      viewBox="0 0 900 330"
      title="Hvem slipper inn: teamet sender inn MCP-serveren til fleet-control, team copilot godkjenner, fleet-gateway håndhever pilotlisten per rute, Nais accessPolicy åpner nettverket, og Texas utsteder tokenene."
    >
      <Defs id={m} />
      <Box x={10} y={60} w={150} h={80} title="Teamet" sub={["sender inn", "MCP-server"]} />

      <rect x={200} y={20} width={460} height={160} rx="14" className="fg-fleet" />
      <text x={220} y={44} className="fg-group">
        nav-fleet
      </text>
      <Box x={220} y={60} w={190} h={96} title="fleet-control" sub={["registeret", "team copilot godkjenner"]} />
      <Box x={450} y={60} w={190} h={96} title="fleet-gateway" sub={["håndhever", "pilotliste per rute"]} />

      <Box x={730} y={60} w={160} h={96} title="Teamets" sub={["MCP-server"]} />

      <line x1={160} y1={100} x2={218} y2={100} className="fg-line" markerEnd={arrow} />
      <line x1={410} y1={108} x2={448} y2={108} className="fg-line" markerEnd={arrow} />
      <line x1={640} y1={108} x2={728} y2={108} className="fg-line" markerEnd={arrow} />

      <Box
        x={560}
        y={210}
        w={240}
        h={96}
        title="Nais accessPolicy"
        sub={["gateway: outbound", "teamets server: inbound"]}
      />
      <line x1={600} y1={156} x2={630} y2={208} className="fg-line fg-dash" markerStart={arrow} markerEnd={arrow} />
      <line x1={790} y1={156} x2={760} y2={208} className="fg-line fg-dash" markerStart={arrow} markerEnd={arrow} />

      <Box x={240} y={210} w={240} h={80} title="Texas" sub={["utsteder tokenene"]} />
      <line x1={480} y1={240} x2={520} y2={158} className="fg-line fg-dash" markerEnd={arrow} />
    </Figure>
  );
}

export function FleetGatewayCallDiagram() {
  const m = "fg-arrow-call";
  const arrow = `url(#${m})`;
  return (
    <Figure
      titleId="fg-call-title"
      viewBox="0 0 960 330"
      title="Slik går et kall: utviklerens agent kaller cplt på vertsmaskinen, cplt kaller fleet-gateway, der brukeren logger inn med Nav-konto via Entra, fleet-gateway henter et OBO-token fra Texas og kaller teamets MCP-server, som sjekker aud, idtyp, azp og NAVident."
    >
      <Defs id={m} />
      <rect x={10} y={20} width={360} height={160} rx="14" className="fg-host" />
      <text x={30} y={44} className="fg-group">
        Utviklermaskin
      </text>
      <Box x={30} y={60} w={150} h={96} title="Agenten" sub={["i sandkassen"]} />
      <Box x={200} y={60} w={150} h={96} title="cplt" sub={["på vertsmaskinen", "holder tokenene"]} />

      <rect x={410} y={20} width={260} height={290} rx="14" className="fg-fleet" />
      <text x={430} y={44} className="fg-group">
        nav-fleet
      </text>
      <Box x={430} y={60} w={220} h={96} title="fleet-gateway" sub={["innlogging med", "Nav-konto via Entra"]} />
      <Box x={430} y={200} w={220} h={90} title="Texas" sub={["veksler til OBO-token", "for teamets server"]} />

      <Box x={720} y={60} w={230} h={130} title="Teamets MCP-server" sub={["sjekker aud, idtyp,", "azp og NAVident"]} />

      <line x1={180} y1={108} x2={198} y2={108} className="fg-line" markerEnd={arrow} />
      <line x1={350} y1={108} x2={428} y2={108} className="fg-line" markerEnd={arrow} />
      <line x1={540} y1={156} x2={540} y2={198} className="fg-line" markerStart={arrow} markerEnd={arrow} />
      <line x1={650} y1={108} x2={718} y2={108} className="fg-line" markerEnd={arrow} />
      <text x={684} y={98} textAnchor="middle" className="fg-sub">
        OBO
      </text>
    </Figure>
  );
}
