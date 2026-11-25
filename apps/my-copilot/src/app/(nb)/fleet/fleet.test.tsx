import { fireEvent, render, screen, within } from "@testing-library/react";
import { vi } from "vitest";

const cookieJar = new Map<string, string>();
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
  usePathname: () => "/fleet",
  useRouter: () => ({ refresh: vi.fn(), replace: vi.fn(), push: vi.fn() }),
}));

import FleetPage from "./page";
import ServerPage from "./mcp-servere/[namespace]/[name]/page";
import AgentPage from "./agenter/[namespace]/[name]/page";
import OldRoutePage from "./ruter/[namespace]/[name]/page";
import CatalogPage from "./katalog/page";

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
    expect(screen.getByText("Kort fortalt")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Til godkjenning" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "studio" })).toHaveAttribute("href", "/fleet/agenter/aksel/studio");
    expect(screen.getByText("MCP-server team-a/docs")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "team-a" })).toBeInTheDocument();
    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: /Registrer/ })).toHaveLength(1);
    expect(screen.getByRole("link", { name: "Registrer MCP-server" })).toHaveAttribute("href", "/fleet/registrer");
    expect(screen.getByRole("link", { name: "Registrer MCP-server" })).toHaveClass("aksel-link");
    expect(screen.getByRole("navigation", { name: "Fleet" })).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "docs" })[0]).toHaveAttribute("href", "/fleet/mcp-servere/team-a/docs");
    expect(screen.getByText("Ny versjon venter")).toBeInTheDocument();
    // team-a has an MCP server and no agent profiles: no empty section.
    expect(screen.queryByText("team-a har ingen agentprofiler ennå.")).not.toBeInTheDocument();
  });

  it("still says so when a team has nothing at all", async () => {
    stub({ [`${API}/namespaces/team-a/mcpservers`]: { items: [] } });
    render(await FleetPage(sp()));
    expect(screen.getByText("team-a har ingen agentprofiler ennå.")).toBeInTheDocument();
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
    expect(screen.getByRole("link", { name: "saker" })).toHaveAttribute("href", "/fleet/mcp-servere/team-b/saker");
  });

  it("lists published MCP servers from every team, with search and a data category filter", async () => {
    render(await CatalogPage());
    const table = () => screen.getByRole("table", { name: "Publiserte MCP-servere" });
    expect(table()).toHaveTextContent("team-b");
    expect(table()).toHaveTextContent("Fortrolige data");
    fireEvent.change(screen.getByRole("textbox", { name: /Søk/ }), { target: { value: "SAK" } });
    expect(screen.queryByRole("link", { name: "docs" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "saker" })).toBeInTheDocument();
    fireEvent.change(screen.getByRole("textbox", { name: /Søk/ }), { target: { value: "" } });
    fireEvent.change(screen.getByRole("combobox", { name: "Datakategori" }), { target: { value: "Intern" } });
    expect(screen.getByRole("link", { name: "docs" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "saker" })).not.toBeInTheDocument();
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
    expect(screen.getByText("/mcp/team-a/docs")).toBeInTheDocument();
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
    expect(screen.getByRole("heading", { name: "Om docs", level: 3 })).toBeInTheDocument();
    expect(container.querySelector("script")).toBeNull();
    expect(container.querySelector("#raw")).toBeNull();
    expect(container.querySelector("img")).toBeNull();
    for (const t of ["ond", "data", "blandet"]) expect(screen.getByText(t).closest("a")).toBeNull();
    expect(screen.getByRole("link", { name: "god" })).toHaveAttribute("rel", "noopener noreferrer");
    expect(screen.getByText("Søker i dokumentasjonen")).toBeInTheDocument();
  });

  it("tells the owner team to resubmit a draft no person sent in", async () => {
    const p = profile();
    Object.assign(p.status, { needsResubmit: true });
    stub({ [`${API}/namespaces/aksel/agentprofiles/studio`]: p });
    render(await AgentPage(params("aksel", "studio")));
    expect(screen.getByText(/må sende det inn på nytt/)).toBeInTheDocument();
  });

  it("offers approval with the pending version, and revocation only while not revoked", async () => {
    render(await ServerPage(params("team-a", "docs")));
    expect(screen.getByRole("button", { name: "Godkjenn og publiser" })).toBeInTheDocument();
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
    expect(screen.queryByRole("button", { name: "Godkjenn og publiser" })).toBeNull();
    expect(screen.getByText("Du sendte inn versjon 3 selv. En annen på team-a må godkjenne den.")).toBeInTheDocument();
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
    expect(screen.getAllByText("Ikke publisert ennå").length).toBeGreaterThan(0);
    expect(screen.getByText("1.2.0")).toBeInTheDocument();
    expect(screen.getByText("streamable-http https://x")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Trekk tilbake" })).toBeNull();
  });

  it("shows the registration wizard with the person's teams", async () => {
    const { default: RegisterPage } = await import("./registrer/page");
    const sp = (team?: string) => ({ searchParams: Promise.resolve({ team }) });
    render(await RegisterPage(sp()));
    expect(screen.getByRole("option", { name: "team-a" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Navn" })).toBeInTheDocument();
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
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Be eierteamet om tilgang" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Nais Console" })).toHaveAttribute(
      "href",
      "https://console.nav.cloud.nais.io/team/team-b"
    );
    expect(screen.getByRole("heading", { name: "Team med tilgang" })).toBeInTheDocument();
    const consumers = screen.getByRole("table", { name: "Agentprofiler med tilgang" });
    expect(consumers).toHaveTextContent("team-a");
    expect(consumers).toHaveTextContent("studio");
    expect(screen.getByText("Teamet ditt har tilgang gjennom studio.")).toBeInTheDocument();
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
    expect(screen.getByText("search (ikke gitt tilgang)")).toBeInTheDocument();
    expect(screen.queryByText("Historikk")).not.toBeInTheDocument();
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
    expect(screen.getByText(/^search \(tilgang gitt av B1 /)).toBeInTheDocument();
    expect(screen.queryByText(/Venter på:/)).not.toBeInTheDocument();
    stub({ [`${API}/namespaces/aksel/agentprofiles/studio`]: profile({ granted: true, grantedAt: T }) });
    render(await AgentPage(params("aksel", "studio")));
    expect(screen.getByText(/^search \(tilgang gitt \d/)).toBeInTheDocument();
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
    expect(screen.getByText("search (tilgang trukket tilbake)")).toBeInTheDocument();
    expect(screen.queryByText(/Venter på:/)).not.toBeInTheDocument();
    expect(screen.getByText(/1\s000 kall i døgnet/)).toBeInTheDocument();
  });

  const waiting = () => profile({ granted: false, contentHash: "g1" });

  it("offers «gi tilgang» in the inbox and on the MCP server page, with the grant's hash", async () => {
    stub({ [`${API}/agentprofiles?awaitingApprovalBy=me`]: { items: [waiting()] } });
    const { unmount } = render(await FleetPage(sp()));
    expect(screen.getByRole("button", { name: "Gi tilgang til docs" })).toBeInTheDocument();
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
    const reason = screen.getAllByRole("textbox", { name: "Begrunnelse" }).at(-1)!;
    fireEvent.change(reason, { target: { value: "lekker data" } });
    const before = vi.mocked(fetch).mock.calls.length;
    fireEvent.click(screen.getByRole("button", { name: "Trekk tilbake" }));
    expect(screen.getByRole("dialog", { name: "Trekk tilbake docs?" })).toBeInTheDocument();
    expect(vi.mocked(fetch).mock.calls.length).toBe(before);
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
    render(await FleetPage(sp()));
    const post = vi.fn(async () => new Response(JSON.stringify({ error: "Nei" }), { status: 403 }));
    vi.stubGlobal("fetch", post);
    fireEvent.click(screen.getByRole("button", { name: "Gi tilgang til docs" }));
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
});
