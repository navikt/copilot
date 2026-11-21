import { BooksIcon, PersonGroupIcon, TestFlaskIcon } from "@navikt/aksel-icons";

// Norwegian copy for the gateway servers in apps/mcp-registry/allowlist.json.
// servers.test.ts fails if a server here is missing from the registry or the prod routes.
export const PILOT_SERVERS = [
  {
    title: "Felles kodeverk",
    id: "io.github.navikt/kodeverk",
    route: "/mcp/copilot/kodeverk",
    icon: BooksIcon,
    summary: "Slår opp og søker i Navs felles kodeverk, koder, betydninger og hierarkier.",
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
      "Lister gruppene dine i Dolly, viser testpersonene i en gruppe og sjekker bestillinger. Kan bestille testpersoner etter at du har godkjent en forhåndsvisning.",
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
    summary: "Slår opp team, produktområder og klynger i Teamkatalogen.",
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
