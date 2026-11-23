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
  usePathname: () => "/fleet",
  useRouter: () => ({ refresh: vi.fn() }),
}));

import FleetPage from "./page";
import RoutePage from "./ruter/[namespace]/[name]/page";
import AgentPage from "./agenter/[namespace]/[name]/page";

const profile = {
  namespace: "aksel",
  name: "studio",
  identity: { cluster: "dev-gcp", namespace: "aksel", app: "studio-api", entra_client_id: "cid" },
  mode: "delegated",
  grants: [{ route: "team-a/docs", tools: ["search"], max_risk: "read" }],
  quota: { max_in_flight: 2, rpm: 60, daily_calls_per_user: 100, daily_calls: 1000 },
  dpia: { owner: "Ola", reference: "DPIA-1" },
  status: "draft",
};

const routes: Record<string, unknown> = {
  "/admin/v1/me": { ident: "A123456", teams: ["team-a"] },
  "/admin/v1/servers?namespace=team-a": {
    servers: [{ namespace: "team-a", name: "docs", status: "published", published_version: 2, pending_version: 3 }],
  },
  "/admin/v1/agents?namespace=team-a": { agents: [] },
  "/admin/v1/agents/pending-approval": { agents: [{ id: 7, profile, awaiting: ["team-a/docs"] }] },
  "/admin/v1/servers/team-a/docs/audit": {
    events: [{ at: "2026-11-20T10:00:00Z", actor: "A123456", action: "draft_submitted", version: 3, detail: null }],
  },
  "/admin/v1/agents/aksel/studio": { id: 7, status: "draft", profile, approvals: { "team-a/docs": null } },
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
    await expect(RoutePage(params("team-a", "docs"))).rejects.toThrow("NEXT_NOT_FOUND");
    await expect(AgentPage(params("aksel", "studio"))).rejects.toThrow("NEXT_NOT_FOUND");
  });

  it("shows Kort fortalt, the inbox and the team's routes with status", async () => {
    render(await FleetPage());
    expect(screen.getByText("Kort fortalt")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Til godkjenning" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "studio" })).toHaveAttribute("href", "/fleet/agenter/aksel/studio");
    expect(screen.getByRole("tab", { name: "team-a" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "docs" })).toHaveAttribute("href", "/fleet/ruter/team-a/docs");
    expect(screen.getByText("Ny versjon venter")).toBeInTheDocument();
    expect(screen.getByText("team-a har ingen agentprofiler ennå.")).toBeInTheDocument();
  });

  it("explains the empty state for a person with no teams", async () => {
    stub({ "/admin/v1/me": { ident: "A123456", teams: [] } });
    render(await FleetPage());
    expect(screen.getByText(/ikke med i noe team/)).toBeInTheDocument();
  });

  it("shows an Alert on 403 and a retry on 503", async () => {
    stub({}, { "/admin/v1/me": 403 });
    render(await FleetPage());
    expect(screen.getByText(/ikke tilgang/)).toBeInTheDocument();
    stub({}, { "/admin/v1/me": 503 });
    render(await FleetPage());
    expect(screen.getByRole("button", { name: "Prøv igjen" })).toBeInTheDocument();
  });

  it("sends 401 to login", async () => {
    stub({}, { "/admin/v1/me": 401 });
    await expect(FleetPage()).rejects.toThrow("NEXT_REDIRECT /oauth2/login");
  });

  it("shows a route with who acts next and its history", async () => {
    render(await RoutePage(params("team-a", "docs")));
    expect(screen.getByText(/må godkjenne versjon 3/)).toBeInTheDocument();
    expect(screen.getByText("Historikk")).toBeInTheDocument();
  });

  it("shows that a revoked route with a new draft waits for approval", async () => {
    stub({
      "/admin/v1/servers?namespace=team-a": {
        servers: [{ namespace: "team-a", name: "docs", status: "revoked", published_version: 2, pending_version: 3 }],
      },
    });
    render(await RoutePage(params("team-a", "docs")));
    expect(screen.getByText("Trukket tilbake, ny versjon venter")).toBeInTheDocument();
    expect(screen.getByText(/må godkjenne versjon 3 før ruten blir publisert igjen/)).toBeInTheDocument();
  });

  it("is 404 for a route the team does not have, or another team's route", async () => {
    await expect(RoutePage(params("team-a", "nope"))).rejects.toThrow("NEXT_NOT_FOUND");
    await expect(RoutePage(params("other", "docs"))).rejects.toThrow("NEXT_NOT_FOUND");
  });

  it("shows an agent profile to a route owner without the owner's history", async () => {
    render(await AgentPage(params("aksel", "studio")));
    expect(screen.getByText(/Venter på: team-a\/docs/)).toBeInTheDocument();
    expect(screen.queryByText("Historikk")).not.toBeInTheDocument();
  });
});
