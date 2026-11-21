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
      "Viser grupper, testpersoner og bestillinger i Dolly. Bestiller testpersoner når du har godkjent en forhåndsvisning.",
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
  readOnly: boolean;
  example: string;
  codeLabel: string;
  inboundPrUrl?: string;
  inboundPrLabel?: string;
}[];
