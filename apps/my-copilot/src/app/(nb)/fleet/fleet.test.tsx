import { render, screen } from "@testing-library/react";
import { vi } from "vitest";

vi.mock("next/headers", () => ({ headers: vi.fn() }));
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
  useRouter: () => ({ refresh: vi.fn() }),
}));

import FleetPage from "./page";
import ServerPage from "./mcp-servere/[namespace]/[name]/page";
import AgentPage from "./agenter/[namespace]/[name]/page";
import OldRoutePage from "./ruter/[namespace]/[name]/page";

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
    pending: { generation: 3 },
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

const routes: Record<string, unknown> = {
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

const params = (namespace: string, name: string) => ({ params: Promise.resolve({ namespace, name }) });

describe("/fleet", () => {
  beforeEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("FLEET_CONTROL_URL", "http://fc");
    stub();
  });

  it("is 404 on every page without FLEET_CONTROL_URL", async () => {
    vi.stubEnv("FLEET_CONTROL_URL", "");
    await expect(FleetPage()).rejects.toThrow("NEXT_NOT_FOUND");
    await expect(ServerPage(params("team-a", "docs"))).rejects.toThrow("NEXT_NOT_FOUND");
    await expect(AgentPage(params("aksel", "studio"))).rejects.toThrow("NEXT_NOT_FOUND");
  });

  it("calls only the v1alpha1 API, never /admin/v1", async () => {
    render(await FleetPage());
    render(await ServerPage(params("team-a", "docs")));
    render(await AgentPage(params("aksel", "studio")));
    const urls = vi.mocked(fetch).mock.calls.map((c) => String(c[0]));
    expect(urls.length).toBeGreaterThan(0);
    expect(urls.every((u) => u.startsWith(`http://fc${API}/`))).toBe(true);
  });

  it("shows Kort fortalt, the inbox and the team's MCP servers with status", async () => {
    render(await FleetPage());
    expect(screen.getByText("Kort fortalt")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Til godkjenning" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "studio" })).toHaveAttribute("href", "/fleet/agenter/aksel/studio");
    expect(screen.getByText("MCP-server team-a/docs")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "team-a" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "docs" })).toHaveAttribute("href", "/fleet/mcp-servere/team-a/docs");
    expect(screen.getByText("Ny versjon venter")).toBeInTheDocument();
    expect(screen.getByText("team-a har ingen agentprofiler ennå.")).toBeInTheDocument();
  });

  it("explains the empty state for a person with no teams", async () => {
    stub({ [`${API}/whoami`]: { name: "A123456", teams: [] } });
    render(await FleetPage());
    expect(screen.getByText(/ikke med i noe team/)).toBeInTheDocument();
  });

  it("shows an Alert on 403 and 409, and a retry on 503", async () => {
    stub({}, { [`${API}/whoami`]: 403 });
    render(await FleetPage());
    expect(screen.getByText(/ikke tilgang/)).toBeInTheDocument();
    stub({}, { [`${API}/whoami`]: 409 });
    render(await FleetPage());
    expect(screen.getByText("Noen andre har endret dette. Last inn siden på nytt.")).toBeInTheDocument();
    stub({}, { [`${API}/whoami`]: 503 });
    render(await FleetPage());
    expect(screen.getByRole("button", { name: "Prøv igjen" })).toBeInTheDocument();
  });

  it("sends 401 to login", async () => {
    stub({}, { [`${API}/whoami`]: 401 });
    await expect(FleetPage()).rejects.toThrow("NEXT_REDIRECT /oauth2/login");
  });

  it("shows an MCP server with its route, who acts next and its history", async () => {
    render(await ServerPage(params("team-a", "docs")));
    expect(screen.getByText(/må godkjenne versjon 3/)).toBeInTheDocument();
    expect(screen.getByText("/mcp/team-a/docs")).toBeInTheDocument();
    expect(screen.getByText("Historikk")).toBeInTheDocument();
  });

  it("shows that a revoked MCP server with a new draft waits for approval", async () => {
    stub({ [`${API}/namespaces/team-a/mcpservers/docs`]: server(true) });
    render(await ServerPage(params("team-a", "docs")));
    expect(screen.getByText("Trukket tilbake, ny versjon venter")).toBeInTheDocument();
    expect(screen.getByText(/må godkjenne versjon 3 før MCP-serveren blir publisert igjen/)).toBeInTheDocument();
  });

  it("is 404 for an MCP server the team does not have, or another team's", async () => {
    await expect(ServerPage(params("team-a", "nope"))).rejects.toThrow("NEXT_NOT_FOUND");
    await expect(ServerPage(params("other", "docs"))).rejects.toThrow("NEXT_NOT_FOUND");
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
});
