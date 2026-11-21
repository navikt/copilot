import { render, screen, fireEvent } from "@testing-library/react";
import type { EnrichedCustomization } from "@/lib/enrich-customizations";
import { CustomizationCatalog } from "./customization-catalog";

const mockReplace = vi.fn();
let mockSearchParams = new URLSearchParams();

vi.mock("next/navigation", () => ({
  useSearchParams: () => mockSearchParams,
  useRouter: () => ({ replace: mockReplace }),
}));

const ITEMS: EnrichedCustomization[] = [
  {
    id: "aksel-agent",
    name: "aksel",
    description: "Svarer på spørsmål om Aksel Design System",
    type: "agent",
    domain: "frontend",
    filePath: ".github/agents/aksel.agent.md",
    repoPath: "agents/aksel.agent.md",
    rawGitHubUrl: "https://raw.githubusercontent.com/navikt/copilot/main/agents/aksel.agent.md",
    installUrl: "vscode:chat-agent/install?url=https://example.com/aksel",
    insidersInstallUrl: null,
    tools: ["search"],
    usageCount: 38,
    usedBy: ["my-copilot"],
  },
  {
    id: "nais-deploy",
    name: "nais-deploy",
    description: "Nais-manifest, accessPolicy og deploy",
    type: "instruction",
    domain: "platform",
    filePath: ".github/instructions/nais-deploy.instructions.md",
    repoPath: "instructions/nais-deploy.instructions.md",
    rawGitHubUrl: "https://raw.githubusercontent.com/navikt/copilot/main/instructions/nais-deploy.instructions.md",
    installUrl: null,
    insidersInstallUrl: null,
    applyTo: "**/nais*.yaml",
    usageCount: 15,
    usedBy: [],
  },
  {
    id: "mcp-io.github.navikt/kodeverk",
    name: "kodeverk",
    serverId: "io.github.navikt/kodeverk",
    description: "Felles kodeverk gjennom fleet-gateway",
    type: "mcp",
    domain: "general",
    filePath: "",
    repoPath: "",
    rawGitHubUrl: "",
    installUrl: null,
    insidersInstallUrl: null,
    version: "1.0.0",
    remotes: [{ type: "streamable-http", url: "https://fleet-gateway.intern.nav.no/kodeverk" }],
    tools: ["list_kodeverk"],
    usageCount: 10,
    usedBy: [],
  },
  {
    id: "old-skill",
    name: "deprecated-skill",
    description: "Skal ikke vises",
    type: "skill",
    domain: "frontend",
    filePath: "skills/old/SKILL.md",
    repoPath: "skills/old/SKILL.md",
    rawGitHubUrl: "",
    installUrl: null,
    insidersInstallUrl: null,
    deprecated: true,
    usageCount: 0,
    usedBy: [],
  },
];

describe("CustomizationCatalog", () => {
  beforeEach(() => {
    mockReplace.mockReset();
    mockSearchParams = new URLSearchParams();
  });

  it("shows grouped domain sections by default and hides deprecated items", () => {
    render(<CustomizationCatalog items={ITEMS} />);

    expect(screen.getByRole("heading", { level: 1, name: "Hva skal Copilot hjelpe deg med?" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "Plattform" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "Frontend" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "Generelt" })).toBeInTheDocument();
    expect(screen.queryByText("deprecated-skill")).not.toBeInTheDocument();
    expect(screen.queryByText(/Koble til Navs MCP-servere/)).not.toBeInTheDocument();
  });

  it("shows flat list and fleet-gateway info box when MCP Server type chip is toggled", () => {
    render(<CustomizationCatalog items={ITEMS} />);

    fireEvent.click(screen.getByRole("button", { name: /MCP Server/ }));

    expect(screen.getByText(/Koble til Navs MCP-servere/)).toBeInTheDocument();
    expect(screen.getByText(/Tilby den gjennom fleet-gateway/)).toBeInTheDocument();
    expect(screen.getByText("1 av 3 tilpasninger")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 3, name: "kodeverk" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { level: 3, name: "@aksel" })).not.toBeInTheDocument();
  });

  it("supports multi-select type filtering and resetting filters", () => {
    render(<CustomizationCatalog items={ITEMS} />);

    fireEvent.click(screen.getByRole("button", { name: /Agent/ }));
    fireEvent.click(screen.getByRole("button", { name: /Instruksjon/ }));

    expect(screen.getByText("2 av 3 tilpasninger")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 3, name: "@aksel" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 3, name: "nais-deploy" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Nullstill filtre" }));

    expect(screen.getByRole("heading", { level: 2, name: "Frontend" })).toBeInTheDocument();
  });

  it("filters by intent quick-pick and opens domain via Se alle", () => {
    render(<CustomizationCatalog items={ITEMS} />);

    fireEvent.click(screen.getByRole("button", { name: "Deploye til Nais" }));
    expect(screen.getByText("1 av 3 tilpasninger")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 3, name: "nais-deploy" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Nullstill filtre" }));
    fireEvent.click(screen.getAllByRole("button", { name: /Se alle/ })[0]);
    expect(screen.getByText("1 av 3 tilpasninger")).toBeInTheDocument();
  });

  it("restores initial filters from URL search params", () => {
    mockSearchParams = new URLSearchParams("domain=frontend&type=agent&q=aksel");
    render(<CustomizationCatalog items={ITEMS} />);

    expect(screen.getByText("1 av 3 tilpasninger")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 3, name: "@aksel" })).toBeInTheDocument();
  });

  it("honours prefers-reduced-motion by keeping static placeholder", () => {
    vi.useFakeTimers();
    const originalMatchMedia = window.matchMedia;
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: query === "(prefers-reduced-motion: reduce)",
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    }));

    try {
      render(<CustomizationCatalog items={ITEMS} />);
      const input = screen.getByRole("searchbox");
      vi.advanceTimersByTime(2000);
      expect(input).toHaveAttribute("placeholder", "Prøv «deploy til Nais», «Aksel-skjema» eller «testpersoner»");
    } finally {
      window.matchMedia = originalMatchMedia;
      vi.useRealTimers();
    }
  });
});
