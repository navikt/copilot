/**
 * Static diagrams for the fleet-gateway pilot page. The text lists on the page are the text alternative;
 * colours come from Aksel tokens so the diagrams follow the active theme.
 */

const STYLE = `
  .fg-box { fill: var(--ax-bg-default); stroke: var(--ax-border-neutral); stroke-width: 1.5; }
  .fg-fleet { fill: var(--ax-bg-accent-soft); stroke: var(--ax-border-accent-subtle); stroke-width: 1.5; }
  .fg-host { fill: var(--ax-bg-neutral-soft); stroke: var(--ax-border-neutral-subtle); stroke-width: 1.5; stroke-dasharray: 6 4; }
  .fg-title { fill: var(--ax-text-neutral); font-size: 20px; font-weight: 700; }
  .fg-sub { fill: var(--ax-text-neutral-subtle); font-size: 17px; }
  .fg-group { fill: var(--ax-text-accent); font-size: 18px; font-weight: 700; }
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
      <text x={cx} y={y + 32} textAnchor="middle" className="fg-title">
        {title}
      </text>
      {sub.map((line, i) => (
        <text key={line} x={cx} y={y + 58 + i * 21} textAnchor="middle" className="fg-sub">
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
      viewBox="0 0 970 370"
      title="Hvem slipper inn: teamet sender inn MCP-serveren til fleet-control, team copilot godkjenner, fleet-gateway håndhever pilotlisten per rute, Nais accessPolicy åpner nettverket, og Texas utsteder tokenene."
    >
      <Defs id={m} />
      <Box x={10} y={70} w={150} h={100} title="Teamet" sub={["sender inn", "MCP-server"]} />

      <rect x={190} y={20} width={500} height={180} rx="14" className="fg-fleet" />
      <text x={210} y={50} className="fg-group">
        nav-fleet
      </text>
      <Box x={205} y={65} w={235} h={110} title="fleet-control" sub={["registeret", "team copilot godkjenner"]} />
      <Box x={460} y={65} w={215} h={110} title="fleet-gateway" sub={["håndhever", "pilotliste per rute"]} />

      <Box x={755} y={65} w={205} h={110} title="Teamets" sub={["MCP-server"]} />

      <line x1={160} y1={120} x2={203} y2={120} className="fg-line" markerEnd={arrow} />
      <line x1={440} y1={120} x2={458} y2={120} className="fg-line" markerEnd={arrow} />
      <line x1={675} y1={120} x2={753} y2={120} className="fg-line" markerEnd={arrow} />

      <Box
        x={620}
        y={245}
        w={290}
        h={110}
        title="Nais accessPolicy"
        sub={["fleet-gateway: outbound", "teamets server: inbound"]}
      />
      <line x1={640} y1={177} x2={680} y2={243} className="fg-line fg-dash" markerStart={arrow} markerEnd={arrow} />
      <line x1={860} y1={177} x2={830} y2={243} className="fg-line fg-dash" markerStart={arrow} markerEnd={arrow} />

      <Box x={230} y={245} w={260} h={90} title="Texas" sub={["utsteder tokenene"]} />
      <line x1={490} y1={275} x2={560} y2={179} className="fg-line fg-dash" markerEnd={arrow} />
    </Figure>
  );
}

export function FleetGatewayCallDiagram() {
  const m = "fg-arrow-call";
  const arrow = `url(#${m})`;
  return (
    <Figure
      titleId="fg-call-title"
      viewBox="0 0 970 350"
      title="Slik går et kall: utviklerens agent kaller cplt på vertsmaskinen, cplt kaller fleet-gateway, der brukeren logger inn med Nav-konto via Entra, fleet-gateway henter et OBO-token fra Texas og kaller teamets MCP-server, som sjekker aud, idtyp, azp og NAVident."
    >
      <Defs id={m} />
      <rect x={10} y={20} width={400} height={180} rx="14" className="fg-host" />
      <text x={30} y={50} className="fg-group">
        Utviklermaskin
      </text>
      <Box x={25} y={70} w={150} h={110} title="Agenten" sub={["i sandkassen"]} />
      <Box x={230} y={70} w={165} h={110} title="cplt" sub={["på vertsmaskinen", "holder tokenene"]} />

      <rect x={440} y={20} width={240} height={170} rx="14" className="fg-fleet" />
      <text x={460} y={50} className="fg-group">
        nav-fleet
      </text>
      <Box x={455} y={65} w={210} h={110} title="fleet-gateway" sub={["innlogging med", "Nav-konto via Entra"]} />
      <Box x={440} y={235} w={240} h={100} title="Texas" sub={["veksler til OBO-token", "for teamets server"]} />

      <Box x={740} y={65} w={225} h={130} title="Teamets MCP-server" sub={["sjekker aud, idtyp,", "azp og NAVident"]} />

      <line x1={175} y1={125} x2={228} y2={125} className="fg-line" markerEnd={arrow} />
      <line x1={395} y1={125} x2={453} y2={125} className="fg-line" markerEnd={arrow} />
      <line x1={560} y1={177} x2={560} y2={233} className="fg-line" markerStart={arrow} markerEnd={arrow} />
      <line x1={665} y1={125} x2={738} y2={125} className="fg-line" markerEnd={arrow} />
      <text x={710} y={113} textAnchor="middle" className="fg-sub">
        OBO
      </text>
    </Figure>
  );
}

export function FleetGatewayTeamDiagram() {
  const m = "fg-arrow-team";
  const arrow = `url(#${m})`;
  return (
    <Figure
      titleId="fg-team-title"
      viewBox="0 0 970 300"
      title="Kallet til teamets server: cplt kaller fleet-gateway, fleet-gateway veksler tokenet hos Texas til et OBO-token for teamets app, og kaller teamets app. Steg 1, accessPolicy.inbound, slipper inn fleet-gateway. Steg 2, sjekk av tokenet med Texas-introspeksjon, skjer i teamets app."
    >
      <Defs id={m} />
      <Box x={10} y={60} w={170} h={100} title="cplt" sub={["klienten"]} />
      <Box x={260} y={60} w={210} h={100} title="fleet-gateway" sub={["innlogging, pilotliste"]} />
      <Box x={260} y={200} w={210} h={90} title="Texas" sub={["OBO-veksling"]} />

      <rect x={560} y={20} width={400} height={270} rx="14" className="fg-host" />
      <text x={580} y={50} className="fg-group">
        Teamets namespace
      </text>
      <Box x={590} y={70} w={340} h={80} title="Steg 1: accessPolicy.inbound" sub={["slipper inn fleet-gateway"]} />
      <Box
        x={590}
        y={180}
        w={340}
        h={95}
        title="Steg 2: teamets app"
        sub={["sjekker tokenet med Texas:", "aud, idtyp, azp, NAVident"]}
      />

      <line x1={180} y1={110} x2={258} y2={110} className="fg-line" markerEnd={arrow} />
      <line x1={365} y1={160} x2={365} y2={198} className="fg-line" markerStart={arrow} markerEnd={arrow} />
      <line x1={470} y1={110} x2={588} y2={110} className="fg-line" markerEnd={arrow} />
      <text x={530} y={98} textAnchor="middle" className="fg-sub">
        OBO
      </text>
      <line x1={760} y1={150} x2={760} y2={178} className="fg-line" markerEnd={arrow} />
    </Figure>
  );
}
