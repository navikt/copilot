import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { vi } from "vitest";

const router = { push: vi.fn(), refresh: vi.fn(), replace: vi.fn() };
vi.mock("next/navigation", () => ({ useRouter: () => router }));

import { RegisterWizard } from "./wizard";

// Fake values only: the repo is public.
const apps = [
  { name: "kodeverk-mcp", environment: "dev-gcp" },
  { name: "kodeverk-mcp", environment: "prod-gcp" },
];
const props = { teams: ["team-a", "team-b"], team: "team-a", apps, requireRos: false, discoveryEnabled: true };
const next = () => fireEvent.click(screen.getByRole("button", { name: "Neste" }));
const heading = (name: string) => screen.getByRole("heading", { name, level: 2 });
const reply = (status: number, body: unknown) => new Response(JSON.stringify(body), { status });

async function toStep3() {
  fireEvent.change(screen.getByLabelText("Navn på MCP-serveren"), { target: { value: "kodeverk" } });
  next();
  fireEvent.change(screen.getByLabelText("Nais-appen som kjører MCP-serveren"), { target: { value: "kodeverk-mcp" } });
  next();
  heading("MCP-endpointet");
}

describe("RegisterWizard", () => {
  let fetch: ReturnType<typeof vi.fn>;
  beforeEach(() => {
    fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    router.push.mockClear();
  });

  it("step 1 stops on a missing or invalid name", () => {
    render(<RegisterWizard {...props} />);
    next();
    expect(screen.getByText("Skriv et navn.")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Navn på MCP-serveren"), { target: { value: "Kode Verk" } });
    next();
    expect(heading("Navn")).toBeInTheDocument();
  });

  it("step 2 lists the team's apps once per name and pre-fills the address", async () => {
    render(<RegisterWizard {...props} />);
    fireEvent.change(screen.getByLabelText("Navn på MCP-serveren"), { target: { value: "kodeverk" } });
    next();
    expect(screen.getByRole("option", { name: "kodeverk-mcp (dev-gcp, prod-gcp)" })).toBeInTheDocument();
    next();
    expect(screen.getByText("Velg appen som kjører MCP-serveren.")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Nais-appen som kjører MCP-serveren"), {
      target: { value: "kodeverk-mcp" },
    });
    next();
    expect(screen.getByLabelText("Intern adresse")).toHaveValue("http://kodeverk-mcp.team-a/mcp");
  });

  it("step 2 asks for the app name when Nais is unavailable", () => {
    render(<RegisterWizard {...props} apps={null} />);
    fireEvent.change(screen.getByLabelText("Navn på MCP-serveren"), { target: { value: "kodeverk" } });
    next();
    expect(screen.getByText(/fikk ikke hentet appene fra Nais/)).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Nais-appen som kjører MCP-serveren" })).toBeInTheDocument();
  });

  it("step 3 refuses an address with a query", async () => {
    render(<RegisterWizard {...props} />);
    await toStep3();
    fireEvent.change(screen.getByLabelText("Intern adresse"), { target: { value: "http://x.team-a/mcp?a=1" } });
    next();
    expect(screen.getByText(/uten \? eller #/)).toBeInTheDocument();
  });

  it("shows the nais.yaml snippet when the gateway answers 403 or cannot connect", async () => {
    fetch.mockResolvedValueOnce(reply(502, { kind: "access", error: "Fleet fikk ikke kontakt med MCP-serveren." }));
    render(<RegisterWizard {...props} />);
    await toStep3();
    fireEvent.click(screen.getByRole("button", { name: "Hent verktøy" }));
    expect(await screen.findByText(/Fleet fikk ikke kontakt/)).toBeInTheDocument();
    expect(screen.getByText(/application: fleet-gateway/)).toBeInTheDocument();
    expect(JSON.parse(fetch.mock.calls[0][1].body)).toEqual({ namespace: "team-a", name: "kodeverk" });
  });

  it("shows a missing route as a warning, without the snippet, and allows typing tools", async () => {
    fetch.mockResolvedValueOnce(reply(404, { kind: "noroute", error: "Fleet har ingen rute til MCP-serveren ennå." }));
    render(<RegisterWizard {...props} />);
    await toStep3();
    fireEvent.click(screen.getByRole("button", { name: "Hent verktøy" }));
    expect(await screen.findByText(/ingen rute/)).toBeInTheDocument();
    expect(screen.queryByText(/application: fleet-gateway/)).toBeNull();
    next();
    expect(screen.getByText(/Ingen verktøy ennå/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Legg til verktøy"), { target: { value: "a/b" } });
    fireEvent.click(screen.getByRole("button", { name: "Legg til" }));
    expect(screen.getByText(/bare ha bokstaver, tall/)).toBeInTheDocument();
  });

  it("pre-fills discovered tools, leaves destructive ones unticked, and needs a default level", async () => {
    fetch.mockResolvedValueOnce(
      reply(200, {
        tools: [
          { name: "hent_kode", description: "Henter én kode.", suggestedRisk: "read", destructive: false },
          { name: "slett_kode", suggestedRisk: "write", destructive: true },
        ],
      })
    );
    render(<RegisterWizard {...props} />);
    await toStep3();
    fireEvent.click(screen.getByRole("button", { name: "Hent verktøy" }));
    expect(await screen.findByText(/fant 2 verktøy/)).toBeInTheDocument();
    next();
    expect(screen.getByRole("checkbox", { name: /hent_kode/ })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /slett_kode/ })).not.toBeChecked();
    expect(screen.getByRole("radio", { name: "Leser" })).not.toBeChecked();
    next();
    expect(screen.getByText("Velg hva verktøyene gjør som standard.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("radio", { name: "Leser" }));
    expect(screen.queryByText("Velg hva verktøyene gjør som standard.")).toBeNull();
  });

  it("needs a ROS for write tools in prod", async () => {
    render(<RegisterWizard {...props} requireRos />);
    await toStep3();
    next();
    fireEvent.change(screen.getByLabelText("Legg til verktøy"), { target: { value: "lagre" } });
    fireEvent.click(screen.getByRole("button", { name: "Legg til" }));
    fireEvent.click(screen.getByRole("radio", { name: "Skriver" }));
    next();
    expect(screen.getByText(/krever en ROS i prod. Lim inn/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("radio", { name: "Foreslår" }));
    next();
    expect(heading("Data og beskrivelse")).toBeInTheDocument();
  });

  it("needs a data class, submits the spec, and opens the step fleet-control refused", async () => {
    render(<RegisterWizard {...props} />);
    await toStep3();
    next();
    fireEvent.change(screen.getByLabelText("Legg til verktøy"), { target: { value: "hent" } });
    fireEvent.click(screen.getByRole("button", { name: "Legg til" }));
    fireEvent.click(screen.getByRole("radio", { name: "Leser" }));
    next();
    for (const name of ["Åpne data", "Interne data", "Fortrolige data"]) {
      const radio = screen.getByRole("radio", { name });
      expect(radio).not.toBeChecked();
      expect(radio).toHaveAccessibleDescription(/\w/);
    }
    next();
    expect(screen.getByText("Velg en datakategori.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("radio", { name: "Interne data" }));
    next();
    heading("Se over");

    fetch.mockResolvedValueOnce(reply(422, { error: "Rett feltene.", fields: { url: "Feil adresse." } }));
    fireEvent.click(screen.getByRole("button", { name: "Send til godkjenning" }));
    await waitFor(() => heading("MCP-endpointet"));
    expect(screen.getByText("Feil adresse.")).toBeInTheDocument();
    expect(JSON.parse(fetch.mock.calls[0][1].body)).toEqual({
      action: "register",
      namespace: "team-a",
      name: "kodeverk",
      spec: {
        dataCategory: "Intern",
        upstream: { app: "kodeverk-mcp", url: "http://kodeverk-mcp.team-a/mcp" },
        allowedTools: ["hent"],
        defaultRisk: "read",
      },
    });

    for (let i = 0; i < 3; i++) next();
    fetch.mockResolvedValueOnce(reply(200, { href: "/fleet/mcp-servere/team-a/kodeverk" }));
    fireEvent.click(screen.getByRole("button", { name: "Send til godkjenning" }));
    await waitFor(() => expect(router.push).toHaveBeenCalledWith("/fleet/mcp-servere/team-a/kodeverk"));
  });
});
