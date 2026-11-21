import { render, screen, fireEvent } from "@testing-library/react";
import type { EnrichedCustomization } from "@/lib/enrich-customizations";
import { getAllCustomizations } from "@/lib/customizations";
import { enrichWithUsage } from "@/lib/enrich-customizations";
import { CustomizationCatalog, PLACEHOLDER_EXAMPLES, matchesSearch } from "./customization-catalog";

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

    expect(screen.getByRole("heading", { level: 1, name: "Hva skal nav-pilot hjelpe deg med?" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "Plattform" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "Frontend" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "Generelt" })).toBeInTheDocument();
    expect(screen.queryByText("deprecated-skill")).not.toBeInTheDocument();
    expect(screen.getByText(/Koble til Navs MCP-servere/)).toBeInTheDocument();
  });

  it("shows flat list and fleet-gateway info box when MCP-server type chip is toggled", () => {
    render(<CustomizationCatalog items={ITEMS} />);

    fireEvent.click(screen.getByRole("button", { name: /MCP-server/, pressed: false }));

    expect(screen.getByText(/Koble til Navs MCP-servere/)).toBeInTheDocument();
    expect(screen.getByText(/Tilby den gjennom fleet-gateway/)).toBeInTheDocument();
    expect(screen.getByText("1 av 3 verktøy")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 3, name: "kodeverk" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { level: 3, name: "@aksel" })).not.toBeInTheDocument();
  });

  it("supports multi-select type filtering and resetting filters", () => {
    render(<CustomizationCatalog items={ITEMS} />);

    fireEvent.click(screen.getByRole("button", { name: /Agent/, pressed: false }));
    fireEvent.click(screen.getByRole("button", { name: /Instruksjon/, pressed: false }));

    expect(screen.getByText("2 av 3 verktøy")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 3, name: "@aksel" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 3, name: "nais-deploy" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Nullstill filtre" }));

    expect(screen.getByRole("heading", { level: 2, name: "Frontend" })).toBeInTheDocument();
  });

  it("filters by intent quick-pick and opens domain via Se alle", () => {
    render(<CustomizationCatalog items={ITEMS} />);

    fireEvent.click(screen.getByRole("button", { name: "Deploye til Nais" }));
    expect(screen.getByText("1 av 3 verktøy")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 3, name: "nais-deploy" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Nullstill filtre" }));
    fireEvent.click(screen.getAllByRole("button", { name: /Se alle/ })[0]);
    expect(screen.getByText("1 av 3 verktøy")).toBeInTheDocument();
  });

  it("restores initial filters from URL search params", () => {
    mockSearchParams = new URLSearchParams("domain=frontend&type=agent&q=aksel");
    render(<CustomizationCatalog items={ITEMS} />);

    expect(screen.getByText("1 av 3 verktøy")).toBeInTheDocument();
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
      expect(input).toHaveAttribute("placeholder", "Prøv «deploy Nais», «Aksel» eller «trusselmodell»");
    } finally {
      window.matchMedia = originalMatchMedia;
      vi.useRealTimers();
    }
  });

  it("opens the drawer from ?item= and closes with the close button and Esc", () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response("[]"));
    mockSearchParams = new URLSearchParams("item=agent:aksel-agent");
    const { unmount } = render(<CustomizationCatalog items={ITEMS} />);
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Lukk/ }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    unmount();

    render(<CustomizationCatalog items={ITEMS} />);
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    vi.restoreAllMocks();
  });

  it("filters on a legacy ?domain= link alone", () => {
    mockSearchParams = new URLSearchParams("domain=platform");
    render(<CustomizationCatalog items={ITEMS} />);
    expect(screen.getByText("1 av 3 verktøy")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 3, name: "nais-deploy" })).toBeInTheDocument();
  });

  it("sorts by most used by default", () => {
    mockSearchParams = new URLSearchParams("q=a");
    render(<CustomizationCatalog items={ITEMS} />);
    const names = screen.getAllByRole("heading", { level: 3 }).map((h) => h.textContent);
    expect(names).toEqual(["@aksel", "nais-deploy", "kodeverk"]);
  });

  it("sets aria-pressed on a type chip after a click", () => {
    render(<CustomizationCatalog items={ITEMS} />);
    const chip = screen.getByRole("button", { name: /Agent/, pressed: false });
    fireEvent.click(chip);
    expect(chip).toHaveAttribute("aria-pressed", "true");
  });

  it("names Se alle with the count and domain", () => {
    render(<CustomizationCatalog items={ITEMS} />);
    expect(screen.getByRole("button", { name: "Se alle 1 i Plattform" })).toBeInTheDocument();
  });

  it.each(PLACEHOLDER_EXAMPLES)("example search %s gives at least one hit in the catalog", (example) => {
    const catalog = enrichWithUsage(getAllCustomizations(), []);
    expect(catalog.filter((item) => matchesSearch(item, example)).length).toBeGreaterThan(0);
  });

  it("matches each search term on its own", () => {
    mockSearchParams = new URLSearchParams("q=deploy%20nais");
    render(<CustomizationCatalog items={ITEMS} />);
    expect(screen.getByRole("heading", { level: 3, name: "nais-deploy" })).toBeInTheDocument();
  });

  it("moves focus to the results heading after Se alle", () => {
    render(<CustomizationCatalog items={ITEMS} />);
    fireEvent.click(screen.getByRole("button", { name: "Se alle 1 i Plattform" }));
    expect(screen.getByRole("heading", { level: 2, name: "Treff" })).toHaveFocus();
  });
});
