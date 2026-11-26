import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { vi } from "vitest";

const cookieJar = new Map<string, string>();
const nav = vi.hoisted(() => ({ path: "/fleet" }));
vi.mock("next/headers", () => ({
  headers: vi.fn(),
  cookies: vi.fn(async () => ({ get: (k: string) => (cookieJar.has(k) ? { value: cookieJar.get(k) } : undefined) })),
}));
vi.mock("next/navigation", () => ({
  notFound: vi.fn(() => {
    throw new Error("NEXT_NOT_FOUND");
  }),
  redirect: vi.fn((to: string) => {
    throw new Error(`NEXT_REDIRECT ${to}`);
  }),
  permanentRedirect: vi.fn((to: string) => {
    throw new Error(`NEXT_PERMANENT_REDIRECT ${to}`);
  }),
  usePathname: () => nav.path,
  useRouter: () => ({ refresh: vi.fn(), replace: vi.fn(), push: vi.fn() }),
}));

import FleetPage from "./page";
import ServerPage from "./mcp-servere/[namespace]/[name]/page";
import AgentPage from "./agenter/[namespace]/[name]/page";
import OldRoutePage from "./ruter/[namespace]/[name]/page";
import CatalogPage from "./katalog/page";
import { ServerForm, ServerInbox } from "./fleet-ui";
import type { MCPServer } from "@/lib/fleet-control";

const API = "/apis/fleet.nav.no/v1alpha1";
const T = "2026-11-20T10:00:00Z";
const cond = (type: string, on: boolean, reason = "Test") => ({
  type,
  status: on ? "True" : "False",
  reason,
  observedGeneration: 3,
  lastTransitionTime: T,
});
const ref = { group: "fleet.nav.no", kind: "MCPServer", namespace: "team-a", name: "docs" };

// Fake values only: the repo is public.
const server = (revoked = false) => ({
  apiVersion: "fleet.nav.no/v1alpha1",
  kind: "MCPServer",
  metadata: { namespace: "team-a", name: "docs", generation: 3, resourceVersion: "9" },
  spec: {},
  status: {
    publishedGeneration: 2,
    route: { path: "/mcp/team-a/docs" },
    pending: { generation: 3, contentHash: "h3", spec: { dataCategory: "Intern", allowedTools: ["search"] } },
    conditions: [cond("Published", !revoked), cond("PendingApproval", true), cond("Revoked", revoked)],
  },
});

const profile = (granted: Record<string, unknown> = { granted: false }) => ({
  apiVersion: "fleet.nav.no/v1alpha1",
  kind: "AgentProfile",
  metadata: { namespace: "aksel", name: "studio" },
  spec: {
    identity: { cluster: "dev-gcp", app: "studio-api", entraClientID: "00000000-0000-0000-0000-000000000000" },
    mode: "Delegated",
    grants: [{ serverRef: ref, allowedTools: ["search"], maxRisk: "Read" }],
    quota: { maxInFlight: 2, requestsPerMinute: 60, dailyCallsPerUser: 100, dailyCalls: 1000 },
  },
  status: { pending: { generation: 1 }, grants: [{ serverRef: ref, ...granted }] },
});

const entry = (namespace: string, name: string, dataCategory = "Intern") => ({
  apiVersion: "fleet.nav.no/v1alpha1",
  kind: "MCPServerCatalogEntry",
  metadata: { namespace, name },
  spec: { dataCategory, allowedTools: ["hent_sak"] },
  status: { publishedGeneration: 1, route: { path: `/mcp/${namespace}/${name}` } },
});

const routes: Record<string, unknown> = {
  [`${API}/mcpservercatalogentries`]: { items: [entry("team-a", "docs"), entry("team-b", "saker", "Fortrolig")] },
  [`${API}/namespaces/team-b/mcpservercatalogentries/saker`]: {
    ...entry("team-b", "saker", "Fortrolig"),
    spec: {
      dataCategory: "Fortrolig",
      allowedTools: ["hent_sak"],
      tools: [{ name: "hent_sak", description: "Henter én sak." }],
    },
    status: {
      publishedGeneration: 1,
      route: { path: "/mcp/team-b/saker" },
      consumers: [{ namespace: "team-a", name: "studio" }],
    },
  },
  [`${API}/whoami`]: { name: "A123456", oid: "x", teams: ["team-a"] },
  [`${API}/namespaces/team-a/mcpservers`]: { items: [server()] },
  [`${API}/namespaces/team-a/mcpservers/docs`]: server(),
  [`${API}/namespaces/team-a/agentprofiles`]: { items: [] },
  [`${API}/agentprofiles?awaitingApprovalBy=me`]: { items: [profile()] },
  [`${API}/namespaces/team-a/mcpservers/docs/history`]: {
    items: [{ at: T, actor: { name: "A123456" }, action: "draft_submitted", generation: 3 }],
  },
  [`${API}/namespaces/aksel/agentprofiles/studio`]: profile(),
};

function stub(overrides: Record<string, unknown> = {}, status: Record<string, number> = {}) {
  const all = { ...routes, ...overrides };
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) => {
      const path = url.replace("http://fc", "");
      if (status[path]) return new Response("{}", { status: status[path] });
      return path in all ? new Response(JSON.stringify(all[path])) : new Response("{}", { status: 404 });
    })
  );
}

const sp = (team?: string) => ({ searchParams: Promise.resolve(team === undefined ? {} : { team }) });
const params = (namespace: string, name: string) => ({ params: Promise.resolve({ namespace, name }) });

