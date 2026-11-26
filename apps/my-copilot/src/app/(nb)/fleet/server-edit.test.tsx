import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { vi } from "vitest";
import type { MCPServer } from "@/lib/fleet-control";

const router = { push: vi.fn(), refresh: vi.fn(), replace: vi.fn() };
vi.mock("next/navigation", () => ({ useRouter: () => router }));

import { EditServer } from "./server-edit";

// Fake values only: the repo is public.
const spec = {
  dataCategory: "Intern",
  upstream: { app: "docs-mcp", url: "http://docs-mcp.team-a/mcp" },
  allowedTools: ["search", "save"],
  tools: [{ name: "search", description: "Søker i dokumentene." }],
  defaultRisk: "read",
  toolRisk: { save: "write" },
  rosRef: "https://ros.example/1",
};
const server = {
  metadata: { namespace: "team-a", name: "docs", resourceVersion: "7" },
  spec,
  status: { publishedGeneration: 2 },
} as unknown as MCPServer;
const props = { server, published: spec, apps: null, requireRos: true, discoveryEnabled: true };
const reply = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });
const changes = () => screen.queryByRole("list", { name: "Endringer mot den publiserte versjonen" });

describe("EditServer and the shared fields", () => {
  let fetch: ReturnType<typeof vi.fn>;
  beforeEach(() => {
    fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    router.push.mockClear();
  });

  it("shows team and name read-only, and pre-fills the table from the stored spec", () => {
    render(<EditServer {...props} />);
    expect(screen.getByLabelText("Team")).toHaveAttribute("readonly");
    expect(screen.getByText(/kan ikke endres/)).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Tillat search" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Tillat save" })).toBeChecked();
    expect(screen.getByText("Søker i dokumentene.")).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Hva gjør search?" })).toHaveValue("read");
    expect(screen.getByRole("combobox", { name: "Hva gjør save?" })).toHaveValue("write");
    expect(screen.queryByRole("textbox", { name: /^Verktøy/ })).toBeNull();
    expect(changes()).toBeNull();
    expect(screen.getByRole("button", { name: "Publiser endringen" })).toBeDisabled();
  });

  it("«Hent verktøy» keeps allowed tools and unticks only new tools that can change something", async () => {
    fetch.mockResolvedValueOnce(
      reply({
        tools: [
          { name: "save", suggestedRisk: "write", destructive: true },
          { name: "find", description: "Finner.", suggestedRisk: "read", destructive: false },
          { name: "send", suggestedRisk: "write", destructive: false },
        ],
      })
    );
    render(<EditServer {...props} />);
    fireEvent.click(screen.getByRole("button", { name: "Hent verktøy" }));
    expect(await screen.findByText(/Fant 3 verktøy/)).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Tillat save" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Tillat search" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Tillat find" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Tillat send" })).not.toBeChecked();
    expect(screen.getByRole("combobox", { name: "Hva gjør send?" })).toHaveValue("write");
    // find is new and allowed: more access, so team copilot approves.
    expect(within(changes()!).getByText("Nytt verktøy: find (Leser)")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Send til godkjenning" })).toBeEnabled();
  });

  it("adds a tool by hand with validation and dedupe", () => {
    render(<EditServer {...props} />);
    const add = (v: string) => {
      fireEvent.change(screen.getByLabelText("Legg til verktøy"), { target: { value: v } });
      fireEvent.click(screen.getByRole("button", { name: "Legg til" }));
    };
    add("ikke gyldig");
    expect(screen.getByText(/Verktøynavn kan bare ha/)).toBeInTheDocument();
    add("search");
    expect(screen.getByText("search står allerede i lista.")).toBeInTheDocument();
    add("ny");
    expect(screen.getByRole("checkbox", { name: "Tillat ny" })).toBeChecked();
  });

  it("removing a tool publishes at once; the ROS stays required in prod while a tool is Utfører", async () => {
    render(<EditServer {...props} />);
    fireEvent.click(screen.getByRole("checkbox", { name: "Tillat search" }));
    expect(within(changes()!).getByText("Fjernet verktøy: search")).toBeInTheDocument();
    expect(screen.queryByText("Må godkjennes av team copilot")).toBeNull();
    expect(screen.getByRole("button", { name: "Publiser endringen" })).toBeEnabled();
    fireEvent.change(screen.getByLabelText("Lenke til ROS"), { target: { value: "" } });
    expect(screen.getByText("Må godkjennes av team copilot")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Send til godkjenning" }));
    expect(screen.getByText(/krever en ROS i prod. Lim inn/)).toBeInTheDocument();
    expect(fetch).not.toHaveBeenCalled();
  });

  it("submits the whole spec with the resourceVersion and opens the receipt", async () => {
    render(<EditServer {...props} />);
    fireEvent.click(screen.getByRole("checkbox", { name: "Tillat search" }));
    fetch.mockResolvedValueOnce(reply({ href: "/fleet/mcp-servere/team-a/docs?sendt=1" }));
    fireEvent.click(screen.getByRole("button", { name: "Publiser endringen" }));
    await waitFor(() => expect(router.push).toHaveBeenCalledWith("/fleet/mcp-servere/team-a/docs?sendt=1"));
    expect(JSON.parse(fetch.mock.calls[0][1].body)).toEqual({
      action: "edit",
      namespace: "team-a",
      name: "docs",
      resourceVersion: "7",
      spec: {
        dataCategory: "Intern",
        upstream: spec.upstream,
        allowedTools: ["save"],
        defaultRisk: "read",
        toolRisk: { save: "write" },
        rosRef: "https://ros.example/1",
      },
    });
  });

  it("raising the data category needs approval", () => {
    render(<EditServer {...props} />);
    fireEvent.click(screen.getByRole("radio", { name: "Fortrolige data" }));
    expect(within(changes()!).getByText("Datakategori: Interne data til Fortrolige data")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Send til godkjenning" })).toBeEnabled();
  });
});
