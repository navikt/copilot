import { BooksIcon, PersonGroupIcon, TestFlaskIcon } from "@navikt/aksel-icons";

// Norwegian copy for the gateway servers in apps/mcp-registry/allowlist.json.
// fleet-gateway.test.tsx fails if a server here is missing from the registry or the prod routes.
export const PILOT_SERVERS = [
  {
    title: "Felles kodeverk",
    id: "io.github.navikt/kodeverk",
    route: "/mcp/copilot/kodeverk",
    icon: BooksIcon,
    summary: "Slår opp koder, betydninger og hierarkier i Navs felles kodeverk.",
    readOnly: true,
    example: "Hvilke koder og betydninger finnes i kodeverket Landkoder?",
    codeLabel: "apps/mcp-kodeverk",
    inboundPrUrl: "https://github.com/navikt/felleskodeverk/pull/1298",
    inboundPrLabel: "navikt/felleskodeverk#1298",
  },
  {
    title: "Dolly",
    id: "io.github.navikt/dolly",
    route: "/mcp/copilot/dolly",
    icon: TestFlaskIcon,
    summary:
      "Viser grupper, testpersoner og bestillinger i Dolly. Bestiller testpersoner når du har bekreftet i klienten. Bestilling krever Copilot CLI 1.0.95 eller nyere. I autopilot kan klienten bekrefte for deg, så slå den av hvis du vil bekrefte selv.",
    // The /mcp card shows this; the full summary stays on the setup cards on /mcp/fleet-gateway.
    short: "Søk i og bestill syntetiske testpersoner.",
    readOnly: false,
    example: "Hvilke testpersoner ligger i Dolly-gruppen til teamet mitt?",
    codeLabel: "apps/mcp-dolly",
    inboundPrUrl: "https://github.com/navikt/testnorge/pull/4427",
    inboundPrLabel: "navikt/testnorge#4427",
  },
  {
    title: "Teamkatalogen",
    id: "io.github.navikt/team-catalog",
    route: "/mcp/copilot/team-catalog",
    icon: PersonGroupIcon,
    summary: "Slår opp team, produktområder, klynger og Nais-team i Teamkatalogen.",
    readOnly: true,
    example: "Hvilket team i Teamkatalogen hører til Nais-teamet copilot?",
    codeLabel: "apps/mcp-team-catalog",
  },
] as const satisfies {
  title: string;
  id: string;
  route: string;
  icon: typeof BooksIcon;
  summary: string;
  short?: string;
  readOnly: boolean;
  example: string;
  codeLabel: string;
  inboundPrUrl?: string;
  inboundPrLabel?: string;
}[];

/** The nav-pilot command that turns on an MCP server. Shared by the gateway cards and the /verktoy drawer. */
export const enableCommand = (server: string) => `nav-pilot mcp enable ${server}`;