describe("/fleet", () => {
  beforeEach(() => {
    nav.path = "/fleet";
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("FLEET_CONTROL_URL", "http://fc");
    cookieJar.clear();
    stub();
  });

  it("is 404 on every page without FLEET_CONTROL_URL", async () => {
    vi.stubEnv("FLEET_CONTROL_URL", "");
    await expect(FleetPage(sp())).rejects.toThrow("NEXT_NOT_FOUND");
    await expect(ServerPage(params("team-a", "docs"))).rejects.toThrow("NEXT_NOT_FOUND");
    await expect(AgentPage(params("aksel", "studio"))).rejects.toThrow("NEXT_NOT_FOUND");
  });

  it("calls only the v1alpha1 API, never /admin/v1", async () => {
    render(await FleetPage(sp()));
    render(await ServerPage(params("team-a", "docs")));
    render(await AgentPage(params("aksel", "studio")));
    const urls = vi.mocked(fetch).mock.calls.map((c) => String(c[0]));
    expect(urls.length).toBeGreaterThan(0);
    expect(urls.every((u) => u.startsWith(`http://fc${API}/`))).toBe(true);
  });

  it("wraps every table in its own scroll box, so a phone never scrolls the page sideways", async () => {
    const { container } = render(
      <>
        {await FleetPage(sp())}
        {await ServerPage(params("team-a", "docs"))}
        {await AgentPage(params("aksel", "studio"))}
        {await CatalogPage()}
      </>
    );
    const tables = [...container.querySelectorAll("table")];
    expect(tables.length).toBeGreaterThan(3);
    for (const t of tables) expect(t.parentElement).toHaveAttribute("data-fleet-table");
  });

  it("shows Kort fortalt, the inbox and the team's MCP servers with status", async () => {
    render(await FleetPage(sp()));
    expect(screen.getByText("Kort fortalt: hva er Fleet?")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Til godkjenning" })).toBeInTheDocument();
    // Main: the stat strip and the team's own versions waiting for approval.
    expect(screen.getByText("Publisert")).toBeInTheDocument();
    // The agent profiles waiting for «gi tilgang» moved to /fleet/agenter; the overview links there.
    expect(screen.getByRole("link", { name: "Se agentprofilene" })).toHaveAttribute(
      "href",
      "/fleet/agenter?team=team-a#venter-pa-tilgang"
    );
    expect(
      within(screen.getByRole("list", { name: "MCP-servere til godkjenning" })).getByRole("button", {
        name: "Se endringen",
      })
    ).toHaveAttribute("href", "/fleet/mcp-servere/team-a/docs");
    expect(screen.getByRole("heading", { name: "team-a" })).toBeInTheDocument();
    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
    // The header button and the side navigation's button; Aksel's Button as a link keeps role="button".
    expect(screen.getAllByRole("button", { name: "Registrer MCP-server" }).map((b) => b.getAttribute("href"))).toEqual([
      "/fleet/registrer?team=team-a",
      "/fleet/registrer?team=team-a",
    ]);
    expect(screen.getByRole("navigation", { name: "Brødsmuler" })).toHaveTextContent("Fleet");
    expect(screen.getByRole("navigation", { name: "Fleet" })).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "docs" })[0]).toHaveAttribute("href", "/fleet/mcp-servere/team-a/docs");
    // Table and phone list both render; CSS picks one.
    expect(screen.getAllByText("Ny versjon venter")).toHaveLength(2);
    // team-a has an MCP server and no agent profiles: no empty section.
    expect(screen.queryByText("team-a har ingen agentprofiler ennå")).not.toBeInTheDocument();
  });

  it("lists the team's agent profiles on their own page, and says so when there are none", async () => {
    const { default: AgentsPage } = await import("./agenter/page");
    render(await AgentsPage(sp("team-a")));
    expect(screen.getByText("team-a har ingen agentprofiler ennå")).toBeInTheDocument();
  });

  it("sends a person with no teams to the catalogue, which explains how to start", async () => {
    stub({ [`${API}/whoami`]: { name: "A123456", teams: [] } });
    await expect(FleetPage(sp())).rejects.toThrow("NEXT_REDIRECT /fleet/katalog");
    render(await CatalogPage());
    expect(screen.getByRole("heading", { name: "Kom i gang" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Nais Console" })).toHaveAttribute(
      "href",
      "https://console.nav.cloud.nais.io/"
    );
    expect(
      within(screen.getByRole("table", { name: "Publiserte MCP-servere" })).getByRole("link", { name: "saker" })
    ).toHaveAttribute("href", "/fleet/mcp-servere/team-b/saker");
  });

  it("lists published MCP servers from every team, with search and a data category filter", async () => {
    render(await CatalogPage());
    const table = () => screen.getByRole("table", { name: "Publiserte MCP-servere" });
    expect(table()).toHaveTextContent("team-b");
    expect(table()).toHaveTextContent("Fortrolige data");
    fireEvent.change(screen.getByRole("textbox", { name: /Søk/ }), { target: { value: "SAKER" } });
    expect(within(table()).queryByRole("link", { name: "docs" })).not.toBeInTheDocument();
    expect(within(table()).getByRole("link", { name: "saker" })).toBeInTheDocument();
    // Katalog: the search also finds a tool name.
    fireEvent.change(screen.getByRole("textbox", { name: /Søk/ }), { target: { value: "hent_sak" } });
    expect(within(table()).getByRole("link", { name: "saker" })).toBeInTheDocument();
    fireEvent.change(screen.getByRole("textbox", { name: /Søk/ }), { target: { value: "" } });
    fireEvent.change(screen.getByRole("combobox", { name: "Datakategori" }), { target: { value: "Intern" } });
    expect(within(table()).getByRole("link", { name: "docs" })).toBeInTheDocument();
    expect(within(table()).queryByRole("link", { name: "saker" })).not.toBeInTheDocument();
    expect(screen.getByText("Viser 1 av 2")).toBeInTheDocument();
  });

  const threeTeams = {
    [`${API}/whoami`]: { name: "A123456", teams: ["team-a", "team-d"] },
    [`${API}/namespaces/team-d/mcpservers`]: { items: [] },
    [`${API}/namespaces/team-d/agentprofiles`]: { items: [profile()] },
  };

  it("asks a person in several teams to choose, then remembers the team", async () => {
    stub(threeTeams);
    const { unmount } = render(await FleetPage(sp()));
    expect(screen.getByRole("heading", { name: "Velg team" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "team-d" })).toHaveAttribute("href", "/fleet?team=team-d");
    expect(screen.getByText("1 MCP-server · 1 venter på godkjenning")).toBeInTheDocument();
    unmount();
    render(await FleetPage(sp("team-d")));
    expect(document.cookie).toContain("fleet-team=team-d");
    expect(screen.getByRole("combobox", { name: "Team" })).toHaveValue("team-d");
    cookieJar.set("fleet-team", "team-d");
    render(await FleetPage(sp()));
    expect(screen.queryByRole("heading", { name: "Velg team" })).not.toBeInTheDocument();
  });

  it("ignores a remembered team the person has left, and ?team= empty shows the chooser", async () => {
    stub(threeTeams);
    cookieJar.set("fleet-team", "team-x");
    const { unmount } = render(await FleetPage(sp()));
    expect(screen.getByRole("heading", { name: "Velg team" })).toBeInTheDocument();
    expect(screen.queryByText(/ikke med i/)).not.toBeInTheDocument();
    unmount();
    cookieJar.set("fleet-team", "team-d");
    render(await FleetPage(sp("")));
    expect(screen.getByRole("heading", { name: "Velg team" })).toBeInTheDocument();
    // VelgTeam: the remembered team is marked, and the way to the catalogue is there.
    expect(within(screen.getByRole("list", { name: "Teamene dine" })).getByText("Sist brukt")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "katalogen og Kom i gang" })).toHaveAttribute("href", "/fleet/katalog");
  });

  it("explains an unknown team and offers the chooser; ignores an invalid one", async () => {
    stub(threeTeams);
    const { unmount } = render(await FleetPage(sp("team-x")));
    expect(screen.getByText(/Du er ikke med i team-x/)).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Velg team" })).toBeInTheDocument();
    unmount();
    render(await FleetPage(sp("Not a label!")));
    expect(screen.getByText(/Du er ikke med i det teamet\./)).toBeInTheDocument();
    expect(screen.queryByText(/Not a label/)).not.toBeInTheDocument();
  });

  it("shows an Alert on 403 and 409, and a retry on 503", async () => {
    stub({}, { [`${API}/whoami`]: 403 });
    render(await FleetPage(sp()));
    expect(screen.getByText(/ikke tilgang/)).toBeInTheDocument();
    stub({}, { [`${API}/whoami`]: 409 });
    render(await FleetPage(sp()));
    expect(screen.getByText("Noen andre har endret dette. Last inn siden på nytt.")).toBeInTheDocument();
    stub({}, { [`${API}/whoami`]: 503 });
    render(await FleetPage(sp()));
    expect(screen.getByRole("button", { name: "Prøv igjen" })).toBeInTheDocument();
  });

  it("sends 401 to login", async () => {
    stub({}, { [`${API}/whoami`]: 401 });
    await expect(FleetPage(sp())).rejects.toThrow("NEXT_REDIRECT /oauth2/login");
  });

  it("shows an MCP server with its route, who acts next and its history", async () => {
    render(await ServerPage(params("team-a", "docs")));
    expect(screen.getByText(/må godkjenne versjon 3/)).toBeInTheDocument();
    expect(screen.getAllByText("/mcp/team-a/docs").length).toBeGreaterThan(0);
    // Server: the curl test for the published route, and Nødtiltak for revocation.
    expect(screen.getByRole("heading", { name: "Test MCP-endpointet" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Nødtiltak" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Versjon 3 venter" })).toBeInTheDocument();
    expect(screen.getByText("Historikk")).toBeInTheDocument();
  });

  it("renders the untrusted readme without raw HTML, unsafe links or images, and shows tool descriptions", async () => {
    const s = server();
    Object.assign(s.status.pending.spec, {
      readme:
        '# Om docs\n\n<script>alert(1)</script><b id="raw">rå</b>\n\n[ond](javascript:alert(1)) [data](data:text/html,x) [blandet](JaVaScRiPt:alert(1)) [god](https://example.com) ![bilde](https://evil.example/x.png)',
      tools: [{ name: "search", description: "Søker i dokumentasjonen" }],
    });
    stub({ [`${API}/namespaces/team-a/mcpservers/docs`]: s });
    const { container } = render(await ServerPage(params("team-a", "docs")));
    // The readme sits in a collapsed «Om MCP-serveren», after Nødtiltak.
    fireEvent.click(within(container.querySelector('[aria-label="Om MCP-serveren"]')!).getByRole("button"));
    expect(screen.getByRole("heading", { name: "Om docs", level: 3 })).toBeInTheDocument();
    expect(container.querySelector("script")).toBeNull();
    expect(container.querySelector("#raw")).toBeNull();
    expect(container.querySelector("img")).toBeNull();
    for (const t of ["ond", "data", "blandet"]) expect(screen.getByText(t).closest("a")).toBeNull();
    expect(screen.getByRole("link", { name: "god" })).toHaveAttribute("rel", "noopener noreferrer");
    // Once as stacked rows below md, once in the table from md up.
    expect(screen.getAllByText("Søker i dokumentasjonen")).toHaveLength(2);
  });

  it("tells the owner team to resubmit a draft no person sent in", async () => {
    const p = profile();
    Object.assign(p.status, { needsResubmit: true });
    stub({ [`${API}/namespaces/aksel/agentprofiles/studio`]: p });
    render(await AgentPage(params("aksel", "studio")));
    expect(screen.getByText(/må sende det inn på nytt/)).toBeInTheDocument();
  });

  it("the overview names «Godkjenn» as the next step only for the platform team", async () => {
    render(await FleetPage(sp()));
    expect(screen.queryAllByText("Godkjenn versjon 3")).toHaveLength(0);
    expect(screen.getAllByText("Venter på team copilot").length).toBeGreaterThan(0);
    cleanup();
    stub({
      [`${API}/whoami`]: { name: "A123456", teams: ["team-a", "copilot"] },
      [`${API}/namespaces/copilot/mcpservers`]: { items: [] },
      [`${API}/namespaces/copilot/agentprofiles`]: { items: [] },
    });
    render(await FleetPage(sp("team-a")));
    expect(screen.getAllByText("Godkjenn versjon 3").length).toBeGreaterThan(0);
  });

  it("shows «Godkjenn» to the platform team only, whoever submitted", async () => {
    // Owner team, not in copilot: no button, the rule instead.
    render(await ServerPage(params("team-a", "docs")));
    expect(screen.queryByRole("button", { name: "Godkjenn versjon 3" })).toBeNull();
    expect(screen.getByText(/^Versjon 3 venter på team copilot\.$/)).toBeInTheDocument();
    cleanup();
    // In copilot and the submitter: self-approval is allowed.
    const own = server();
    Object.assign(own.status.pending, { submittedBy: { kind: "User", name: "A123456" } });
    stub({
      [`${API}/namespaces/team-a/mcpservers/docs`]: own,
      [`${API}/whoami`]: { name: "A123456", teams: ["team-a", "copilot"] },
    });
    render(await ServerPage(params("team-a", "docs")));
    expect(screen.getByRole("button", { name: "Godkjenn versjon 3" })).toBeInTheDocument();
  });

  it("offers approval with the pending version, and revocation only while not revoked", async () => {
    // A copilot member on another team's server (#3095 lists these).
    stub({ [`${API}/whoami`]: { name: "B654321", teams: ["copilot"] } });
    render(await ServerPage(params("team-a", "docs")));
    expect(screen.getByRole("button", { name: "Godkjenn versjon 3" })).toBeInTheDocument();
    expect(document.querySelector('input[name="contentHash"]')).toHaveValue("h3");
    expect(screen.getByRole("button", { name: "Trekk tilbake" })).toBeInTheDocument();
    expect(screen.getByRole("table", { name: /versjonen som venter/ })).toBeInTheDocument();
    stub({ [`${API}/namespaces/team-a/mcpservers/docs`]: server(true) });
    const { container } = render(await ServerPage(params("team-a", "docs")));
    expect(container.querySelector('textarea[name="reason"]')).toBeNull();
  });

  it("hides approval from the person who submitted the pending version", async () => {
    const own = server();
    Object.assign(own.status.pending, { submittedBy: { kind: "User", name: "A123456" } });
    stub({ [`${API}/namespaces/team-a/mcpservers/docs`]: own });
    render(await ServerPage(params("team-a", "docs")));
    expect(screen.queryByRole("button", { name: "Godkjenn versjon 3" })).toBeNull();
    expect(
      screen.getByText(
        "Versjon 3 venter på team copilot. Vil du endre noe før det, bruk «Endre MCP-serveren» nederst på siden. Det blir en ny versjon som erstatter denne."
      )
    ).toBeInTheDocument();
  });

  it("gives a next step when the team has no access", async () => {
    stub({}, { [`${API}/namespaces/team-a/mcpservers/docs`]: 403 });
    render(await ServerPage(params("team-a", "docs")));
    expect(screen.getByRole("link", { name: "Nais Console" })).toBeInTheDocument();
  });

  it("shows a never-published draft as not published, with registry changes, and no revocation", async () => {
    const draft = server();
    draft.status.conditions = [cond("Published", false), cond("Revoked", false)];
    Object.assign(draft.status.pending.spec, {
      registry: { version: "1.2.0", remotes: [{ type: "streamable-http", url: "https://x" }] },
    });
    stub({ [`${API}/namespaces/team-a/mcpservers/docs`]: draft });
    render(await ServerPage(params("team-a", "docs")));
    expect(screen.getByText(/^Ikke publisert ennå: alt under er nytt/)).toBeInTheDocument();
    // Table on a wide screen, changed fields as a list on a phone.
    expect(screen.getAllByText("1.2.0")).toHaveLength(2);
    expect(screen.getAllByText("streamable-http https://x")).toHaveLength(2);
    expect(screen.queryByRole("button", { name: "Trekk tilbake" })).toBeNull();
  });

  it("shows the registration wizard with the person's teams", async () => {
    const { default: RegisterPage } = await import("./registrer/page");
    const sp = (team?: string) => ({ searchParams: Promise.resolve({ team }) });
    render(await RegisterPage(sp()));
    expect(screen.getByRole("option", { name: "team-a" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Navn" })).toBeInTheDocument();
    // Registrer: the lead names the team and says nothing is published before approval.
    expect(screen.getByText(/^For team-a\. Tar rundt fem minutter\./)).toBeInTheDocument();
    vi.stubEnv("FLEET_CONTROL_URL", "");
    await expect(RegisterPage(sp())).rejects.toThrow("NEXT_NOT_FOUND");
  });

  it("shows that a revoked MCP server with a new draft waits for approval", async () => {
    stub({ [`${API}/namespaces/team-a/mcpservers/docs`]: server(true) });
    render(await ServerPage(params("team-a", "docs")));
    expect(screen.getByText("Trukket tilbake, ny versjon venter")).toBeInTheDocument();
    expect(screen.getByText(/må godkjenne versjon 3 før MCP-serveren blir publisert igjen/)).toBeInTheDocument();
  });

  it("is 404 for an MCP server that is not in the catalogue", async () => {
    await expect(ServerPage(params("team-a", "nope"))).rejects.toThrow("NEXT_NOT_FOUND");
    await expect(ServerPage(params("other", "docs"))).rejects.toThrow("NEXT_NOT_FOUND");
  });

  it("shows another team's published MCP server read-only, without history or actions", async () => {
    render(await ServerPage(params("team-b", "saker")));
    expect(screen.getByText("Fortrolige data")).toBeInTheDocument();
    expect(screen.getByText("/mcp/team-b/saker")).toBeInTheDocument();
    expect(screen.getByText("hent_sak")).toBeInTheDocument();
    expect(screen.getByText(/Henter én sak\./)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Godkjenn|Trekk tilbake|Endre/ })).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Be eierteamet om tilgang" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Nais Console" })).toHaveAttribute(
      "href",
      "https://console.nav.cloud.nais.io/team/team-b"
    );
    expect(screen.getByRole("heading", { name: "Konsumenter" })).toBeInTheDocument();
    // ServerRead: the read-only notice says who decides, and tools are listed with descriptions.
    expect(screen.getByText(/Utkast, historikk og handlinger ser bare eierteamet/)).toBeInTheDocument();
    const consumers = screen.getByRole("table", { name: "Agentprofiler med tilgang" });
    expect(consumers).toHaveTextContent("team-a");
    expect(consumers).toHaveTextContent("studio");
    expect(screen.getAllByText(/Teamet ditt har tilgang gjennom studio\./).length).toBeGreaterThan(0);
    const urls = vi.mocked(fetch).mock.calls.map((c) => String(c[0]));
    expect(urls).toContain(`http://fc${API}/namespaces/team-b/mcpservercatalogentries/saker`);
  });

  const catalogFetched = () =>
    vi.mocked(fetch).mock.calls.some((c) => String(c[0]).includes("mcpservercatalogentries"));

  it("does not fall back to the catalogue on 503", async () => {
    stub({}, { [`${API}/namespaces/team-b/mcpservers/saker`]: 503 });
    render(await ServerPage(params("team-b", "saker")));
    expect(catalogFetched()).toBe(false);
    expect(screen.getByText(/Vi får ikke kontakt med Fleet/)).toBeInTheDocument();
  });

  it("sends 401 on the server to login without fetching the catalogue", async () => {
    stub({}, { [`${API}/namespaces/team-b/mcpservers/saker`]: 401 });
    await expect(ServerPage(params("team-b", "saker"))).rejects.toThrow("NEXT_REDIRECT /oauth2/login");
    expect(catalogFetched()).toBe(false);
  });

  it("does not switch to the read-only view when only history is 404", async () => {
    stub({}, { [`${API}/namespaces/team-a/mcpservers/docs/history`]: 404 });
    await expect(ServerPage(params("team-a", "docs"))).rejects.toThrow("NEXT_NOT_FOUND");
    expect(catalogFetched()).toBe(false);
  });

  it("shows an agent profile to an MCP server's owner team without the history", async () => {
    render(await AgentPage(params("aksel", "studio")));
    expect(screen.getByText(/Venter på: MCP-server team-a\/docs\.$/)).toBeInTheDocument();
    expect(screen.getByText("ikke gitt tilgang")).toBeInTheDocument();
    expect(screen.queryByText("Historikk")).not.toBeInTheDocument();
    // Agent: status in #2920's words and one table row per MCP server.
    expect(screen.getByText("Venter på eierteamene")).toBeInTheDocument();
    expect(screen.getByRole("table", { name: "Tilganger per MCP-server" })).toHaveTextContent("team-a");
  });

  it("shows who granted access, and leaves it out when the excerpt hides it", async () => {
    stub({
      [`${API}/namespaces/aksel/agentprofiles/studio`]: profile({
        granted: true,
        grantedBy: { name: "B1" },
        grantedAt: T,
      }),
    });
    render(await AgentPage(params("aksel", "studio")));
    expect(screen.getByText(/^tilgang gitt av B1 /)).toBeInTheDocument();
    expect(screen.queryByText(/Venter på:/)).not.toBeInTheDocument();
    stub({ [`${API}/namespaces/aksel/agentprofiles/studio`]: profile({ granted: true, grantedAt: T }) });
    render(await AgentPage(params("aksel", "studio")));
    expect(screen.getByText(/^tilgang gitt \d/)).toBeInTheDocument();
  });

  it("reads an unknown condition or reason as not true", async () => {
    const odd = server();
    odd.status.conditions = [cond("Published", true, "SomethingNew"), { ...cond("Revoked", true), status: "Unknown" }];
    stub({ [`${API}/namespaces/team-a/mcpservers/docs`]: odd });
    render(await ServerPage(params("team-a", "docs")));
    expect(screen.getByText("Ny versjon venter")).toBeInTheDocument();
  });

  it("sends the old /fleet/ruter page permanently to /fleet/mcp-servere", async () => {
    await expect(OldRoutePage(params("team-a", "docs"))).rejects.toThrow(
      "NEXT_PERMANENT_REDIRECT /fleet/mcp-servere/team-a/docs"
    );
  });

  it.each(["..", "%2e%2e", "a/b", "Team"])("is 404 for the name %s and never fetches or redirects", async (bad) => {
    await expect(ServerPage(params(bad, "docs"))).rejects.toThrow("NEXT_NOT_FOUND");
    await expect(ServerPage(params("team-a", bad))).rejects.toThrow("NEXT_NOT_FOUND");
    await expect(AgentPage(params(bad, "studio"))).rejects.toThrow("NEXT_NOT_FOUND");
    await expect(OldRoutePage(params("team-a", bad))).rejects.toThrow("NEXT_NOT_FOUND");
    expect(fetch).not.toHaveBeenCalled();
  });

  it("is 404 on the old /fleet/ruter page without FLEET_CONTROL_URL", async () => {
    vi.stubEnv("FLEET_CONTROL_URL", "");
    await expect(OldRoutePage(params("team-a", "docs"))).rejects.toThrow("NEXT_NOT_FOUND");
  });

  it("shows a revoked grant as revoked, not as waiting, and formats quotas", async () => {
    stub({
      [`${API}/namespaces/aksel/agentprofiles/studio`]: profile({
        granted: false,
        grantedAt: T,
        reason: "AccessRevoked",
      }),
    });
    render(await AgentPage(params("aksel", "studio")));
    expect(screen.getByText("tilgang trukket tilbake")).toBeInTheDocument();
    expect(screen.queryByText(/Venter på:/)).not.toBeInTheDocument();
    expect(screen.getByText(/1\s000 kall i døgnet/)).toBeInTheDocument();
  });

  const waiting = () => profile({ granted: false, contentHash: "g1" });

  it("offers «gi tilgang» in the inbox and on the MCP server page, with the grant's hash", async () => {
    stub({ [`${API}/agentprofiles?awaitingApprovalBy=me`]: { items: [waiting()] } });
    const { default: AgentsPage } = await import("./agenter/page");
    const { unmount } = render(await AgentsPage(sp("team-a")));
    expect(screen.getAllByRole("button", { name: "Gi tilgang til docs" }).at(-1)!).toBeInTheDocument();
    unmount();
    render(await ServerPage(params("team-a", "docs")));
    const table = screen.getByRole("table", { name: "Agentprofiler som venter på tilgang" });
    expect(within(table).getByRole("link", { name: "aksel/studio" })).toBeInTheDocument();
    expect(within(table).getByRole("button", { name: "Gi tilgang" })).toBeInTheDocument();
    expect(table.querySelector('input[name="contentHash"]')).toHaveValue("g1");
    expect(table.querySelector('input[name="generation"]')).toHaveValue("1");
  });

  it("offers no «gi tilgang» without a pending version or hash", async () => {
    render(await FleetPage(sp()));
    expect(screen.queryByRole("button", { name: /Gi tilgang/ })).toBeNull();
  });

  it("asks in a Modal before revoking", async () => {
    render(await ServerPage(params("team-a", "docs")));
    const before = vi.mocked(fetch).mock.calls.length;
    fireEvent.click(screen.getByRole("button", { name: "Trekk tilbake" }));
    const dialog = screen.getByRole("dialog", { name: "Trekk tilbake docs?" });
    expect(vi.mocked(fetch).mock.calls.length).toBe(before);
    // The reason is asked for in the Modal and still belongs to the form.
    const reason = within(dialog).getByRole("textbox", { name: /Begrunnelse/ });
    expect(document.getElementById(reason.getAttribute("form")!)?.tagName).toBe("FORM");
    fireEvent.change(reason, { target: { value: "lekker data" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Trekk tilbake" }));
    await vi.waitFor(() => expect(vi.mocked(fetch).mock.calls.length).toBe(before + 1));
    const sent = JSON.parse(String((vi.mocked(fetch).mock.calls.at(-1)![1] as RequestInit).body));
    expect(sent).toMatchObject({ action: "revoke", name: "docs", reason: "lekker data" });
  });

  it("tells a revoked MCP server how to come back, not «Ingen»", async () => {
    const revoked = server(true);
    delete (revoked.status as { pending?: unknown }).pending;
    stub({ [`${API}/namespaces/team-a/mcpservers/docs`]: revoked });
    render(await ServerPage(params("team-a", "docs")));
    expect(screen.getByText(/Endre MCP-serveren og send inn en ny versjon/)).toBeInTheDocument();
    expect(screen.queryByText(/Ingen\. MCP-serveren/)).toBeNull();
    expect(screen.getByRole("heading", { name: "Endre MCP-serveren" })).toBeInTheDocument();
  });

  it("posts «gi tilgang» with the profile, the MCP server and the grant's hash", async () => {
    stub({ [`${API}/agentprofiles?awaitingApprovalBy=me`]: { items: [waiting()] } });
    const { default: AgentsPage } = await import("./agenter/page");
    render(await AgentsPage(sp("team-a")));
    const post = vi.fn(async () => new Response(JSON.stringify({ error: "Nei" }), { status: 403 }));
    vi.stubGlobal("fetch", post);
    fireEvent.click(screen.getAllByRole("button", { name: "Gi tilgang til docs" }).at(-1)!);
    expect(await screen.findByText("Nei")).toBeInTheDocument();
    const [url, init] = post.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("/api/fleet");
    expect(JSON.parse(String(init.body))).toEqual({
      action: "grant",
      namespace: "aksel",
      name: "studio",
      serverNamespace: "team-a",
      serverName: "docs",
      generation: "1",
      contentHash: "g1",
    });
  });

  it("warns when the waiting agent profiles cannot be fetched, instead of hiding them", async () => {
    stub({}, { [`${API}/agentprofiles?awaitingApprovalBy=me`]: 503 });
    render(await ServerPage(params("team-a", "docs")));
    expect(screen.getByText(/Vi fikk ikke hentet agentprofilene/)).toBeInTheDocument();
  });

  it("shows the edit form and no rollback only to the owner team", async () => {
    render(await ServerPage(params("team-a", "docs")));
    expect(screen.getByRole("heading", { name: "Endre MCP-serveren" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Rull tilbake/ })).toBeNull();
    expect(screen.getByText(/ruller dere tilbake appen i Nais/)).toBeInTheDocument();
    stub({ [`${API}/whoami`]: { name: "A123456", teams: ["plattform"] } });
    const { container } = render(await ServerPage(params("team-a", "docs")));
    expect(container.querySelector('[aria-label="Endre MCP-serveren"]')).toBeNull();
  });

  it("states: 503 says the MCP servers still work, an empty team gets next steps, loading has a skeleton", async () => {
    stub({}, { [`${API}/whoami`]: 503 });
    render(await FleetPage(sp()));
    expect(screen.getByRole("heading", { name: "Vi får ikke kontakt med Fleet akkurat nå" })).toBeInTheDocument();
    expect(screen.getByText(/MCP-serverne virker som før/)).toBeInTheDocument();
    stub({ [`${API}/namespaces/team-a/mcpservers`]: { items: [] } });
    const { container: container0 } = render(await FleetPage(sp()));
    expect(screen.getByRole("heading", { name: "team-a har ingen MCP-servere ennå" })).toBeInTheDocument();
    expect(container0.querySelector('a[href="/fleet/registrer?team=team-a"]')).not.toBeNull();
    const { default: Loading } = await import("./loading");
    const { container } = render(<Loading />);
    expect(container.querySelector("[aria-busy=true]")).not.toBeNull();
  });

  it("states: after sending in, the MCP server page shows the receipt", async () => {
    render(await ServerPage({ ...params("team-a", "docs"), searchParams: Promise.resolve({ sendt: "1" }) }));
    expect(screen.getByRole("heading", { name: "docs er sendt til godkjenning" })).toBeInTheDocument();
  });

  it("Hva endres marks added and removed tools, and unchanged fields once", async () => {
    const s = server();
    s.spec = { ...s.spec, allowedTools: ["search", "old"] };
    s.status.pending.spec = { ...s.status.pending.spec, ...s.spec, allowedTools: ["search", "new"] };
    stub({ [`${API}/namespaces/team-a/mcpservers/docs`]: s });
    render(await ServerPage(params("team-a", "docs")));
    const table = screen.getByRole("table", { name: /versjonen som venter/ });
    expect(within(table).getByText("+ new")).toBeInTheDocument();
    expect(within(table).getByText("− old")).toBeInTheDocument();
    expect(within(table).getAllByText("Uendret").length).toBeGreaterThan(0);
  });

  it("marks Katalog, not MCP-servere, on another team's server, and keeps the approval count on every page", async () => {
    nav.path = "/fleet/mcp-servere/team-b/saker";
    const { container } = render(await ServerPage(params("team-b", "saker")));
    // Desktop menu and phone link list each mark exactly one item, and it is Katalog.
    for (const list of container.querySelectorAll('nav[aria-label="Fleet"] ul')) {
      const current = [...list.querySelectorAll('[aria-current="page"]')].map((e) => e.textContent?.trim());
      if (current.length) expect(current).toEqual(["Katalog"]);
    }
    expect(container.querySelectorAll('nav[aria-label="Fleet"] [aria-current="page"]')).toHaveLength(2);
    expect(container.querySelector('nav[aria-label="Fleet"] [aria-label$="venter"]')).not.toBeNull();
  });

  it("orders the owner's MCP server page as in the sketch", async () => {
    const { container } = render(await ServerPage(params("team-a", "docs")));
    const order = [...container.querySelectorAll("h2")].map((h) => h.textContent?.trim());
    const at = (t: string) => order.indexOf(t);
    for (const t of ["Hva endres", "Test MCP-endpointet", "Nødtiltak", "Detaljer", "Historikk", "Endre MCP-serveren"])
      expect(at(t), t).toBeGreaterThanOrEqual(0);
    expect(at("Hva endres")).toBeLessThan(at("Test MCP-endpointet"));
    expect(at("Test MCP-endpointet")).toBeLessThan(at("Nødtiltak"));
    expect(at("Nødtiltak")).toBeLessThan(at("Detaljer"));
    expect(at("Historikk")).toBeLessThan(at("Endre MCP-serveren"));
  });

  it("renders without the badge when the badge call is slow, after the short timeout", async () => {
    const { BADGE_TIMEOUT_MS } = await import("./frame");
    const fast = vi.mocked(fetch).getMockImplementation()!;
    vi.stubGlobal(
      "fetch",
      vi.fn((url: string, init?: RequestInit) =>
        url.endsWith("/namespaces/team-a/mcpservers") ? new Promise<Response>(() => {}) : fast(url, init)
      )
    );
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const page = ServerPage(params("team-b", "saker"));
      await vi.advanceTimersByTimeAsync(BADGE_TIMEOUT_MS + 10);
      const { container } = render(await page);
      expect(container.querySelector('nav[aria-label="Fleet"] [aria-label$="venter"]')).toBeNull();
      expect(screen.getByRole("heading", { name: "Konsumenter" })).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("ignores a malformed team cookie for the badge, and still renders", async () => {
    cookieJar.set("fleet-team", "../Ikke gyldig/%2F");
    const { fleetApi } = await import("@/lib/fleet-control");
    const servers = vi.spyOn(fleetApi, "servers");
    const { container } = render(await ServerPage(params("team-b", "saker")));
    expect(servers.mock.calls.map((c) => c[0])).not.toContain("../Ikke gyldig/%2F");
    servers.mockRestore();
    expect(screen.getByRole("heading", { name: "Konsumenter" })).toBeInTheDocument();
    const urls = vi.mocked(fetch).mock.calls.map((c) => String(c[0]));
    expect(urls.some((u) => u.includes("Ikke") || u.includes(".."))).toBe(false);
    // The nav falls back to the person's own team.
    expect(container.querySelector('nav[aria-label="Fleet"]')).toHaveTextContent("team-a");
  });

  it("starts the badge request before the page's own fetches finish", async () => {
    cookieJar.set("fleet-team", "team-a");
    render(await ServerPage(params("team-a", "docs")));
    const urls = vi.mocked(fetch).mock.calls.map((c) => String(c[0]).replace("http://fc", ""));
    const badge = urls.indexOf(`${API}/namespaces/team-a/mcpservers`);
    expect(badge).toBeGreaterThanOrEqual(0);
    // whoami runs only after the MCP server itself has loaded; the badge request is already out.
    expect(badge).toBeLessThan(urls.indexOf(`${API}/whoami`));
    // Cookie team and page team are the same: one request, not two.
    expect(urls.filter((u) => u === `${API}/namespaces/team-a/mcpservers`)).toHaveLength(1);
  });

  it("clears the badge timeout once the badge call has settled", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      render(await ServerPage(params("team-b", "saker")));
      // Every request answered at once; no 2 s timer may be left behind.
      expect(vi.getTimerCount()).toBe(0);
    } finally {
      vi.useRealTimers();
    }
  });
});

describe("/fleet/godkjenning", () => {
  const page = async (team?: string) => render(await (await import("./godkjenning/page")).default(sp(team)));
  beforeEach(() => {
    nav.path = "/fleet/godkjenning";
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("FLEET_CONTROL_URL", "http://fc");
    cookieJar.clear();
    stub();
  });

  it("lists what waits on the team: the pending MCP server and the waiting agent profiles", async () => {
    await page("team-a");
    expect(screen.getByRole("heading", { name: "Til godkjenning", level: 1 })).toBeInTheDocument();
    expect(screen.getByText(/docs/)).toBeInTheDocument();
    expect(screen.getByText(/1 agentprofil venter/)).toBeInTheDocument();
    expect(screen.queryByText(/Ingenting venter/)).toBeNull();
  });

  it("says «Ingenting venter» when nothing waits", async () => {
    const quiet = server();
    delete (quiet.status as { pending?: unknown }).pending;
    stub({
      [`${API}/namespaces/team-a/mcpservers`]: { items: [quiet] },
      [`${API}/agentprofiles?awaitingApprovalBy=me`]: { items: [] },
    });
    await page("team-a");
    expect(screen.getByText("Ingenting venter på team-a nå.")).toBeInTheDocument();
  });

  it("tells a person with no team to join one in Nais", async () => {
    stub({ [`${API}/whoami`]: { name: "A123456", teams: [] } });
    await page();
    expect(screen.getByText("Du må være med i et team i Nais for å godkjenne endringer.")).toBeInTheDocument();
    expect(screen.queryByText(/Ingenting venter/)).toBeNull();
  });

  it("shows a problem alert when fleet-control fails", async () => {
    stub({}, { [`${API}/whoami`]: 503 });
    await page("team-a");
    expect(screen.getByRole("button", { name: "Prøv igjen" })).toBeInTheDocument();
    expect(screen.queryByText(/Ingenting venter/)).toBeNull();
  });

  it("marks «Til godkjenning» as current in the side menu, and only it", async () => {
    const { container } = await page("team-a");
    const current = [...container.querySelectorAll('nav[aria-label="Fleet"] [aria-current="page"]')];
    expect(current.length).toBeGreaterThan(0);
    for (const e of current) expect(e.textContent).toContain("Til godkjenning");
  });
});

describe("ServerInbox avatars", () => {
  const by = (who: string) => {
    const s = server() as unknown as MCPServer;
    s.status.pending!.submittedBy = { name: who };
    return s;
  };
  it("shows up to three initials from the signed-in person's name, never the NAVident letter", () => {
    const { container } = render(
      <ServerInbox servers={[by("Z990001")]} me="Z990001" myName="Hans Kristian Flaatten" />
    );
    expect(screen.getByText("Sendt inn av Hans Kristian Flaatten (deg)")).toBeInTheDocument();
    expect(container.textContent).toContain("HKF");
  });
  it("shows a person icon for someone else, since fleet-control only has the NAVident", () => {
    const { container } = render(
      <ServerInbox servers={[by("F123456")]} me="Z990001" myName="Hans Kristian Flaatten" />
    );
    expect(screen.getByText("Sendt inn av F123456")).toBeInTheDocument();
    const avatar = container.querySelector("[aria-hidden]")!;
    expect(avatar.querySelector("svg")).not.toBeNull();
    expect(avatar.textContent).toBe("");
  });
  const initials = (myName?: string) =>
    render(<ServerInbox servers={[by("Z990001")]} me="Z990001" myName={myName} />).container.querySelector(
      "[aria-hidden]"
    )!;
  it("shows one initial for a one-word name", () => {
    expect(initials("Madonna").textContent).toBe("M");
  });
  it("upper-cases æøå initials", () => {
    expect(initials("åse østby ærlig").textContent).toBe("ÅØÆ");
  });
  it("shows a person icon, not an empty circle, when the name is empty or missing", () => {
    for (const name of ["", "   ", undefined]) {
      const avatar = initials(name);
      expect(avatar.textContent).toBe("");
      expect(avatar.querySelector("svg")).not.toBeNull();
      cleanup();
    }
  });
  it("writes «deg» alone without a name, and «Navn (deg)» on the server page with one", async () => {
    const own = server();
    Object.assign(own.status.pending, { submittedBy: { kind: "User", name: "A123456" } });
    const { ServerDetail } = await import("./fleet-ui");
    const detail = (myName?: string) =>
      render(<ServerDetail server={own as unknown as MCPServer} events={[]} me="A123456" myName={myName} owner />);
    detail("Kari Nordmann");
    expect(screen.getByText("Kari Nordmann (deg)")).toBeInTheDocument();
    cleanup();
    detail(undefined);
    expect(screen.getByText("deg")).toBeInTheDocument();
    expect(screen.queryByText(/\(deg\)/)).toBeNull();
  });
});

describe("ServerForm discovery (edit)", () => {
  const server = {
    metadata: { namespace: "team-a", name: "docs" },
    spec: { upstream: { app: "docs-mcp", url: "http://docs-mcp.team-a/mcp" }, allowedTools: ["old"] },
    status: {},
  } as unknown as MCPServer;
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("hides «Hent verktøy» without the probe", () => {
    render(<ServerForm server={server} />);
    expect(screen.queryByRole("button", { name: "Hent verktøy" })).toBeNull();
  });

  it("fills the field with discovered tools, leaves destructive ones out and shows the hints", async () => {
    const fetch = vi.fn(async () =>
      Response.json({
        tools: [
          { name: "hent", suggestedRisk: "read", destructive: false },
          { name: "slett", suggestedRisk: "write", destructive: true },
        ],
      })
    );
    vi.stubGlobal("fetch", fetch);
    render(<ServerForm server={server} discoveryEnabled />);
    fireEvent.click(screen.getByRole("button", { name: "Hent verktøy" }));
    expect(await screen.findByText(/Fant 2 verktøy/)).toBeInTheDocument();
    expect(screen.getByLabelText("Verktøy")).toHaveValue("hent");
    expect(screen.getByText(/Kan endre data, ikke lagt inn/)).toBeInTheDocument();
    expect(JSON.parse((fetch.mock.calls[0] as unknown as [string, RequestInit])[1].body as string)).toEqual({
      namespace: "team-a",
      app: "docs-mcp",
      path: "/mcp",
    });
  });

  it("shows the accessPolicy snippet on access", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => Response.json({ kind: "access", error: "Legg fleet-gateway til." }, { status: 502 }))
    );
    render(<ServerForm server={server} discoveryEnabled />);
    fireEvent.click(screen.getByRole("button", { name: "Hent verktøy" }));
    expect(await screen.findByText(/Legg fleet-gateway til/)).toBeInTheDocument();
    expect(screen.getByText(/application: fleet-gateway/)).toBeInTheDocument();
  });
});
